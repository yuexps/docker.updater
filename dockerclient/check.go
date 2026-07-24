package dockerclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"docker-updater/db"
	"docker-updater/utils"

	"github.com/docker/docker/api/types"
)

// CredentialsProvider 凭证获取接口。
type CredentialsProvider interface {
	GetCredential(ctx context.Context, registry string) (username, password string, ok bool)
}

// GlobalCredentialsProvider 全局凭证提供者。
var GlobalCredentialsProvider CredentialsProvider

const manifestAccept = "application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.docker.distribution.manifest.v2+json, " +
	"application/vnd.oci.image.index.v1+json, " +
	"application/vnd.oci.image.manifest.v1+json"

// parseImage 解析镜像名称。
func parseImage(imageName string) (string, string, string) {
	tag := "latest"
	name := imageName

	if idx := strings.Index(imageName, "@"); idx != -1 {
		name = imageName[:idx]
	}

	lastSlash := strings.LastIndex(name, "/")
	lastColon := strings.LastIndex(name, ":")
	if lastColon > lastSlash {
		tag = name[lastColon+1:]
		name = name[:lastColon]
	}

	parts := strings.Split(name, "/")
	first := parts[0]

	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first, strings.Join(parts[1:], "/"), tag
	}

	repo := name
	if !strings.Contains(name, "/") {
		repo = "library/" + name
	}
	return "registry-1.docker.io", repo, tag
}

// getDockerRepoCandidates 获取匹配的仓库别名候选列表。
func getDockerRepoCandidates(registry, repo string) []string {
	candidates := []string{repo, registry + "/" + repo}
	if registry == "registry-1.docker.io" {
		candidates = append(candidates, "docker.io/"+repo)
		if strings.HasPrefix(repo, "library/") {
			short := repo[8:]
			candidates = append(candidates, short, "docker.io/"+short, "registry-1.docker.io/"+short)
		}
	}
	return candidates
}

// parseChallengeHeader 解析 WWW-Authenticate 响应头。
func parseChallengeHeader(authHeader string) (string, map[string]string) {
	params := make(map[string]string)
	re := regexp.MustCompile(`(\w+)="([^"]*)"`)
	matches := re.FindAllStringSubmatch(authHeader, -1)

	var realm string
	for _, match := range matches {
		key := match[1]
		val := match[2]
		if key == "realm" {
			realm = val
		} else {
			params[key] = val
		}
	}
	return realm, params
}

// getBearerToken 获取 Bearer Token。
func getBearerToken(authHeader string, registry string) (string, error) {
	realm, params := parseChallengeHeader(authHeader)
	if realm == "" {
		return "", fmt.Errorf("missing realm in auth header")
	}

	u, err := url.Parse(realm)
	if err != nil {
		return "", err
	}

	q := u.Query()
	for k, v := range params {
		q.Set(k, v)
	}
	u.RawQuery = q.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest("GET", u.String(), nil)
	if err != nil {
		return "", err
	}

	if GlobalCredentialsProvider != nil {
		if username, password, ok := GlobalCredentialsProvider.GetCredential(context.Background(), registry); ok {
			authVal := username + ":" + password
			req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(authVal)))
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token request failed: %s", resp.Status)
	}

	var data map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return "", err
	}

	if token, ok := data["token"].(string); ok && token != "" {
		return token, nil
	}
	if token, ok := data["access_token"].(string); ok && token != "" {
		return token, nil
	}
	return "", fmt.Errorf("token not found in response")
}

// getRemoteDigest 获取远程镜像 Manifest 摘要与 Config 摘要。
func getRemoteDigest(imageName string, localDigest string, localImageID string, localPlatform map[string]string) (string, string, error) {
	registry, repo, tag := parseImage(imageName)

	var hosts []string
	if registry == "registry-1.docker.io" {
		var appMirrors []string
		tempMirrorsStr := db.GetSetting("temp_mirrors", "[]")
		_ = json.Unmarshal([]byte(tempMirrorsStr), &appMirrors)

		// 融合应用级配置的镜像加速器并进行去重
		seen := make(map[string]bool)
		var mirrors []string
		for _, m := range appMirrors {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if !seen[m] {
				seen[m] = true
				mirrors = append(mirrors, m)
			}
		}

		// 提取出各个加速器的 Host
		for _, m := range mirrors {
			u, err := url.Parse(m)
			if err == nil && u.Host != "" {
				hosts = append(hosts, u.Host)
			} else {
				mClean := strings.TrimPrefix(m, "https://")
				mClean = strings.TrimPrefix(mClean, "http://")
				mClean = strings.TrimSuffix(mClean, "/")
				if mClean != "" {
					hosts = append(hosts, mClean)
				}
			}
		}
	}
	hosts = append(hosts, registry) // 官方或第三方原始注册表作为兜底

	var lastErr error
	for _, host := range hosts {
		digest, configDigest, err := getRemoteDigestWithHost(host, registry, repo, tag, localDigest, localImageID, localPlatform)
		if err == nil {
			return digest, configDigest, nil
		}
		lastErr = err
		utils.LogWarning("使用镜像服务器 %s 获取 %s 的 Digest 失败: %s，正在尝试下一个...", host, imageName, err.Error())
	}
	return "", "", lastErr
}

// getRemoteDigestWithHost 获取指定镜像源的 Manifest 摘要与 Config 摘要。
func getRemoteDigestWithHost(host, registry, repo, tag, localDigest, localImageID string, localPlatform map[string]string) (string, string, error) {
	endpoint := fmt.Sprintf("https://%s/v2/%s/manifests/%s", host, repo, tag)
	client := &http.Client{Timeout: 8 * time.Second}

	// 本地 Digest 为空时强制使用 GET 请求以解析 config.digest
	forceGet := localDigest == "" && localImageID != ""
	method := "HEAD"
	if forceGet {
		method = "GET"
	}

	req, err := http.NewRequest(method, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Accept", manifestAccept)

	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}

	var token string
	// 401 认证失败则获取 Token 重试
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		token, err = getBearerToken(authHeader, registry)
		if err == nil {
			resp.Body.Close()

			req, err = http.NewRequest(method, endpoint, nil)
			if err != nil {
				return "", "", err
			}
			req.Header.Set("Accept", manifestAccept)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err = client.Do(req)
			if err != nil {
				return "", "", err
			}
		} else {
			resp.Body.Close()
			return "", "", err
		}
	}

	// HEAD 失败时降级为 GET 请求
	if resp.StatusCode != http.StatusOK && method == "HEAD" {
		resp.Body.Close()
		method = "GET"

		req, err = http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return "", "", err
		}
		req.Header.Set("Accept", manifestAccept)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err = client.Do(req)
		if err != nil {
			return "", "", err
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	remoteDigest := resp.Header.Get("Docker-Content-Digest")

	// 获取指定架构的 Manifest Body
	getManifestBody := func(ref string) (io.ReadCloser, string, error) {
		subURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", host, repo, ref)
		subReq, subErr := http.NewRequest("GET", subURL, nil)
		if subErr != nil {
			return nil, "", subErr
		}
		subReq.Header.Set("Accept", manifestAccept)
		if token != "" {
			subReq.Header.Set("Authorization", "Bearer "+token)
		}
		subResp, subErr := client.Do(subReq)
		if subErr != nil {
			return nil, "", subErr
		}
		if subResp.StatusCode != http.StatusOK {
			subResp.Body.Close()
			return nil, "", fmt.Errorf("unexpected status: %s", subResp.Status)
		}
		subDigest := subResp.Header.Get("Docker-Content-Digest")
		return subResp.Body, subDigest, nil
	}

	type manifestResponse struct {
		Manifests []struct {
			Digest   string            `json:"digest"`
			Platform map[string]string `json:"platform"`
		} `json:"manifests"`
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}

	// 解析 GET 响应的 Body
	if method == "GET" {
		var m manifestResponse
		if json.NewDecoder(resp.Body).Decode(&m) == nil {
			if len(m.Manifests) > 0 {
				for _, sub := range m.Manifests {
					if sub.Digest == localDigest {
						return localDigest, "", nil
					}
				}
				if localImageID != "" && localPlatform != nil {
					for _, sub := range m.Manifests {
						if sub.Platform != nil && sub.Platform["os"] == localPlatform["os"] &&
							sub.Platform["architecture"] == localPlatform["architecture"] {
							// 获取子架构的 config.digest
							var subConfigDigest string
							if localDigest == "" {
								subBody, _, subErr := getManifestBody(sub.Digest)
								if subErr == nil {
									defer subBody.Close()
									var subM manifestResponse
									if json.NewDecoder(subBody).Decode(&subM) == nil {
										subConfigDigest = subM.Config.Digest
									}
								}
							}
							return sub.Digest, subConfigDigest, nil
						}
					}
				}
			} else if m.Config.Digest != "" {
				// 单架构镜像
				return remoteDigest, m.Config.Digest, nil
			}
		}
	} else {
		// HEAD 响应且摘要不匹配时解析多架构列表
		if remoteDigest != "" && localDigest != "" && remoteDigest != localDigest {
			body, _, getErr := getManifestBody(tag)
			if getErr == nil {
				defer body.Close()
				var m manifestResponse
				if json.NewDecoder(body).Decode(&m) == nil && len(m.Manifests) > 0 {
					for _, sub := range m.Manifests {
						if sub.Digest == localDigest {
							return localDigest, "", nil
						}
					}
					if localImageID != "" && localPlatform != nil {
						for _, sub := range m.Manifests {
							if sub.Platform != nil && sub.Platform["os"] == localPlatform["os"] &&
								sub.Platform["architecture"] == localPlatform["architecture"] {
								return sub.Digest, "", nil
							}
						}
					}
				}
			}
		}
	}

	return remoteDigest, "", nil
}

// getLocalDigestFromImage 获取本地镜像 Digest。
func getLocalDigestFromImage(repoDigests []string, imageName string) string {
	if len(repoDigests) == 0 {
		return ""
	}
	registry, repo, _ := parseImage(imageName)
	candidates := getDockerRepoCandidates(registry, repo)

	for _, digestRef := range repoDigests {
		parts := strings.SplitN(digestRef, "@", 2)
		if len(parts) < 2 {
			continue
		}
		repoRef := parts[0]
		digestVal := parts[1]
		for _, c := range candidates {
			if repoRef == c {
				return digestVal
			}
		}
	}
	parts := strings.SplitN(repoDigests[0], "@", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// semverRegexp 提取严格的语义化版本。
var semverRegexp = regexp.MustCompile(`^v?(\d+(?:\.\d+)+(?:-[a-zA-Z0-9._-]+)?)`)

// ExtractSemanticVersion 解析镜像标签、Tag 或环境变量中的版本号。
func ExtractSemanticVersion(labels map[string]string, env []string, imageName string) string {
	// 1. OCI Labels
	if labels != nil {
		labelKeys := []string{
			"org.opencontainers.image.version",
			"org.label-schema.version",
			"version",
			"app.kubernetes.io/version",
		}
		for _, key := range labelKeys {
			if val, ok := labels[key]; ok && strings.TrimSpace(val) != "" {
				val = strings.TrimSpace(val)
				if semverRegexp.MatchString(val) {
					return val
				}
			}
		}
	}

	// 2. Tag
	if imageName != "" {
		_, _, tag := parseImage(imageName)
		if tag != "" && tag != "latest" && tag != "main" && tag != "master" && tag != "dev" && tag != "test" && tag != "stable" {
			match := semverRegexp.FindString(tag)
			if match != "" {
				return match
			}
		}
	}

	// 3. Env
	for _, e := range env {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			k, v := parts[0], strings.TrimSpace(parts[1])
			if (k == "VERSION" || k == "APP_VERSION" || k == "RELEASE_VERSION" || k == "BUILD_VERSION") && v != "" {
				if semverRegexp.MatchString(v) || len(v) <= 15 {
					return v
				}
			}
		}
	}

	return ""
}

// checkItem 待检查容器项结构体。
type checkItem struct {
	containerName  string
	composeProject string
	imageName      string
	imageID        string
	localDigest    string
	localVersion   string
	localPlatform  map[string]string
}

// checkResult 镜像检测结果。
type checkResult struct {
	item               checkItem
	remoteDigest       string
	remoteConfigDigest string
	remoteVersion      string
	err                error
}

// UpdateCheckResult 容器版本检测结果。
type UpdateCheckResult struct {
	ContainerName  string
	Image          string
	LocalDigest    string
	RemoteDigest   string
	LocalVersion   string
	RemoteVersion  string
	CheckedAt      string
	ComposeProject string
	HasUpdate      bool
}

// ScanLocalHostForUpdates 扫描本地容器版本。
func ScanLocalHostForUpdates(ctx context.Context) ([]UpdateCheckResult, error) {
	utils.LogInfo("开始扫描本地活动容器进行版本比对检查...")

	cli, err := NewLocalClient()
	if err != nil {
		utils.LogError("建立 Docker 引擎连接失败: %s", err.Error())
		return nil, err
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		utils.LogError("获取本地容器列表失败: %s", err.Error())
		return nil, err
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	var deferList []db.DeferredUpdate
	db.DB.Find(&deferList)
	deferMap := make(map[string]db.DeferredUpdate)
	today := time.Now().Format("2006-01-02")
	for _, d := range deferList {
		deferMap[d.ContainerName] = d
	}

	var items []checkItem
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if name == "" || strings.HasSuffix(name, "_backup_docker_updater") {
			continue
		}

		if d, isDeferred := deferMap[name]; isDeferred && (d.Until == "forever" || d.Until > today) {
			utils.LogInfo("容器 %s 当前处于暂挂检测状态 (暂挂至: %s)，自动跳过扫描", name, d.Until)
			continue
		}

		inspect, err := cli.ContainerInspect(ctx, c.ID)
		if err != nil {
			utils.LogWarning("无法获取容器 %s 详情: %s", name, err.Error())
			continue
		}

		imageName := inspect.Config.Image
		imageInspect, _, err := cli.ImageInspectWithRaw(ctx, inspect.Image)
		if err != nil {
			utils.LogWarning("无法获取镜像 %s 的 inspect 详情: %s", imageName, err.Error())
			continue
		}

		rawLocalDigest := ""
		if len(imageInspect.RepoDigests) > 0 {
			rawLocalDigest = getLocalDigestFromImage(imageInspect.RepoDigests, imageName)
		}

		// 若 RepoDigests 缺失，以镜像 ID 作为回退摘要
		displayLocalDigest := rawLocalDigest
		if displayLocalDigest == "" && imageInspect.ID != "" {
			displayLocalDigest = imageInspect.ID
		}

		localVersion := ExtractSemanticVersion(imageInspect.Config.Labels, imageInspect.Config.Env, imageName)

		items = append(items, checkItem{
			containerName:  name,
			composeProject: c.Labels["com.docker.compose.project"],
			imageName:      imageName,
			imageID:        imageInspect.ID,
			localDigest:    displayLocalDigest,
			localVersion:   localVersion,
			localPlatform: map[string]string{
				"os":           imageInspect.Os,
				"architecture": imageInspect.Architecture,
				"variant":      imageInspect.Variant,
			},
		})
	}

	if len(items) == 0 {
		utils.LogInfo("未发现需检查的容器，扫描结束")
		return nil, nil
	}

	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)
	resultCh := make(chan checkResult, len(items))

	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		go func(it checkItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			remoteDigest, remoteConfigDigest, err := getRemoteDigest(it.imageName, it.localDigest, it.imageID, it.localPlatform)
			resultCh <- checkResult{
				item:               it,
				remoteDigest:       remoteDigest,
				remoteConfigDigest: remoteConfigDigest,
				err:                err,
			}
		}(item)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var results []UpdateCheckResult
	var updateCount int
	for res := range resultCh {
		if res.err != nil {
			utils.LogWarning("获取服务 %s 的远程 Registry 镜像 Digest 失败: %s", res.item.containerName, res.err.Error())
			continue
		}

		hasUpdate := false
		if res.remoteDigest != "" {
			// 优先使用镜像规范 RepoDigest 比对
			if res.item.localDigest != "" && res.item.localDigest != res.item.imageID {
				hasUpdate = res.item.localDigest != res.remoteDigest
			} else {
				// 无 RepoDigest 时优先比对远程 Config.Digest 与本地 Image ID
				if res.remoteConfigDigest != "" && res.item.imageID != "" {
					localID := res.item.imageID
					remoteID := res.remoteConfigDigest
					if !strings.HasPrefix(localID, "sha256:") {
						localID = "sha256:" + localID
					}
					if !strings.HasPrefix(remoteID, "sha256:") {
						remoteID = "sha256:" + remoteID
					}
					hasUpdate = localID != remoteID
				} else {
					// 兜底查询本地是否存在此远程 Digest 镜像
					_, _, err = cli.ImageInspectWithRaw(ctx, res.remoteDigest)
					if err != nil {
						hasUpdate = true
					}
				}
			}
		}

		remoteVersion := ExtractSemanticVersion(nil, nil, res.item.imageName)

		if hasUpdate {
			localVerPart := ""
			if res.item.localVersion != "" {
				localVerPart = "version:" + res.item.localVersion + " "
			}
			remoteVerPart := ""
			if remoteVersion != "" {
				remoteVerPart = "version:" + remoteVersion + " "
			}
			localHash := strings.TrimPrefix(res.item.localDigest, "sha256:")
			remoteHash := strings.TrimPrefix(res.remoteDigest, "sha256:")

			utils.LogInfo("服务 %s 存在可用升级 (本地: %ssha256:%s, 远端: %ssha256:%s)",
				res.item.containerName, localVerPart, localHash, remoteVerPart, remoteHash)
			updateCount++
		}

		results = append(results, UpdateCheckResult{
			ContainerName:  res.item.containerName,
			Image:          res.item.imageName,
			LocalDigest:    res.item.localDigest,
			RemoteDigest:   res.remoteDigest,
			LocalVersion:   res.item.localVersion,
			RemoteVersion:  remoteVersion,
			CheckedAt:      nowStr,
			ComposeProject: res.item.composeProject,
			HasUpdate:      hasUpdate,
		})
	}

	utils.LogInfo("容器扫描比对检查结束。当前共发现 %d 个待升级服务。", updateCount)
	return results, nil
}

// GetRemoteTags 获取远程 Registry 的镜像标签列表。
func GetRemoteTags(imageName string) ([]string, error) {
	registry, repo, _ := parseImage(imageName)
	endpoint := fmt.Sprintf("https://%s/v2/%s/tags/list", registry, repo)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		token, err := getBearerToken(authHeader, registry)
		if err == nil {
			resp.Body.Close()

			req, err = http.NewRequest("GET", endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err = client.Do(req)
			if err != nil {
				return nil, err
			}
		}
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	var data struct {
		Tags []string `json:"tags"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	var filtered []string
	for _, t := range data.Tags {
		if t == "latest" || strings.Contains(t, "-dev") || strings.Contains(t, "-test") || strings.HasPrefix(t, "sha256-") {
			continue
		}
		filtered = append(filtered, t)
	}

	for i, j := 0, len(filtered)-1; i < j; i, j = i+1, j-1 {
		filtered[i], filtered[j] = filtered[j], filtered[i]
	}

	if len(filtered) > 20 {
		filtered = filtered[:20]
	}
	return filtered, nil
}

// ScanSingleContainer 单独对指定容器进行版本比对检查。
func ScanSingleContainer(ctx context.Context, targetName string) (*UpdateCheckResult, error) {
	cli, err := NewLocalClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(ctx, targetName)
	if err != nil {
		return nil, fmt.Errorf("无法获取容器 %s 详情: %w", targetName, err)
	}

	imageName := inspect.Config.Image
	imageInspect, _, err := cli.ImageInspectWithRaw(ctx, inspect.Image)
	if err != nil {
		return nil, fmt.Errorf("无法获取镜像 %s 详情: %w", imageName, err)
	}

	rawLocalDigest := ""
	if len(imageInspect.RepoDigests) > 0 {
		rawLocalDigest = getLocalDigestFromImage(imageInspect.RepoDigests, imageName)
	}

	displayLocalDigest := rawLocalDigest
	if displayLocalDigest == "" && imageInspect.ID != "" {
		displayLocalDigest = imageInspect.ID
	}

	localVersion := ExtractSemanticVersion(imageInspect.Config.Labels, imageInspect.Config.Env, imageName)
	localPlatform := map[string]string{
		"os":           imageInspect.Os,
		"architecture": imageInspect.Architecture,
		"variant":      imageInspect.Variant,
	}

	remoteDigest, remoteConfigDigest, err := getRemoteDigest(imageName, displayLocalDigest, imageInspect.ID, localPlatform)
	if err != nil {
		return nil, fmt.Errorf("获取远程 Registry 镜像 Digest 失败: %w", err)
	}

	hasUpdate := false
	if remoteDigest != "" {
		if displayLocalDigest != "" && displayLocalDigest != imageInspect.ID {
			hasUpdate = displayLocalDigest != remoteDigest
		} else {
			if remoteConfigDigest != "" && imageInspect.ID != "" {
				localID := imageInspect.ID
				remoteID := remoteConfigDigest
				if !strings.HasPrefix(localID, "sha256:") {
					localID = "sha256:" + localID
				}
				if !strings.HasPrefix(remoteID, "sha256:") {
					remoteID = "sha256:" + remoteID
				}
				hasUpdate = localID != remoteID
			} else {
				_, _, err = cli.ImageInspectWithRaw(ctx, remoteDigest)
				if err != nil {
					hasUpdate = true
				}
			}
		}
	}

	remoteVersion := ExtractSemanticVersion(nil, nil, imageName)
	nowStr := time.Now().UTC().Format(time.RFC3339)

	composeProject := ""
	if inspect.Config.Labels != nil {
		composeProject = inspect.Config.Labels["com.docker.compose.project"]
	}

	return &UpdateCheckResult{
		ContainerName:  targetName,
		Image:          imageName,
		LocalDigest:    displayLocalDigest,
		RemoteDigest:   remoteDigest,
		LocalVersion:   localVersion,
		RemoteVersion:  remoteVersion,
		CheckedAt:      nowStr,
		ComposeProject: composeProject,
		HasUpdate:      hasUpdate,
	}, nil
}

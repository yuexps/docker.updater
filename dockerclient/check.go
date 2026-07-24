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
	"github.com/docker/docker/client"
)

// CredentialsProvider 凭证获取接口
type CredentialsProvider interface {
	GetCredential(ctx context.Context, registry string) (username, password string, ok bool)
}

// GlobalCredentialsProvider 全局凭证提供者
var GlobalCredentialsProvider CredentialsProvider

const manifestAccept = "application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.docker.distribution.manifest.v2+json, " +
	"application/vnd.oci.image.index.v1+json, " +
	"application/vnd.oci.image.manifest.v1+json"

// parseImage 解析镜像名称
func parseImage(imageName string) (registry string, repo string, tag string) {
	tag = "latest"
	name := imageName

	if idx := strings.Index(name, "@"); idx != -1 {
		name = name[:idx]
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
		registry = first
		repo = strings.Join(parts[1:], "/")
	} else {
		registry = "docker.io"
		repo = name
	}

	if registry == "docker.io" || registry == "index.docker.io" || registry == "registry.docker.io" {
		registry = "registry-1.docker.io"
		if !strings.Contains(repo, "/") {
			repo = "library/" + repo
		}
	}

	return registry, repo, tag
}

// getDockerRepoCandidates 获取仓库候选别名列表
func getDockerRepoCandidates(registry, repo string) []string {
	candidates := []string{repo, registry + "/" + repo}
	if registry == "registry-1.docker.io" {
		candidates = append(candidates, "docker.io/"+repo, "index.docker.io/"+repo)
		if strings.HasPrefix(repo, "library/") {
			short := repo[8:]
			candidates = append(candidates, short, "docker.io/"+short, "index.docker.io/"+short, "registry-1.docker.io/"+short)
		}
	}
	return candidates
}

// parseChallengeHeader 解析 WWW-Authenticate 响应头
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

// handleAuthChallenge 处理 401 认证挑战
func handleAuthChallenge(authHeader string, registry string) (string, string, error) {
	authHeaderTrim := strings.TrimSpace(authHeader)
	if strings.HasPrefix(strings.ToLower(authHeaderTrim), "basic") {
		if GlobalCredentialsProvider != nil {
			if username, password, ok := GlobalCredentialsProvider.GetCredential(context.Background(), registry); ok {
				authVal := username + ":" + password
				return "Authorization", "Basic " + base64.StdEncoding.EncodeToString([]byte(authVal)), nil
			}
		}
		return "", "", fmt.Errorf("received basic auth challenge but credentials for %s not configured", registry)
	}

	token, err := getBearerToken(authHeader, registry)
	if err == nil {
		return "Authorization", "Bearer " + token, nil
	}
	return "", "", err
}

// getBearerToken 获取 Bearer Token
func getBearerToken(authHeader string, registry string) (string, error) {
	realm, params := parseChallengeHeader(authHeader)
	if realm == "" {
		return "", fmt.Errorf("missing realm in auth header")
	}

	u, err := url.Parse(realm)
	if err != nil || u.Scheme == "" {
		return "", fmt.Errorf("invalid or non-http realm URL in auth header: %s", realm)
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

// isPlatformMatch 比对 Manifest 平台架构
func isPlatformMatch(subPlatform, localPlatform map[string]string) bool {
	if subPlatform == nil || localPlatform == nil {
		return false
	}
	if subPlatform["os"] != localPlatform["os"] || subPlatform["architecture"] != localPlatform["architecture"] {
		return false
	}
	if localVariant, ok := localPlatform["variant"]; ok && localVariant != "" {
		if subVariant, subOk := subPlatform["variant"]; subOk && subVariant != "" {
			if subVariant != localVariant {
				return false
			}
		}
	}
	return true
}

// DistributionInspectResult 镜像 DistributionInspect 检查结果
type DistributionInspectResult struct {
	RemoteDigest       string
	RemoteConfigDigest string
}

// inspectRemoteDistribution 获取远端 DistributionInspect 镜像元数据
func inspectRemoteDistribution(imageName string, localPlatform map[string]string) (*DistributionInspectResult, error) {
	registry, repo, tag := parseImage(imageName)
	hosts := getRegistryHosts(registry)

	var lastErr error
	for _, host := range hosts {
		res, err := inspectRemoteDistributionWithHost(host, registry, repo, tag, localPlatform)
		if err == nil {
			return res, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// inspectRemoteDistributionWithHost 从指定 Host 获取 Manifest
func inspectRemoteDistributionWithHost(host, registry, repo, tag string, localPlatform map[string]string) (*DistributionInspectResult, error) {
	endpoint := fmt.Sprintf("https://%s/v2/%s/manifests/%s", host, repo, tag)
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", manifestAccept)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	var authHeaderName, authHeaderVal string
	if resp.StatusCode == http.StatusUnauthorized {
		authChallenge := resp.Header.Get("WWW-Authenticate")
		headerName, headerVal, authErr := handleAuthChallenge(authChallenge, registry)
		if authErr == nil {
			resp.Body.Close()
			authHeaderName = headerName
			authHeaderVal = headerVal

			req, err = http.NewRequest("GET", endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Accept", manifestAccept)
			req.Header.Set(authHeaderName, authHeaderVal)
			resp, err = client.Do(req)
			if err != nil {
				return nil, err
			}
		} else {
			resp.Body.Close()
			return nil, authErr
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	headerDigest := resp.Header.Get("Docker-Content-Digest")

	type manifestResponse struct {
		Manifests []struct {
			Digest   string            `json:"digest"`
			Platform map[string]string `json:"platform"`
		} `json:"manifests"`
		Config struct {
			Digest string `json:"digest"`
		} `json:"config"`
	}

	var m manifestResponse
	bodyBytes, readErr := io.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}

	if json.Unmarshal(bodyBytes, &m) == nil {
		if len(m.Manifests) > 0 {
			for _, sub := range m.Manifests {
				if isPlatformMatch(sub.Platform, localPlatform) {
					subURL := fmt.Sprintf("https://%s/v2/%s/manifests/%s", host, repo, sub.Digest)
					subReq, subErr := http.NewRequest("GET", subURL, nil)
					if subErr == nil {
						subReq.Header.Set("Accept", manifestAccept)
						if authHeaderName != "" {
							subReq.Header.Set(authHeaderName, authHeaderVal)
						}
						subResp, subErr := client.Do(subReq)
						if subErr == nil {
							defer subResp.Body.Close()
							if subResp.StatusCode == http.StatusOK {
								var subM manifestResponse
								if json.NewDecoder(subResp.Body).Decode(&subM) == nil {
									return &DistributionInspectResult{
										RemoteDigest:       sub.Digest,
										RemoteConfigDigest: subM.Config.Digest,
									}, nil
								}
							}
						}
					}
					return &DistributionInspectResult{
						RemoteDigest: sub.Digest,
					}, nil
				}
			}
		} else if m.Config.Digest != "" {
			return &DistributionInspectResult{
				RemoteDigest:       headerDigest,
				RemoteConfigDigest: m.Config.Digest,
			}, nil
		}
	}

	return &DistributionInspectResult{
		RemoteDigest: headerDigest,
	}, nil
}

// getLocalDigestFromImage 获取本地镜像 Digest
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

// semverRegexp 提取语义化版本
var semverRegexp = regexp.MustCompile(`^v?(\d+(?:\.\d+)+(?:-[a-zA-Z0-9._-]+)?)`)

// ExtractSemanticVersion 解析镜像或环境变量中的版本号
func ExtractSemanticVersion(labels map[string]string, env []string, imageName string) string {
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

	if imageName != "" {
		_, _, tag := parseImage(imageName)
		if tag != "" && tag != "latest" && tag != "main" && tag != "master" && tag != "dev" && tag != "test" && tag != "stable" {
			match := semverRegexp.FindString(tag)
			if match != "" {
				return match
			}
		}
	}

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

// checkItem 待检测容器结构体
type checkItem struct {
	containerName  string
	composeProject string
	imageName      string
	imageID        string
	rawRepoDigest  string
	repoDigests    []string
	localDigest    string
	localVersion   string
	localPlatform  map[string]string
}

// checkResult 镜像检测结果
type checkResult struct {
	item               checkItem
	remoteDigest       string
	remoteConfigDigest string
	remoteVersion      string
	err                error
}

// UpdateCheckResult 容器版本检测结果
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

// buildCheckItem 构造 checkItem 结构体
func buildCheckItem(containerName string, composeProject string, inspect types.ContainerJSON, imageInspect types.ImageInspect) checkItem {
	imageName := inspect.Config.Image
	rawLocalDigest := ""
	if len(imageInspect.RepoDigests) > 0 {
		rawLocalDigest = getLocalDigestFromImage(imageInspect.RepoDigests, imageName)
	}

	displayLocalDigest := rawLocalDigest
	if displayLocalDigest == "" && imageInspect.ID != "" {
		displayLocalDigest = imageInspect.ID
	}

	localVersion := ExtractSemanticVersion(imageInspect.Config.Labels, imageInspect.Config.Env, imageName)

	return checkItem{
		containerName:  containerName,
		composeProject: composeProject,
		imageName:      imageName,
		imageID:        imageInspect.ID,
		rawRepoDigest:  rawLocalDigest,
		repoDigests:    imageInspect.RepoDigests,
		localDigest:    displayLocalDigest,
		localVersion:   localVersion,
		localPlatform: map[string]string{
			"os":           imageInspect.Os,
			"architecture": imageInspect.Architecture,
			"variant":      imageInspect.Variant,
		},
	}
}

// evaluatePassiveUpdateStatus 校验镜像摘要更新状态
func evaluatePassiveUpdateStatus(ctx context.Context, cli *client.Client, item checkItem, remoteDigest, remoteConfigDigest string) bool {
	if remoteDigest == "" && remoteConfigDigest == "" {
		return false
	}

	if remoteDigest != "" {
		if item.rawRepoDigest != "" && item.rawRepoDigest == remoteDigest {
			return false
		}
		for _, rd := range item.repoDigests {
			if strings.Contains(rd, "@"+remoteDigest) {
				return false
			}
		}
	}

	if remoteConfigDigest != "" && item.imageID != "" {
		localID := item.imageID
		remoteID := remoteConfigDigest
		if !strings.HasPrefix(localID, "sha256:") {
			localID = "sha256:" + localID
		}
		if !strings.HasPrefix(remoteID, "sha256:") {
			remoteID = "sha256:" + remoteID
		}
		if localID == remoteID {
			return false
		}
	}

	if remoteDigest != "" {
		if _, _, err := cli.ImageInspectWithRaw(ctx, remoteDigest); err == nil {
			return false
		}
	}
	if remoteConfigDigest != "" {
		if _, _, errCfg := cli.ImageInspectWithRaw(ctx, remoteConfigDigest); errCfg == nil {
			return false
		}
	}

	return true
}

// ScanLocalHostForUpdates 扫描本地活动容器镜像版本
func ScanLocalHostForUpdates(ctx context.Context) ([]UpdateCheckResult, error) {
	utils.LogInfo("开始全量扫描本地活动容器...")

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

		imageInspect, _, err := cli.ImageInspectWithRaw(ctx, inspect.Image)
		if err != nil {
			utils.LogWarning("无法获取镜像 %s 的 inspect 详情: %s", inspect.Config.Image, err.Error())
			continue
		}

		items = append(items, buildCheckItem(name, c.Labels["com.docker.compose.project"], inspect, imageInspect))
	}

	if len(items) == 0 {
		utils.LogInfo("未发现需检查的容器，扫描结束")
		return nil, nil
	}

	const maxConcurrent = 3
	sem := make(chan struct{}, maxConcurrent)
	resultCh := make(chan checkResult, len(items))

	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		go func(it checkItem) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			inspectRes, err := inspectRemoteDistribution(it.imageName, it.localPlatform)
			remoteDigest := ""
			remoteConfigDigest := ""
			if inspectRes != nil {
				remoteDigest = inspectRes.RemoteDigest
				remoteConfigDigest = inspectRes.RemoteConfigDigest
			}

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
			utils.LogWarning("获取容器 %s 的远程 Registry 镜像 Digest 失败: %s", res.item.containerName, res.err.Error())
			continue
		}

		hasUpdate := evaluatePassiveUpdateStatus(ctx, cli, res.item, res.remoteDigest, res.remoteConfigDigest)
		remoteVersion := ExtractSemanticVersion(nil, nil, res.item.imageName)

		if hasUpdate {
			logCheckResult(res.item.containerName, res.item, res.remoteDigest, remoteVersion, true)
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

	utils.LogInfo("容器扫描比对检查结束。当前共发现 %d 个待升级容器。", updateCount)
	return results, nil
}

// logCheckResult 统一记录容器版本比对结果日志
func logCheckResult(containerName string, item checkItem, remoteDigest string, remoteVersion string, hasUpdate bool) {
	if hasUpdate {
		localVerPart := ""
		if item.localVersion != "" {
			localVerPart = "version:" + item.localVersion + " "
		}
		remoteVerPart := ""
		if remoteVersion != "" {
			remoteVerPart = "version:" + remoteVersion + " "
		}
		localHash := strings.TrimPrefix(item.localDigest, "sha256:")
		remoteHash := strings.TrimPrefix(remoteDigest, "sha256:")

		utils.LogInfo("容器 %s 存在可用升级 (本地: %ssha256:%s, 远端: %ssha256:%s)",
			containerName, localVerPart, localHash, remoteVerPart, remoteHash)
	} else {
		utils.LogInfo("容器 %s 检查完成，已是最新版本", containerName)
	}
}

// GetRemoteTags 获取远端 Registry 镜像标签列表
func GetRemoteTags(imageName string) ([]string, error) {
	registry, repo, _ := parseImage(imageName)

	hosts := getRegistryHosts(registry)

	var lastErr error
	for _, host := range hosts {
		tags, err := getRemoteTagsWithHost(host, registry, repo)
		if err == nil {
			return tags, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

// getRemoteTagsWithHost 从指定 Host 获取标签列表
func getRemoteTagsWithHost(host, registry, repo string) ([]string, error) {
	endpoint := fmt.Sprintf("https://%s/v2/%s/tags/list", host, repo)
	client := &http.Client{Timeout: 10 * time.Second}

	req, err := http.NewRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		authChallenge := resp.Header.Get("WWW-Authenticate")
		headerName, headerVal, authErr := handleAuthChallenge(authChallenge, registry)
		if authErr == nil {
			resp.Body.Close()

			req, err = http.NewRequest("GET", endpoint, nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set(headerName, headerVal)
			resp, err = client.Do(req)
			if err != nil {
				return nil, err
			}
		} else {
			resp.Body.Close()
			return nil, authErr
		}
	}

	defer resp.Body.Close()

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

// ScanSingleContainer 扫描单个指定容器版本
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

	imageInspect, _, err := cli.ImageInspectWithRaw(ctx, inspect.Image)
	if err != nil {
		return nil, fmt.Errorf("无法获取镜像 %s 详情: %w", inspect.Config.Image, err)
	}

	composeProject := ""
	if inspect.Config.Labels != nil {
		composeProject = inspect.Config.Labels["com.docker.compose.project"]
	}

	item := buildCheckItem(targetName, composeProject, inspect, imageInspect)

	inspectRes, err := inspectRemoteDistribution(item.imageName, item.localPlatform)
	if err == nil && inspectRes != nil {
		hasUpdate := evaluatePassiveUpdateStatus(ctx, cli, item, inspectRes.RemoteDigest, inspectRes.RemoteConfigDigest)
		if !hasUpdate || item.rawRepoDigest != "" {
			remoteVersion := ExtractSemanticVersion(nil, nil, item.imageName)
			nowStr := time.Now().UTC().Format(time.RFC3339)
			logCheckResult(targetName, item, inspectRes.RemoteDigest, remoteVersion, hasUpdate)
			return &UpdateCheckResult{
				ContainerName:  targetName,
				Image:          item.imageName,
				LocalDigest:    item.localDigest,
				RemoteDigest:   inspectRes.RemoteDigest,
				LocalVersion:   item.localVersion,
				RemoteVersion:  remoteVersion,
				CheckedAt:      nowStr,
				ComposeProject: item.composeProject,
				HasUpdate:      hasUpdate,
			}, nil
		}
	}

	utils.LogInfo("单容器 %s 被动校验存疑，发起主动校验...", targetName)
	var tempMirrors []string
	tempMirrorsStr := db.GetSetting("temp_mirrors", "[]")
	_ = json.Unmarshal([]byte(tempMirrorsStr), &tempMirrors)

	reader, pullErr := pullImageWithMirrors(ctx, cli, item.imageName, tempMirrors, make(chan string, 100))
	if pullErr != nil {
		return nil, fmt.Errorf("主动检查 ImagePull 失败: %w", pullErr)
	}
	_ = reader.Close()

	newImageInspect, _, inspectErr := cli.ImageInspectWithRaw(ctx, item.imageName)
	if inspectErr != nil {
		return nil, fmt.Errorf("无法获取 Pull 后的最新镜像详情: %w", inspectErr)
	}

	hasUpdate := newImageInspect.ID != item.imageID
	remoteVersion := ExtractSemanticVersion(nil, nil, item.imageName)
	nowStr := time.Now().UTC().Format(time.RFC3339)
	logCheckResult(targetName, item, newImageInspect.ID, remoteVersion, hasUpdate)

	return &UpdateCheckResult{
		ContainerName:  targetName,
		Image:          item.imageName,
		LocalDigest:    item.localDigest,
		RemoteDigest:   newImageInspect.ID,
		LocalVersion:   item.localVersion,
		RemoteVersion:  remoteVersion,
		CheckedAt:      nowStr,
		ComposeProject: item.composeProject,
		HasUpdate:      hasUpdate,
	}, nil
}

// getRegistryHosts 获取目标 Registry 极其对应的镜像加速源 Host 列表
func getRegistryHosts(registry string) []string {
	var hosts []string
	if registry == "registry-1.docker.io" {
		var appMirrors []string
		tempMirrorsStr := db.GetSetting("temp_mirrors", "[]")
		_ = json.Unmarshal([]byte(tempMirrorsStr), &appMirrors)

		seen := make(map[string]bool)
		for _, m := range appMirrors {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if !seen[m] {
				seen[m] = true
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
	}
	hosts = append(hosts, registry)
	return hosts
}

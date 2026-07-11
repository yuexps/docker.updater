package dockerclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"docker-updater/db"
	"docker-updater/utils"

	"github.com/docker/docker/api/types"
)

// getSystemMirrors 读取宿主机 daemon.json 中配置的镜像加速源列表
func getSystemMirrors() []string {
	filePath := "/etc/docker/daemon.json"
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	var config struct {
		RegistryMirrors []string `json:"registry-mirrors"`
	}
	if err := json.Unmarshal(fileBytes, &config); err != nil {
		return nil
	}
	return config.RegistryMirrors
}

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

// getRemoteDigest 获取 Registry 镜像的最新摘要值（Digest），支持多镜像源加速轮询。
func getRemoteDigest(imageName string, localDigest string, localImageID string, localPlatform map[string]string, systemMirrors []string) (string, error) {
	registry, repo, tag := parseImage(imageName)

	var hosts []string
	if registry == "registry-1.docker.io" {
		var appMirrors []string
		tempMirrorsStr := db.GetSetting("temp_mirrors", "[]")
		_ = json.Unmarshal([]byte(tempMirrorsStr), &appMirrors)

		// 融合系统级与应用级配置的镜像加速器，并进行去重
		seen := make(map[string]bool)
		var mirrors []string
		for _, m := range append(systemMirrors, appMirrors...) {
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
		digest, err := getRemoteDigestWithHost(host, registry, repo, tag, localDigest, localImageID, localPlatform)
		if err == nil {
			return digest, nil
		}
		lastErr = err
		utils.LogWarning("使用镜像服务器 %s 获取 %s 的 Digest 失败: %s，正在尝试下一个...", host, imageName, err.Error())
	}
	return "", lastErr
}

// getRemoteDigestWithHost 基于指定的 Registry/Mirror 主机获取镜像摘要。
func getRemoteDigestWithHost(host, registry, repo, tag, localDigest, localImageID string, localPlatform map[string]string) (string, error) {
	endpoint := fmt.Sprintf("https://%s/v2/%s/manifests/%s", host, repo, tag)

	client := &http.Client{Timeout: 8 * time.Second} // 超时设为 8 秒
	req, err := http.NewRequest("HEAD", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", manifestAccept)

	// 1. 发送 HEAD 请求
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	var token string
	// 2. 如果未授权，获取 Bearer Token 并带 Token 重试 HEAD
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		token, err = getBearerToken(authHeader, registry) // 始终用原始 registry 查询本地凭证和获取 Token
		if err == nil {
			resp.Body.Close()

			req, err = http.NewRequest("HEAD", endpoint, nil)
			if err != nil {
				return "", err
			}
			req.Header.Set("Accept", manifestAccept)
			req.Header.Set("Authorization", "Bearer "+token)
			resp, err = client.Do(req)
			if err != nil {
				return "", err
			}
		} else {
			resp.Body.Close()
			return "", err
		}
	}

	// 3. 降级机制：如果 HEAD 请求没有返回 200 OK，则退避使用 GET 请求重试
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()

		req, err = http.NewRequest("GET", endpoint, nil)
		if err != nil {
			return "", err
		}
		req.Header.Set("Accept", manifestAccept)
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		resp, err = client.Do(req)
		if err != nil {
			return "", err
		}
	}

	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	remoteDigest := resp.Header.Get("Docker-Content-Digest")

	// 只要摘要不匹配，且本地配置了 localDigest，我们都尝试读取并解析 Manifest List（不再强依赖 Content-Type 头中必须包含特定关键字）
	if remoteDigest != "" && localDigest != "" && remoteDigest != localDigest {
		req, err = http.NewRequest("GET", endpoint, nil)
		if err == nil {
			req.Header.Set("Accept", manifestAccept)
			if token != "" {
				req.Header.Set("Authorization", "Bearer "+token)
			}
			getResp, err := client.Do(req)
			if err == nil {
				defer getResp.Body.Close()
				if getResp.StatusCode == http.StatusOK {
					var manifestList struct {
						Manifests []struct {
							Digest   string            `json:"digest"`
							Platform map[string]string `json:"platform"`
						} `json:"manifests"`
					}
					// 尝试解析 JSON，如果成功解码且确实是一个多架构 Index 列表
					if err := json.NewDecoder(getResp.Body).Decode(&manifestList); err == nil && len(manifestList.Manifests) > 0 {
						for _, m := range manifestList.Manifests {
							if m.Digest == localDigest {
								return localDigest, nil
							}
						}
						if localImageID != "" && localPlatform != nil {
							for _, m := range manifestList.Manifests {
								if m.Platform != nil && m.Platform["os"] == localPlatform["os"] &&
									m.Platform["architecture"] == localPlatform["architecture"] {
									return m.Digest, nil
								}
							}
						}
					}
				}
			}
		}
	}

	return remoteDigest, nil
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

// checkItem 待检查容器项结构体。
type checkItem struct {
	containerName  string
	composeProject string
	imageName      string
	imageID        string
	localDigest    string
	localPlatform  map[string]string
}

// checkResult 镜像检测结果。
type checkResult struct {
	item         checkItem
	remoteDigest string
	err          error
}

// UpdateCheckResult 容器版本检测结果。
type UpdateCheckResult struct {
	ContainerName  string
	Image          string
	LocalDigest    string
	RemoteDigest   string
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

	var items []checkItem
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if name == "" || strings.HasSuffix(name, "_backup_docker_updater") {
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

		localDigest := ""
		if len(imageInspect.RepoDigests) > 0 {
			localDigest = getLocalDigestFromImage(imageInspect.RepoDigests, imageName)
		}

		items = append(items, checkItem{
			containerName:  name,
			composeProject: c.Labels["com.docker.compose.project"],
			imageName:      imageName,
			imageID:        imageInspect.ID,
			localDigest:    localDigest,
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

	systemMirrors := getSystemMirrors()

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

			remoteDigest, err := getRemoteDigest(it.imageName, it.localDigest, it.imageID, it.localPlatform, systemMirrors)
			resultCh <- checkResult{item: it, remoteDigest: remoteDigest, err: err}
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
			if res.item.localDigest != "" {
				hasUpdate = res.item.localDigest != res.remoteDigest
			} else {
				// 如果本地无 RepoDigests 记录，则向本地 Docker 引擎查询是否已存在此 remoteDigest 的镜像
				// 若找不到，则判定有升级
				_, _, err = cli.ImageInspectWithRaw(ctx, res.remoteDigest)
				if err != nil {
					hasUpdate = true
				}
			}
		}

		if hasUpdate {
			utils.LogInfo("服务 %s 存在可用升级 (本地: %s, 远端: %s)",
				res.item.containerName, res.item.localDigest, res.remoteDigest)
			updateCount++
		}

		results = append(results, UpdateCheckResult{
			ContainerName:  res.item.containerName,
			Image:          res.item.imageName,
			LocalDigest:    res.item.localDigest,
			RemoteDigest:   res.remoteDigest,
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

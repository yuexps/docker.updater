package dockerclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

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

// getRemoteDigest 获取 Registry 镜像的最新摘要值（Digest）。
func getRemoteDigest(imageName string, localDigest string, localImageID string, localPlatform map[string]string) (string, error) {
	registry, repo, tag := parseImage(imageName)
	endpoint := fmt.Sprintf("https://%s/v2/%s/manifests/%s", registry, repo, tag)

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("HEAD", endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", manifestAccept)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		token, err := getBearerToken(authHeader, registry)
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

	if resp.StatusCode == http.StatusMethodNotAllowed {
		resp.Body.Close()

		req.Method = "GET"
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

	contentType := resp.Header.Get("Content-Type")
	if remoteDigest != "" && localDigest != "" && remoteDigest != localDigest &&
		(strings.Contains(contentType, "manifest.list") || strings.Contains(contentType, "image.index")) {
		req.Method = "GET"
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
				if err := json.NewDecoder(getResp.Body).Decode(&manifestList); err == nil {
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
		if name == "" || strings.HasSuffix(name, "_old") {
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
		if len(imageInspect.RepoDigests) == 0 {
			utils.LogInfo("容器 %s 依赖的镜像 %s 为本地构建镜像，无 RepoDigests 摘要，自动跳过版本检测", name, imageName)
			continue
		}

		localDigest := getLocalDigestFromImage(imageInspect.RepoDigests, imageName)
		if localDigest == "" {
			utils.LogWarning("容器 %s: 无法从 RepoDigests 匹配解析出本地镜像摘要: %s", name, imageName)
			continue
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

			remoteDigest, err := getRemoteDigest(it.imageName, it.localDigest, it.imageID, it.localPlatform)
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

		hasUpdate := res.remoteDigest != "" && res.item.localDigest != res.remoteDigest
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

package dockerclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"

	"docker-updater/db"

	"github.com/docker/docker/api/types"
)

const manifestAccept = "application/vnd.docker.distribution.manifest.list.v2+json, " +
	"application/vnd.docker.distribution.manifest.v2+json, " +
	"application/vnd.oci.image.index.v1+json, " +
	"application/vnd.oci.image.manifest.v1+json"

// parseImage 解析镜像名，返回 registry, repository, tag
func parseImage(imageName string) (string, string, string) {
	tag := "latest"
	name := imageName

	// 去除 digest 形式 (@sha256:...)
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

	// 识别是否包含自定义 registry 域名
	if strings.Contains(first, ".") || strings.Contains(first, ":") || first == "localhost" {
		return first, strings.Join(parts[1:], "/"), tag
	}

	// 默认 Docker Hub 官方镜像源
	repo := name
	if !strings.Contains(name, "/") {
		repo = "library/" + name
	}
	return "registry-1.docker.io", repo, tag
}

// getDockerRepoCandidates 获取匹配的 Repo 别名候选集
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

// parseChallengeHeader 解析 WWW-Authenticate 响应头中的 realm/service/scope
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

// getBearerToken 发生 401 质询时获取 Bearer Token
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

	// 检索本地 SQLite 中存储的该私有仓的账户密码凭据
	var cred db.RegistryCredential
	if err := db.DB.First(&cred, "registry = ?", registry).Error; err == nil {
		authVal := cred.Username + ":" + cred.Password
		req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(authVal)))
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

// getRemoteDigest 获取仓库对应镜像的最新 Digest
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
	defer resp.Body.Close()

	// 发生 401 认证挑战则请求 token 重试
	if resp.StatusCode == http.StatusUnauthorized {
		authHeader := resp.Header.Get("WWW-Authenticate")
		token, err := getBearerToken(authHeader, registry)
		if err == nil {
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
			defer resp.Body.Close()
		}
	}

	// 部分仓库不支持 HEAD 则降级为 GET
	if resp.StatusCode == http.StatusMethodNotAllowed {
		req.Method = "GET"
		resp, err = client.Do(req)
		if err != nil {
			return "", err
		}
		defer resp.Body.Close()
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %s", resp.Status)
	}

	remoteDigest := resp.Header.Get("Docker-Content-Digest")

	// 处理多架构 manifest.list 清单列表
	contentType := resp.Header.Get("Content-Type")
	if remoteDigest != "" && localDigest != "" && remoteDigest != localDigest &&
		(strings.Contains(contentType, "manifest.list") || strings.Contains(contentType, "image.index")) {
		// 需要 GET 完整的 manifest 清单来解析对应的架构
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
							return localDigest, nil // 匹配本地已使用的架构
						}
					}
					// 如未直接匹配，则尝试根据本地 OS/Architecture 结构查找远端对应的平台 Digest
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



// getLocalDigestFromImage 通过镜像 inspect 详情解析
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
	// 默认返回第一个
	parts := strings.SplitN(repoDigests[0], "@", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

// checkItem 单个容器的待检查数据
type checkItem struct {
	containerName  string
	composeProject string
	imageName      string
	imageID        string
	localDigest    string
	localPlatform  map[string]string
}

// checkResult 并发检查的结果
type checkResult struct {
	item         checkItem
	remoteDigest string
	err          error
}

// ScanLocalHostForUpdates 扫描本地所有运行中容器进行版本检查，并将结果写入 SQLite
// 本地 inspect 串行执行，远端 Registry HTTP 检查并发执行（最多 5 个并发）
func ScanLocalHostForUpdates(ctx context.Context) error {
	log.Println("[INFO] 开始扫描本地活动容器进行版本比对检查...")

	cli, err := NewLocalClient()
	if err != nil {
		log.Printf("[ERROR] 建立 Docker 引擎连接失败: %s\n", err.Error())
		return err
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		log.Printf("[ERROR] 获取本地容器列表失败: %s\n", err.Error())
		return err
	}

	nowStr := time.Now().UTC().Format(time.RFC3339)

	// 阶段一：串行 inspect，收集所有待检查项（本地 Unix Socket 无需并发）
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
			log.Printf("[WARNING] 无法获取容器 %s 详情: %s\n", name, err.Error())
			continue
		}

		imageName := inspect.Config.Image
		imageInspect, _, err := cli.ImageInspectWithRaw(ctx, inspect.Image)
		if err != nil {
			log.Printf("[WARNING] 无法获取镜像 %s 的 inspect 详情: %s\n", imageName, err.Error())
			continue
		}
		if len(imageInspect.RepoDigests) == 0 {
			continue // 本地构建镜像，跳过
		}

		localDigest := getLocalDigestFromImage(imageInspect.RepoDigests, imageName)
		if localDigest == "" {
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
		log.Println("[INFO] 未发现需检查的容器，扫描结束")
		return nil
	}

	// 阶段二：并发 HTTP 请求远端 Registry（信号量限制最多 5 并发）
	const maxConcurrent = 5
	sem := make(chan struct{}, maxConcurrent)
	resultCh := make(chan checkResult, len(items))

	var wg sync.WaitGroup
	for _, item := range items {
		wg.Add(1)
		go func(it checkItem) {
			defer wg.Done()
			sem <- struct{}{}        // 占用信号量
			defer func() { <-sem }() // 释放信号量

			remoteDigest, err := getRemoteDigest(it.imageName, it.localDigest, it.imageID, it.localPlatform)
			resultCh <- checkResult{item: it, remoteDigest: remoteDigest, err: err}
		}(item)
	}

	// 等待所有 goroutine 完成后关闭结果通道
	go func() {
		wg.Wait()
		close(resultCh)
	}()

	// 阶段三：收集结果并串行写入 SQLite
	var updateCount int
	for res := range resultCh {
		if res.err != nil {
			log.Printf("[WARNING] 获取服务 %s 的远程 Registry 镜像 Digest 失败: %s\n", res.item.containerName, res.err.Error())
			continue
		}

		hasUpdate := res.remoteDigest != "" && res.item.localDigest != res.remoteDigest
		updateEntry := db.AvailableUpdate{
			ContainerName:  res.item.containerName,
			Image:          res.item.imageName,
			LocalDigest:    res.item.localDigest,
			RemoteDigest:   res.remoteDigest,
			CheckedAt:      nowStr,
			ComposeProject: res.item.composeProject,
		}

		if hasUpdate {
			db.DB.Save(&updateEntry)
			log.Printf("[INFO] 服务 %s 存在可用升级 (本地: %s, 远端: %s)\n",
				res.item.containerName, res.item.localDigest, res.remoteDigest)
			updateCount++
		} else {
			db.DB.Delete(&db.AvailableUpdate{ContainerName: res.item.containerName})
		}
	}

	log.Printf("[INFO] 容器扫描比对检查结束。当前共发现 %d 个待升级服务。\n", updateCount)
	return nil
}

package dockerclient

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
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
	"github.com/google/go-containerregistry/pkg/authn"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/remote"
)

// CredentialsProvider 凭证获取接口
type CredentialsProvider interface {
	GetCredential(ctx context.Context, registry string) (username, password string, ok bool)
}

// GlobalCredentialsProvider 全局凭证提供者
var GlobalCredentialsProvider CredentialsProvider

// parseImage 使用开源库解析镜像名称中的 registry, repo 和 tag
func parseImage(imageName string) (registry string, repo string, tag string) {
	ref, err := name.ParseReference(imageName)
	if err != nil {
		return "registry-1.docker.io", imageName, "latest"
	}
	return ref.Context().RegistryStr(), ref.Context().RepositoryStr(), ref.Identifier()
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
		parts := strings.Split(imageName, ":")
		if len(parts) > 1 {
			tag := parts[len(parts)-1]
			if idx := strings.Index(tag, "@"); idx != -1 {
				tag = tag[:idx]
			}
			if tag != "" && tag != "latest" && tag != "main" && tag != "master" && tag != "dev" && tag != "test" && tag != "stable" {
				match := semverRegexp.FindString(tag)
				if match != "" {
					return match
				}
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
	CheckMethod    string
}

// checkItem 待检测容器结构体
type checkItem struct {
	containerName  string
	composeProject string
	imageName      string
	localVersion   string
	imageInspect   types.ImageInspect
}

// checkResult 镜像检测结果
type checkResult struct {
	item          checkItem
	localDigest   string
	remoteDigest  string
	remoteVersion string
	hasUpdate     bool
	checkMethod   string
	err           error
}

// getGCRAuth 获取 gcr 鉴权凭证
func getGCRAuth(ctx context.Context, registry string) authn.Authenticator {
	if GlobalCredentialsProvider != nil {
		if username, password, ok := GlobalCredentialsProvider.GetCredential(ctx, registry); ok {
			return &authn.Basic{Username: username, Password: password}
		}
	}
	return authn.Anonymous
}

func getStandardTransport() http.RoundTripper {
	dialer := &net.Dialer{
		Timeout:   8 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	return &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, "tcp4", addr)
		},
		TLSHandshakeTimeout: 8 * time.Second,
	}
}

// getCandidateReferences 生成镜像源候选 Reference 列表
func getCandidateReferences(imageName string) []name.Reference {
	var candidates []name.Reference

	ref, err := name.ParseReference(imageName)
	if err != nil {
		return candidates
	}

	reg := ref.Context().RegistryStr()
	repoPath := ref.Context().RepositoryStr()
	tagOrDigest := ref.Identifier()

	isDockerHub := reg == "index.docker.io" || reg == "docker.io" || reg == "registry-1.docker.io"
	isGHCR := reg == "ghcr.io"

	var mirrors []string
	if isDockerHub {
		mirrorsStr := db.GetSetting("dockerhub_mirrors", "[]")
		_ = json.Unmarshal([]byte(mirrorsStr), &mirrors)
	} else if isGHCR {
		mirrorsStr := db.GetSetting("ghcr_mirrors", "[]")
		_ = json.Unmarshal([]byte(mirrorsStr), &mirrors)
	}

	for _, m := range mirrors {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		u, parseErr := url.Parse(m)
		host := m
		if parseErr == nil && u.Host != "" {
			host = u.Host
		} else {
			host = strings.TrimPrefix(host, "https://")
			host = strings.TrimPrefix(host, "http://")
			host = strings.TrimSuffix(host, "/")
		}
		if host != "" {
			mirrorRefStr := fmt.Sprintf("%s/%s:%s", host, repoPath, tagOrDigest)
			if mRef, mErr := name.ParseReference(mirrorRefStr); mErr == nil {
				candidates = append(candidates, mRef)
			}
		}
	}

	candidates = append(candidates, ref)
	return candidates
}

// checkImageUpdate 执行镜像更新比对
func checkImageUpdate(ctx context.Context, cli *client.Client, imageName string, imageInspect types.ImageInspect) (localDigest string, remoteDigest string, remoteVersion string, hasUpdate bool, checkMethod string, err error) {
	transport := getStandardTransport()
	candidateRefs := getCandidateReferences(imageName)

	platform := v1.Platform{
		Architecture: imageInspect.Architecture,
		OS:           imageInspect.Os,
		Variant:      imageInspect.Variant,
	}

	if len(imageInspect.RepoDigests) > 0 {
		localDigestsSet := make(map[string]bool)
		for _, rd := range imageInspect.RepoDigests {
			if parts := strings.SplitN(rd, "@", 2); len(parts) == 2 {
				cleanHash := strings.TrimPrefix(parts[1], "sha256:")
				localDigestsSet[cleanHash] = true
				if localDigest == "" {
					localDigest = parts[1]
				}
			}
		}

		authHeader := getAuthHeader(imageName)
		distInspect, distErr := cli.DistributionInspect(ctx, imageName, authHeader)
		if distErr == nil && distInspect.Descriptor.Digest.String() != "" {
			remoteDigest = distInspect.Descriptor.Digest.String()
			cleanRemote := strings.TrimPrefix(remoteDigest, "sha256:")
			hasUpdate = !localDigestsSet[cleanRemote]
			return localDigest, remoteDigest, "", hasUpdate, "RepoDigest", nil
		}

		var lastErr error = distErr
		for _, cRef := range candidateRefs {
			subCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
			auth := getGCRAuth(subCtx, cRef.Context().RegistryStr())
			desc, headErr := remote.Head(cRef, remote.WithAuth(auth), remote.WithTransport(transport), remote.WithContext(subCtx))
			if headErr == nil && desc.Digest.String() != "" {
				remoteDigest = desc.Digest.String()
				cleanRemote := strings.TrimPrefix(remoteDigest, "sha256:")
				hasUpdate = !localDigestsSet[cleanRemote]
				cancel()
				return localDigest, remoteDigest, "", hasUpdate, "RepoDigest", nil
			}
			if headErr != nil {
				lastErr = headErr
			}
			cancel()
		}

		return "", "", "", false, "RepoDigest", fmt.Errorf("RepoDigest 检测失败: %w", lastErr)
	}

	localDigest = imageInspect.ID
	if len(candidateRefs) == 0 {
		return "", "", "", false, "ConfigDigest", fmt.Errorf("解析镜像引用失败")
	}

	var lastConfigErr error
	for _, cRef := range candidateRefs {
		subCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		auth := getGCRAuth(subCtx, cRef.Context().RegistryStr())
		opts := []remote.Option{
			remote.WithAuth(auth),
			remote.WithPlatform(platform),
			remote.WithTransport(transport),
			remote.WithContext(subCtx),
		}

		img, imgErr := remote.Image(cRef, opts...)
		if imgErr != nil {
			lastConfigErr = imgErr
			cancel()
			continue
		}

		manifest, manErr := img.Manifest()
		if manErr != nil {
			lastConfigErr = manErr
			cancel()
			continue
		}

		remoteDigest = manifest.Config.Digest.String()
		cleanLocal := strings.TrimPrefix(localDigest, "sha256:")
		cleanRemote := strings.TrimPrefix(remoteDigest, "sha256:")
		hasUpdate = (cleanLocal != cleanRemote)

		if cfg, cfgErr := img.ConfigFile(); cfgErr == nil && cfg != nil {
			remoteVersion = ExtractSemanticVersion(cfg.Config.Labels, cfg.Config.Env, imageName)
		}

		cancel()
		return localDigest, remoteDigest, remoteVersion, hasUpdate, "ConfigDigest", nil
	}

	return localDigest, "", remoteVersion, false, "ConfigDigest", fmt.Errorf("Config Digest 降级检测失败: %w", lastConfigErr)
}

// logCheckResult 记录容器版本比对日志
func logCheckResult(containerName, checkMethod, localDigest, remoteDigest, localVersion, remoteVersion string, hasUpdate bool) {
	methodTag := fmt.Sprintf("[%s]", checkMethod)
	if checkMethod == "" {
		methodTag = "[RepoDigest]"
	}

	localHash := strings.TrimPrefix(localDigest, "sha256:")
	remoteHash := strings.TrimPrefix(remoteDigest, "sha256:")

	statusText := "检查完成，已是最新版本"
	if hasUpdate {
		statusText = "存在可用升级"
	}

	if localVersion != "" && remoteVersion != "" {
		utils.LogInfo("容器 %s %s %s (本地: version:%s sha256:%s, 远端: version:%s sha256:%s)",
			containerName, methodTag, statusText, localVersion, localHash, remoteVersion, remoteHash)
	} else {
		utils.LogInfo("容器 %s %s %s (本地: sha256:%s, 远端: sha256:%s)",
			containerName, methodTag, statusText, localHash, remoteHash)
	}
}

// ScanLocalHostForUpdates 扫描本地活动容器镜像版本
func ScanLocalHostForUpdates(ctx context.Context, onResult func(UpdateCheckResult)) error {
	utils.LogInfo("开始全量扫描本地活动容器...")

	cli, err := NewLocalClient()
	if err != nil {
		utils.LogError("建立 Docker 引擎连接失败: %s", err.Error())
		return err
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
	if err != nil {
		utils.LogError("获取本地容器列表失败: %s", err.Error())
		return err
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
		if name == "" {
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

		localVersion := ExtractSemanticVersion(imageInspect.Config.Labels, imageInspect.Config.Env, inspect.Config.Image)
		if localVersion == "" && inspect.Config != nil {
			localVersion = ExtractSemanticVersion(inspect.Config.Labels, inspect.Config.Env, inspect.Config.Image)
		}
		composeProject := ""
		if inspect.Config.Labels != nil {
			composeProject = inspect.Config.Labels["com.docker.compose.project"]
		}

		items = append(items, checkItem{
			containerName:  name,
			composeProject: composeProject,
			imageName:      inspect.Config.Image,
			localVersion:   localVersion,
			imageInspect:   imageInspect,
		})
	}

	if len(items) == 0 {
		utils.LogInfo("未发现需检查的容器，扫描结束")
		return nil
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

			locDigest, remDigest, remVersion, hasUp, cMethod, checkErr := checkImageUpdate(ctx, cli, it.imageName, it.imageInspect)
			resultCh <- checkResult{
				item:          it,
				localDigest:   locDigest,
				remoteDigest:  remDigest,
				remoteVersion: remVersion,
				hasUpdate:     hasUp,
				checkMethod:   cMethod,
				err:           checkErr,
			}
		}(item)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var updateCount int
	for res := range resultCh {
		if res.err != nil {
			utils.LogWarning("获取容器 %s 的远程 Registry 镜像 Digest 失败: %s", res.item.containerName, res.err.Error())
			continue
		}

		remoteVersion := res.remoteVersion
		logCheckResult(res.item.containerName, res.checkMethod, res.localDigest, res.remoteDigest, res.item.localVersion, remoteVersion, res.hasUpdate)
		if res.hasUpdate {
			updateCount++
		}

		if onResult != nil {
			onResult(UpdateCheckResult{
				ContainerName:  res.item.containerName,
				Image:          res.item.imageName,
				LocalDigest:    res.localDigest,
				RemoteDigest:   res.remoteDigest,
				LocalVersion:   res.item.localVersion,
				RemoteVersion:  remoteVersion,
				CheckedAt:      nowStr,
				ComposeProject: res.item.composeProject,
				HasUpdate:      res.hasUpdate,
				CheckMethod:    res.checkMethod,
			})
		}
	}

	utils.LogInfo("容器扫描比对检查结束。当前共发现 %d 个待升级容器。", updateCount)
	return nil
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

	localVersion := ExtractSemanticVersion(imageInspect.Config.Labels, imageInspect.Config.Env, inspect.Config.Image)
	locDigest, remDigest, remVersion, hasUpdate, cMethod, checkErr := checkImageUpdate(ctx, cli, inspect.Config.Image, imageInspect)
	if checkErr != nil {
		return nil, checkErr
	}

	remoteVersion := remVersion
	nowStr := time.Now().Format("2006-01-02 15:04:05")
	logCheckResult(targetName, cMethod, locDigest, remDigest, localVersion, remoteVersion, hasUpdate)

	return &UpdateCheckResult{
		ContainerName:  targetName,
		Image:          inspect.Config.Image,
		LocalDigest:    locDigest,
		RemoteDigest:   remDigest,
		LocalVersion:   localVersion,
		RemoteVersion:  remoteVersion,
		CheckedAt:      nowStr,
		ComposeProject: composeProject,
		HasUpdate:      hasUpdate,
		CheckMethod:    cMethod,
	}, nil
}

// GetRemoteTags 获取远端 Registry 镜像标签列表
func GetRemoteTags(ctx context.Context, imageName string) ([]string, error) {
	if _, err := name.ParseReference(imageName); err != nil {
		return nil, fmt.Errorf("解析镜像名称失败: %w", err)
	}

	candidateRefs := getCandidateReferences(imageName)
	transport := getStandardTransport()

	var tags []string
	var lastErr error

	for _, cRef := range candidateRefs {
		subCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		candRepo := cRef.Context()
		auth := getGCRAuth(subCtx, candRepo.RegistryStr())

		tList, err := remote.List(candRepo, remote.WithAuth(auth), remote.WithContext(subCtx), remote.WithTransport(transport))
		cancel()
		if err == nil && len(tList) > 0 {
			tags = tList
			lastErr = nil
			break
		}
		if err != nil {
			lastErr = err
		}
	}

	if len(tags) == 0 && lastErr != nil {
		return nil, fmt.Errorf("获取远端标签列表失败: %w", lastErr)
	}

	var filtered []string
	for _, t := range tags {
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

package dockerclient

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	specs "github.com/opencontainers/image-spec/specs-go/v1"
)

// UpdateOptions 容器更新参数选项
type UpdateOptions struct {
	DockerhubMirrors []string
	GHCRMirrors      []string
}

// pullImageWithMirrors 拉取镜像
func pullImageWithMirrors(ctx context.Context, cli *client.Client, imageName string, opts UpdateOptions, logChan chan<- string) (io.ReadCloser, error) {
	authHeader := getAuthHeader(imageName)
	pullOpts := types.ImagePullOptions{RegistryAuth: authHeader}

	registry, repo, tag := parseImage(imageName)
	isDockerHub := (registry == "registry-1.docker.io" || registry == "docker.io" || registry == "index.docker.io")
	isGHCR := (registry == "ghcr.io")

	var targetMirrors []string
	var registryName string

	if isDockerHub {
		targetMirrors = opts.DockerhubMirrors
		registryName = "DockerHub"
	} else if isGHCR {
		targetMirrors = opts.GHCRMirrors
		registryName = "GHCR"
	}

	if len(targetMirrors) > 0 {
		seen := make(map[string]bool)
		for _, m := range targetMirrors {
			m = strings.TrimSpace(m)
			if m == "" || seen[m] {
				continue
			}
			seen[m] = true

			mirrorHost := strings.TrimPrefix(m, "https://")
			mirrorHost = strings.TrimPrefix(mirrorHost, "http://")
			mirrorHost = strings.TrimSuffix(mirrorHost, "/")

			var tempImageName string
			fullPath := repo
			if tag != "" && tag != "latest" {
				fullPath = repo + ":" + tag
			}
			tempImageName = fmt.Sprintf("%s/%s", mirrorHost, fullPath)

			cleanTempTag := func() {
				_, _ = cli.ImageRemove(ctx, tempImageName, types.ImageRemoveOptions{PruneChildren: false})
			}

			logChan <- fmt.Sprintf("[INFO] 检测到 %s 镜像，尝试通过镜像源 %s 加速拉取...", registryName, mirrorHost)

			reader, err := cli.ImagePull(ctx, tempImageName, pullOpts)
			if err != nil {
				logChan <- fmt.Sprintf("[WARN] 通过加速源 %s 拉取失败: %s. 尝试下一个...", mirrorHost, err.Error())
				cleanTempTag()
				continue
			}

			consumePullProgress(reader, logChan)
			_ = reader.Close()

			logChan <- fmt.Sprintf("[INFO] 临时加速拉取成功，正在为镜像打标回官方原名: %s", imageName)
			if err := cli.ImageTag(ctx, tempImageName, imageName); err != nil {
				logChan <- fmt.Sprintf("[WARN] 打标回原名失败: %s", err.Error())
				cleanTempTag()
				continue
			}

			logChan <- fmt.Sprintf("[INFO] 正在清理临时镜像源冗余标签: %s", tempImageName)
			cleanTempTag()

			return io.NopCloser(strings.NewReader(`{"status":"Success"}`)), nil
		}
		logChan <- "[WARN] 所有配置的镜像源均已尝试拉取失败，将降级为官方直接拉取..."
	}

	return cli.ImagePull(ctx, imageName, pullOpts)
}

// getAuthHeader 获取对应镜像 Registry 的 Base64 Auth 凭证
func getAuthHeader(imageName string) string {
	if GlobalCredentialsProvider == nil {
		return ""
	}
	registry, _, _ := parseImage(imageName)
	if username, password, ok := GlobalCredentialsProvider.GetCredential(context.Background(), registry); ok {
		authConfig := map[string]string{
			"username": username,
			"password": password,
		}
		if authBytes, err := json.Marshal(authConfig); err == nil {
			return base64.StdEncoding.EncodeToString(authBytes)
		}
	}
	return ""
}

// consumePullProgress 解析 Docker API 拉取流进度并输出日志
func consumePullProgress(reader io.Reader, logChan chan<- string) {
	dec := json.NewDecoder(reader)
	for {
		var pullStatus map[string]interface{}
		if err := dec.Decode(&pullStatus); err != nil {
			break
		}
		statusMsg, _ := pullStatus["status"].(string)
		progressDetail, _ := pullStatus["progress"].(string)
		if statusMsg != "" {
			if progressDetail != "" {
				logChan <- fmt.Sprintf("[INFO] %s %s", statusMsg, progressDetail)
			} else {
				logChan <- fmt.Sprintf("[INFO] %s", statusMsg)
			}
		}
	}
}

// getTempProtectTag 生成旧镜像临时保护 Tag 名称
func getTempProtectTag(imageName, containerName string) string {
	parts := strings.Split(imageName, ":")
	baseImage := parts[0]
	if strings.Contains(baseImage, "@") {
		baseImage = strings.Split(baseImage, "@")[0]
	}
	cleanName := strings.ReplaceAll(containerName, "/", "_")
	return fmt.Sprintf("%s:du_bak_%s", baseImage, cleanName)
}

// ApplyUpdate 拉取镜像并重建容器 (Watchtower/Portainer 纯 Spec 克隆极速重建模式)
func ApplyUpdate(ctx context.Context, name string, targetImage string, opts UpdateOptions, logChan chan<- string) error {
	logChan <- fmt.Sprintf("[INFO] 开始修改容器版本: %s", name)

	cli, err := NewLocalClient()
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 无法连接 Docker 引擎: %s", err.Error())
		return err
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(ctx, name)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 找不到容器 %s: %s", name, err.Error())
		return err
	}

	oldID := inspect.ID
	oldImageID := inspect.Image
	imageName := inspect.Config.Image

	if targetImage != "" {
		if strings.Contains(targetImage, ":") {
			imageName = targetImage
		} else {
			parts := strings.Split(inspect.Config.Image, ":")
			baseImage := parts[0]
			if strings.Contains(baseImage, "@") {
				baseImage = strings.Split(baseImage, "@")[0]
			}
			imageName = baseImage + ":" + targetImage
		}
		logChan <- fmt.Sprintf("[INFO] 指定目标修改版本/镜像: %s", imageName)
	}

	logChan <- fmt.Sprintf("[INFO] 容器名: %s", name)
	logChan <- fmt.Sprintf("[INFO] 镜像源: %s", imageName)

	// 为旧镜像绑定临时保护标签，防止新镜像 Pull 后旧镜像脱标沦为 <none>
	tempBakTag := getTempProtectTag(imageName, name)
	if err := cli.ImageTag(ctx, oldImageID, tempBakTag); err != nil {
		logChan <- fmt.Sprintf("[WARN] 为旧镜像绑定临时保护标签失败: %s", err.Error())
	} else {
		logChan <- fmt.Sprintf("[INFO] 已成功为旧镜像绑定临时保护标签: %s", tempBakTag)
	}
	defer func() {
		_, _ = cli.ImageRemove(ctx, tempBakTag, types.ImageRemoveOptions{PruneChildren: false})
	}()

	// 解析保存原始容器的全量网络 EndpointSettings
	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: make(map[string]*network.EndpointSettings),
	}
	for netName, netConfig := range inspect.NetworkSettings.Networks {
		var ipamConfig *network.EndpointIPAMConfig
		if netConfig.IPAMConfig != nil {
			ipamConfig = &network.EndpointIPAMConfig{
				IPv4Address: netConfig.IPAMConfig.IPv4Address,
				IPv6Address: netConfig.IPAMConfig.IPv6Address,
			}
		}
		networkingConfig.EndpointsConfig[netName] = &network.EndpointSettings{
			IPAMConfig: ipamConfig,
			Aliases:    netConfig.Aliases,
		}
	}

	logChan <- "[INFO] 正在拉取最新镜像..."
	reader, err := pullImageWithMirrors(ctx, cli, imageName, opts, logChan)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 镜像拉取失败: %s", err.Error())
		return err
	}
	defer reader.Close()

	consumePullProgress(reader, logChan)

	var newImageID string
	var targetPlatform *specs.Platform
	if newImageInspect, _, inspectErr := cli.ImageInspectWithRaw(ctx, imageName); inspectErr == nil {
		newImageID = newImageInspect.ID
		if newImageInspect.Os != "" && newImageInspect.Architecture != "" {
			targetPlatform = &specs.Platform{
				OS:           newImageInspect.Os,
				Architecture: newImageInspect.Architecture,
				Variant:      newImageInspect.Variant,
			}
		}
	}

	logChan <- "[INFO] 正在停止旧容器..."
	stopTimeout := 30
	_ = cli.ContainerStop(ctx, oldID, container.StopOptions{Timeout: &stopTimeout})

	logChan <- "[INFO] 正在移除旧容器以释放名称与网络资源..."
	err = cli.ContainerRemove(ctx, oldID, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 移除旧容器失败: %s", err.Error())
		return err
	}

	// 若 Config.Hostname 为自动生成的 12 位 Short ID，自动清洗重置
	if len(oldID) >= 12 && strings.HasPrefix(inspect.Config.Hostname, oldID[:12]) {
		inspect.Config.Hostname = ""
	}

	logChan <- "[INFO] 正在使用新镜像重建容器..."
	inspect.Config.Image = imageName
	newContainer, err := cli.ContainerCreate(
		ctx,
		inspect.Config,
		inspect.HostConfig,
		networkingConfig,
		targetPlatform,
		name,
	)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 创建新容器失败: %s, 正在触发回滚...", err.Error())
		_rollback(ctx, cli, oldImageID, newImageID, imageName, tempBakTag, name, inspect, networkingConfig, logChan)
		return err
	}

	logChan <- "[INFO] 启动新容器..."
	err = cli.ContainerStart(ctx, newContainer.ID, types.ContainerStartOptions{})
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 启动新容器失败: %s, 正在触发回滚...", err.Error())
		_ = cli.ContainerRemove(ctx, newContainer.ID, types.ContainerRemoveOptions{Force: true})
		_rollback(ctx, cli, oldImageID, newImageID, imageName, tempBakTag, name, inspect, networkingConfig, logChan)
		return err
	}

	logChan <- "[INFO] 正在观察新容器运行稳定性 (6s)..."
	stayUp := true
	var checkErr error
	for i := 1; i <= 6; i++ {
		time.Sleep(1 * time.Second)
		newInspect, err := cli.ContainerInspect(ctx, name)
		if err != nil {
			checkErr = err
			stayUp = false
			break
		}
		if newInspect.State.Restarting || (newInspect.State.ExitCode != 0 && !newInspect.State.Running) {
			checkErr = fmt.Errorf("容器异常退出或陷入异常重启 (Status: %s, ExitCode: %d)", newInspect.State.Status, newInspect.State.ExitCode)
			stayUp = false
			break
		}
		if !newInspect.State.Running {
			if newInspect.State.ExitCode == 0 && newInspect.State.Status == "exited" {
				logChan <- "[INFO] 探测到单次任务容器已正常执行完毕退出 (ExitCode: 0)"
				break
			}
			checkErr = fmt.Errorf("容器未持续正常运行 (Status: %s)", newInspect.State.Status)
			stayUp = false
			break
		}
	}
	if !stayUp {
		reason := "容器未持续运行或已退出"
		if checkErr != nil {
			reason = checkErr.Error()
		}
		logChan <- fmt.Sprintf("[ERROR] 新容器健康检查失败: %s, 正在自动回滚...", reason)
		_ = cli.ContainerRemove(ctx, newContainer.ID, types.ContainerRemoveOptions{Force: true})
		_rollback(ctx, cli, oldImageID, newImageID, imageName, tempBakTag, name, inspect, networkingConfig, logChan)
		return fmt.Errorf("container failed to stay up: %s", reason)
	}

	logChan <- "[INFO] 容器版本修改成功并运行中"
	logChan <- "[INFO] 容器版本修改任务全部完成"
	return nil
}

// _rollback 使用旧镜像 Spec 重建并还原旧容器
func _rollback(ctx context.Context, cli *client.Client, oldImageID, newImageID, imageName, tempBakTag, name string, inspect types.ContainerJSON, networkingConfig *network.NetworkingConfig, logChan chan<- string) {
	logChan <- "[INFO] 正在执行回滚，重建恢复原服务..."

	// 1. 将原始 Tag 重新绑定给旧 Image ID
	if oldImageID != "" && imageName != "" {
		if tagErr := cli.ImageTag(ctx, oldImageID, imageName); tagErr != nil {
			logChan <- fmt.Sprintf("[WARN] 恢复旧镜像标签 %s 失败: %s", imageName, tagErr.Error())
		} else {
			logChan <- fmt.Sprintf("[INFO] 已成功将镜像标签 %s 重新绑定回原旧镜像", imageName)
		}
	}

	// 2. 清理故障新镜像
	if newImageID != "" && newImageID != oldImageID {
		logChan <- "[INFO] 正在清理拉取到的故障新镜像..."
		_, _ = cli.ImageRemove(ctx, newImageID, types.ImageRemoveOptions{Force: false, PruneChildren: true})
	}

	// 3. 将 inspect.Config.Image 恢复为旧镜像 ID
	inspect.Config.Image = oldImageID

	// 4. 重新创建旧容器
	recoveredContainer, err := cli.ContainerCreate(
		ctx,
		inspect.Config,
		inspect.HostConfig,
		networkingConfig,
		nil,
		name,
	)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 回滚创建旧容器失败: %s, 需人工介入确认!", err.Error())
		return
	}

	// 5. 启动恢复的旧容器
	err = cli.ContainerStart(ctx, recoveredContainer.ID, types.ContainerStartOptions{})
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 回滚启动旧容器失败: %s, 需人工介入确认!", err.Error())
	} else {
		logChan <- "[INFO] 回滚恢复成功，原容器已重新重建并上线运行"
	}

	// 6. 清理临时保护 Tag
	if tempBakTag != "" {
		_, _ = cli.ImageRemove(ctx, tempBakTag, types.ImageRemoveOptions{PruneChildren: false})
	}
}

// parseDockerHubImage 解析 Docker Hub 镜像
func parseDockerHubImage(image string) (bool, string) {
	registry, repo, tag := parseImage(image)
	if registry == "registry-1.docker.io" {
		fullName := repo
		if tag != "" && tag != "latest" {
			fullName = repo + ":" + tag
		}
		return true, fullName
	}
	return false, ""
}



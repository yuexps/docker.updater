package dockerclient

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"docker-updater/utils"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

// UpdateOptions 容器更新参数选项。
type UpdateOptions struct {
	BackupEnabled bool
	BackupHours   int
	RestartStack  bool
	TempMirrors   []string
}

var (
	stackLock    sync.Mutex
	stackTimers  = make(map[string]*time.Timer)
	stackUpdated = make(map[string]map[string]bool)
)

// ApplyUpdate 拉取镜像并重建容器。
func ApplyUpdate(ctx context.Context, name string, targetImage string, opts UpdateOptions, logChan chan<- string) (string, error) {
	logChan <- fmt.Sprintf("[INFO] 开始修改容器版本: %s", name)

	cli, err := NewLocalClient()
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 无法连接 Docker 引擎: %s", err.Error())
		return "", err
	}
	defer cli.Close()

	inspect, err := cli.ContainerInspect(ctx, name)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 找不到容器 %s: %s", name, err.Error())
		return "", err
	}
	oldID := inspect.ID
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

	composeFile := ""
	project := inspect.Config.Labels["com.docker.compose.project"]
	if project != "" {
		configFiles := inspect.Config.Labels["com.docker.compose.config-files"]
		if configFiles != "" {
			files := strings.Split(configFiles, ",")
			if len(files) > 0 {
				path := strings.TrimSpace(files[0])
				if _, err := os.Stat(path); err == nil {
					composeFile = path
				}
			}
		}

		if composeFile == "" {
			workingDir := inspect.Config.Labels["com.docker.compose.project.working_dir"]
			if workingDir != "" {
				ymlPath := filepath.Join(workingDir, "docker-compose.yml")
				yamlPath := filepath.Join(workingDir, "compose.yml")
				if _, err := os.Stat(ymlPath); err == nil {
					composeFile = ymlPath
				} else if _, err := os.Stat(yamlPath); err == nil {
					composeFile = yamlPath
				}
			}
		}
	}

	if composeFile != "" {
		if targetImage != "" {
			logChan <- fmt.Sprintf("[WARN] 检测到 Compose 项目，但由于指定了目标镜像 %s，将跳过 Compose 联动，降级为常规单容器版本修改...", targetImage)
		} else {
			logChan <- fmt.Sprintf("[INFO] 自动探测到 Compose 项目: %s", project)
			logChan <- fmt.Sprintf("[INFO] 配置文件路径: %s", composeFile)
			logChan <- "[INFO] 启动 Docker Compose 命令行联动升级..."

			if tempReader, tempErr := pullImageWithMirrors(ctx, cli, imageName, opts.TempMirrors, logChan); tempErr == nil {
				_ = tempReader.Close()
			}

			logChan <- "[INFO] 正在拉取最新的 Compose 镜像..."
			pullErr := runComposeCommand(ctx, logChan, composeFile, "pull")
			if pullErr != nil {
				logChan <- fmt.Sprintf("[WARN] docker compose pull 失败: %s，尝试直接执行 up 部署...", pullErr.Error())
			}

			logChan <- "[INFO] 正在重建并部署 Compose 服务..."
			upErr := runComposeCommand(ctx, logChan, composeFile, "up", "-d", "--remove-orphans")

			composeSuccess := false
			if checkInspect, checkErr := cli.ContainerInspect(ctx, name); checkErr == nil {
				if latestImg, _, imgErr := cli.ImageInspectWithRaw(ctx, imageName); imgErr == nil {
					if checkInspect.State.Running && checkInspect.Image == latestImg.ID {
						composeSuccess = true
					}
				}
			}

			if upErr == nil || composeSuccess {
				if upErr != nil {
					logChan <- "[INFO] Docker Compose 命令行已退出并附带警告或异常，但检测到目标服务已使用新镜像成功运行。"
				}
				logChan <- "[INFO] Compose 联动部署成功！"
				logChan <- "[INFO] 容器版本修改任务全部完成"
				return "", nil
			}

			logChan <- fmt.Sprintf("[WARN] docker compose up 部署失败: %s. 降级为常规单容器克隆逻辑升级...", upErr.Error())
		}
	}

	logChan <- "[INFO] 正在拉取最新镜像..."
	reader, err := pullImageWithMirrors(ctx, cli, imageName, opts.TempMirrors, logChan)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 镜像拉取失败: %s", err.Error())
		return "", err
	}
	defer reader.Close()

	dec := json.NewDecoder(reader)
	for {
		var pullStatus map[string]interface{}
		if err := dec.Decode(&pullStatus); err != nil {
			if err == io.EOF {
				break
			}
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

	logChan <- "[INFO] 正在停止旧容器..."
	stopTimeout := 30
	err = cli.ContainerStop(ctx, oldID, container.StopOptions{Timeout: &stopTimeout})
	if err != nil {
		logChan <- fmt.Sprintf("[WARN] 停止容器失败: %s", err.Error())
	}

	oldNameBackup := name + "_backup_docker_updater"
	_ = cli.ContainerRemove(ctx, oldNameBackup, types.ContainerRemoveOptions{Force: true})

	logChan <- fmt.Sprintf("[INFO] 将旧容器改名为备份: %s", oldNameBackup)
	err = cli.ContainerRename(ctx, oldID, oldNameBackup)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 备份重命名失败: %s", err.Error())
		return "", err
	}

	_, _ = cli.ContainerUpdate(ctx, oldID, container.UpdateConfig{
		RestartPolicy: container.RestartPolicy{Name: "no"},
	})

	logChan <- "[INFO] 正在使用新镜像重建容器..."

	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: make(map[string]*network.EndpointSettings),
	}
	var primaryNet string
	for netName, netConfig := range inspect.NetworkSettings.Networks {
		primaryNet = netName
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
		break
	}

	inspect.Config.Image = imageName
	newContainer, err := cli.ContainerCreate(
		ctx,
		inspect.Config,
		inspect.HostConfig,
		networkingConfig,
		nil,
		name,
	)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 创建容器失败: %s, 正在触发回滚...", err.Error())
		_rollback(ctx, cli, oldID, name, inspect.HostConfig.RestartPolicy, logChan)
		return "", err
	}

	if len(inspect.NetworkSettings.Networks) > 1 {
		for netName, netConfig := range inspect.NetworkSettings.Networks {
			if netName == primaryNet {
				continue
			}
			var ipamConfig *network.EndpointIPAMConfig
			if netConfig.IPAMConfig != nil {
				ipamConfig = &network.EndpointIPAMConfig{
					IPv4Address: netConfig.IPAMConfig.IPv4Address,
					IPv6Address: netConfig.IPAMConfig.IPv6Address,
				}
			}
			err = cli.NetworkConnect(ctx, netName, newContainer.ID, &network.EndpointSettings{
				IPAMConfig: ipamConfig,
				Aliases:    netConfig.Aliases,
			})
			if err != nil {
				logChan <- fmt.Sprintf("[WARN] 连接附加网络 %s 失败: %s", netName, err.Error())
			}
		}
	}

	logChan <- "[INFO] 启动新容器..."
	err = cli.ContainerStart(ctx, newContainer.ID, types.ContainerStartOptions{})
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 启动容器失败: %s, 正在触发回滚...", err.Error())
		_ = cli.ContainerRemove(ctx, newContainer.ID, types.ContainerRemoveOptions{Force: true})
		_rollback(ctx, cli, oldID, name, inspect.HostConfig.RestartPolicy, logChan)
		return "", err
	}

	logChan <- "[INFO] 正在观察新容器运行稳定性 (3s)..."
	stayUp := true
	var checkErr error
	for i := 1; i <= 3; i++ {
		time.Sleep(1 * time.Second)
		newInspect, err := cli.ContainerInspect(ctx, name)
		if err != nil {
			checkErr = err
			stayUp = false
			break
		}
		if !newInspect.State.Running {
			checkErr = fmt.Errorf("容器未持续运行或已退出")
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
		_rollback(ctx, cli, oldID, name, inspect.HostConfig.RestartPolicy, logChan)
		return "", fmt.Errorf("container failed to stay up")
	}

	logChan <- "[INFO] 容器版本修改成功并运行中"

	var policyStr string
	if opts.BackupEnabled {
		policyJSON, _ := json.Marshal(inspect.HostConfig.RestartPolicy)
		policyStr = string(policyJSON)
		logChan <- fmt.Sprintf("[INFO] 备份 %s 保留 %d 小时", oldNameBackup, opts.BackupHours)
	} else {
		logChan <- "[INFO] 正在清理临时备份容器..."
		_ = cli.ContainerRemove(ctx, oldID, types.ContainerRemoveOptions{Force: true})
	}

	project = inspect.Config.Labels["com.docker.compose.project"]
	if opts.RestartStack && project != "" {
		scheduleStackRestart(project, name, logChan)
	}

	logChan <- "[INFO] 容器版本修改任务全部完成"
	return policyStr, nil
}

// _rollback 执行旧容器回滚还原。
func _rollback(ctx context.Context, cli *client.Client, oldID, name string, policy container.RestartPolicy, logChan chan<- string) {
	logChan <- "[INFO] 正在执行回滚恢复旧容器..."
	if err := cli.ContainerRename(ctx, oldID, name); err != nil {
		logChan <- fmt.Sprintf("[ERROR] 恢复旧容器重命名失败: %s", err.Error())
	}
	if _, err := cli.ContainerUpdate(ctx, oldID, container.UpdateConfig{RestartPolicy: policy}); err != nil {
		logChan <- fmt.Sprintf("[WARN] 恢复旧容器重启策略失败: %s", err.Error())
	}
	err := cli.ContainerStart(ctx, oldID, types.ContainerStartOptions{})
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 恢复旧容器启动失败: %s, 需人工介入确认!", err.Error())
	} else {
		logChan <- "[INFO] 回滚恢复成功，原容器已上线运行"
	}
}

// ApplyRollback 一键回滚到备份版本。
func ApplyRollback(ctx context.Context, name string, originalPolicy container.RestartPolicy, logChan chan<- string) error {
	logChan <- fmt.Sprintf("[INFO] 开始回滚容器: %s", name)

	cli, err := NewLocalClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	oldName := name + "_backup_docker_updater"
	backup, err := cli.ContainerInspect(ctx, oldName)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 找不到对应的备份容器 %s: %s", oldName, err.Error())
		return err
	}

	logChan <- "[INFO] 正在停止并删除当前新容器..."
	stopTimeout := 30
	if err := cli.ContainerStop(ctx, name, container.StopOptions{Timeout: &stopTimeout}); err != nil {
		logChan <- fmt.Sprintf("[WARN] 停止当前容器 %s 失败 (可能容器已停止): %s", name, err.Error())
	}
	if err := cli.ContainerRemove(ctx, name, types.ContainerRemoveOptions{Force: true}); err != nil {
		logChan <- fmt.Sprintf("[WARN] 删除当前容器 %s 失败: %s", name, err.Error())
	}

	logChan <- fmt.Sprintf("[INFO] 正在恢复备份容器名: %s -> %s", oldName, name)
	err = cli.ContainerRename(ctx, backup.ID, name)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 备份重命名还原失败: %s", err.Error())
		return err
	}

	if _, err := cli.ContainerUpdate(ctx, backup.ID, container.UpdateConfig{RestartPolicy: originalPolicy}); err != nil {
		logChan <- fmt.Sprintf("[WARN] 恢复备份容器重启策略失败: %s", err.Error())
	}

	logChan <- "[INFO] 启动备份还原容器..."
	err = cli.ContainerStart(ctx, backup.ID, types.ContainerStartOptions{})
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 启动还原容器失败: %s", err.Error())
		return err
	}

	logChan <- "[INFO] 容器回滚成功"
	logChan <- "[INFO] 容器回滚任务全部完成"
	return nil
}

// scheduleStackRestart 执行 Compose 容器组延迟重启。
func scheduleStackRestart(project string, excludedContainer string, logChan chan<- string) {
	stackLock.Lock()
	defer stackLock.Unlock()

	if stackUpdated[project] == nil {
		stackUpdated[project] = make(map[string]bool)
	}
	stackUpdated[project][excludedContainer] = true

	if oldTimer, ok := stackTimers[project]; ok {
		oldTimer.Stop()
	}

	logChan <- fmt.Sprintf("[INFO] Compose 栈 %s: 兄弟成员将在 8 秒内进行刷新重启...", project)

	timer := time.AfterFunc(8*time.Second, func() {
		stackLock.Lock()
		updated := stackUpdated[project]
		delete(stackUpdated, project)
		delete(stackTimers, project)
		stackLock.Unlock()

		asyncCtx, asyncCancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer asyncCancel()

		asyncCli, err := NewLocalClient()
		if err != nil {
			utils.LogError("重启 Compose 栈时无法连接 Docker 引擎: %s", err.Error())
			return
		}
		defer asyncCli.Close()

		containers, err := asyncCli.ContainerList(asyncCtx, types.ContainerListOptions{})
		if err != nil {
			return
		}

		for _, c := range containers {
			name := strings.TrimPrefix(c.Names[0], "/")
			if name == "" || strings.HasSuffix(name, "_backup_docker_updater") || updated[name] {
				continue
			}
			if c.Labels["com.docker.compose.project"] == project {
				utils.LogInfo("重启 Compose 栈同僚: %s", name)
				stopTimeout := 30
				if err := asyncCli.ContainerRestart(asyncCtx, c.ID, container.StopOptions{Timeout: &stopTimeout}); err != nil {
					utils.LogWarning("重启 Compose 栈同僚 %s 失败: %s", name, err.Error())
				}
			}
		}
	})
	stackTimers[project] = timer
}

// CleanExpiredBackups 物理清除备份容器。
func CleanExpiredBackups(ctx context.Context, backupNames []string) {
	cli, err := NewLocalClient()
	if err != nil {
		utils.LogError("定时自动清理物理备份容器失败 (无法连接 Docker 引擎): %s", err.Error())
		return
	}
	defer cli.Close()

	for _, backupName := range backupNames {
		err := cli.ContainerRemove(ctx, backupName, types.ContainerRemoveOptions{Force: true})
		if err != nil {
			utils.LogError("定时自动清理已过期备份容器 %s 失败: %s", backupName, err.Error())
		} else {
			utils.LogInfo("定时自动清理已过期备份容器 %s 成功", backupName)
		}
	}
}

// runComposeCommand 调用外部 docker compose 命令。
func runComposeCommand(ctx context.Context, logChan chan<- string, composeFile string, args ...string) error {
	cmdArgs := []string{"compose", "-f", composeFile}
	cmdArgs = append(cmdArgs, args...)
	cmd := exec.CommandContext(ctx, "docker", cmdArgs...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	if err := cmd.Start(); err != nil {
		return err
	}

	var wg sync.WaitGroup
	wg.Add(2)

	readerFunc := func(r io.Reader, prefix string) {
		defer wg.Done()
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			text := scanner.Text()
			logChan <- fmt.Sprintf("%s %s", prefix, text)
		}
		if err := scanner.Err(); err != nil {
			logChan <- fmt.Sprintf("%s [ERROR] 读取流数据异常: %s", prefix, err.Error())
		}
	}

	go readerFunc(stdout, "[INFO]")
	go readerFunc(stderr, "[INFO]")

	wg.Wait()
	return cmd.Wait()
}

// parseDockerHubImage 解析 Docker Hub 镜像。
func parseDockerHubImage(image string) (bool, string) {
	cleanImage := image
	cleanImage = strings.TrimPrefix(cleanImage, "docker.io/")
	cleanImage = strings.TrimPrefix(cleanImage, "registry-1.docker.io/")

	parts := strings.Split(cleanImage, "/")
	if len(parts) == 0 {
		return false, ""
	}

	hasDomain := false
	if len(parts) > 1 {
		firstPart := parts[0]
		if strings.Contains(firstPart, ".") || strings.Contains(firstPart, ":") || firstPart == "localhost" {
			hasDomain = true
		}
	}

	if !hasDomain {
		fullName := cleanImage
		if len(parts) == 1 {
			fullName = "library/" + cleanImage
		}
		return true, fullName
	}

	return false, ""
}

// pullImageWithMirrors 拉取镜像。
func pullImageWithMirrors(ctx context.Context, cli *client.Client, imageName string, tempMirrors []string, logChan chan<- string) (io.ReadCloser, error) {
	isOfficial, fullName := parseDockerHubImage(imageName)

	if isOfficial {
		// 融合应用级配置的镜像加速器并进行去重
		seen := make(map[string]bool)
		var mirrors []string
		for _, m := range tempMirrors {
			m = strings.TrimSpace(m)
			if m == "" {
				continue
			}
			if !seen[m] {
				seen[m] = true
				mirrors = append(mirrors, m)
			}
		}

		if len(mirrors) > 0 {
			for _, mirror := range mirrors {
				mirror = strings.TrimSpace(mirror)
				if mirror == "" {
					continue
				}

				mirrorHost := mirror
				mirrorHost = strings.TrimPrefix(mirrorHost, "https://")
				mirrorHost = strings.TrimPrefix(mirrorHost, "http://")
				mirrorHost = strings.TrimSuffix(mirrorHost, "/")

				tempImageName := fmt.Sprintf("%s/%s", mirrorHost, fullName)
				logChan <- fmt.Sprintf("[INFO] 检测到官方镜像，尝试通过镜像源 %s 加速拉取...", mirrorHost)

				reader, err := cli.ImagePull(ctx, tempImageName, types.ImagePullOptions{})
				if err != nil {
					logChan <- fmt.Sprintf("[WARN] 通过加速源 %s 拉取失败: %s. 尝试下一个...", mirrorHost, err.Error())
					continue
				}

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
				_ = reader.Close()

				logChan <- fmt.Sprintf("[INFO] 临时加速拉取成功，正在为镜像打标回官方原名: %s", imageName)
				if err := cli.ImageTag(ctx, tempImageName, imageName); err != nil {
					logChan <- fmt.Sprintf("[WARN] 打标回原名失败: %s", err.Error())
					_, _ = cli.ImageRemove(ctx, tempImageName, types.ImageRemoveOptions{PruneChildren: true})
					continue
				}

				logChan <- fmt.Sprintf("[INFO] 正在清理临时镜像源冗余标签: %s", tempImageName)
				_, _ = cli.ImageRemove(ctx, tempImageName, types.ImageRemoveOptions{PruneChildren: true})

				return io.NopCloser(strings.NewReader(`{"status":"Success"}`)), nil
			}
			logChan <- "[WARN] 所有配置的镜像源均已尝试拉取失败，将降级为官方直接拉取..."
		}
	}

	return cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
}

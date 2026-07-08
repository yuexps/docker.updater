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

	"docker-updater/db"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
)

var (
	stackLock    sync.Mutex
	stackTimers  = make(map[string]*time.Timer)
	stackUpdated = make(map[string]map[string]bool)
)

// ApplyUpdate 拉取新镜像并重新创建本地容器，如果闪退则秒级回滚
func ApplyUpdate(ctx context.Context, name string, logChan chan<- string) error {
	logChan <- fmt.Sprintf("[INFO] 开始升级容器: %s", name)

	cli, err := NewLocalClient()
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 无法连接 Docker 引擎: %s", err.Error())
		return err
	}
	defer cli.Close()

	// 1. 获取原容器详情
	inspect, err := cli.ContainerInspect(ctx, name)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 找不到容器 %s: %s", name, err.Error())
		return err
	}
	oldID := inspect.ID
	imageName := inspect.Config.Image

	logChan <- fmt.Sprintf("[INFO] 容器名: %s", name)
	logChan <- fmt.Sprintf("[INFO] 镜像源: %s", imageName)

	// 自动反向探测 Docker Compose 部署配置文件
	composeFile := ""
	project := inspect.Config.Labels["com.docker.compose.project"]
	if project != "" {
		// 1. 尝试从 com.docker.compose.config-files 探测
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

		// 2. 如果没找到，尝试从 working_dir 探测
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
		logChan <- fmt.Sprintf("[INFO] 自动探测到 Compose 项目: %s", project)
		logChan <- fmt.Sprintf("[INFO] 配置文件路径: %s", composeFile)
		logChan <- "[INFO] 启动 Docker Compose 命令行联动升级..."

		// 提前通过加速源拉取并打标当前镜像，防止 Compose 命令行拉取时卡网
		if tempReader, tempErr := pullImageWithMirrors(ctx, cli, imageName, logChan); tempErr == nil {
			_ = tempReader.Close()
		}

		logChan <- "[INFO] 正在拉取最新的 Compose 镜像..."
		pullErr := runComposeCommand(ctx, logChan, composeFile, "pull")
		if pullErr != nil {
			logChan <- fmt.Sprintf("[WARNING] docker compose pull 失败: %s，尝试直接执行 up 部署...", pullErr.Error())
		}

		logChan <- "[INFO] 正在重建并部署 Compose 服务..."
		upErr := runComposeCommand(ctx, logChan, composeFile, "up", "-d", "--remove-orphans")
		if upErr == nil {
			logChan <- "[SUCCESS] Compose 联动部署成功！"
			db.DB.Delete(&db.AvailableUpdate{ContainerName: name})
			history := db.UpdateHistory{
				ContainerName: name,
				Image:         imageName,
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
				Status:        "success",
			}
			db.DB.Create(&history)
			logChan <- "[SUCCESS] 容器升级任务全部完成"
			return nil
		}

		logChan <- fmt.Sprintf("[WARNING] docker compose up 部署失败: %s. 降级为常规单容器克隆逻辑升级...", upErr.Error())
	}

	// 2. 拉取镜像
	logChan <- "[INFO] 正在拉取最新镜像..."
	reader, err := pullImageWithMirrors(ctx, cli, imageName, logChan)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 镜像拉取失败: %s", err.Error())
		return err
	}
	defer reader.Close()

	// 过滤拉取进度输出 (不带 emoji)
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
				logChan <- fmt.Sprintf("[PULL] %s %s", statusMsg, progressDetail)
			} else {
				logChan <- fmt.Sprintf("[PULL] %s", statusMsg)
			}
		}
	}

	logChan <- "[INFO] 正在停止旧容器..."
	stopTimeout := 30
	err = cli.ContainerStop(ctx, oldID, container.StopOptions{Timeout: &stopTimeout})
	if err != nil {
		logChan <- fmt.Sprintf("[WARNING] 停止容器失败: %s", err.Error())
	}

	// 3. 重命名为备份容器并更改重启策略
	oldNameBackup := name + "_old"
	_ = cli.ContainerRemove(ctx, oldNameBackup, types.ContainerRemoveOptions{Force: true}) // 清理可能的残留备份

	logChan <- fmt.Sprintf("[INFO] 将旧容器改名为备份: %s", oldNameBackup)
	err = cli.ContainerRename(ctx, oldID, oldNameBackup)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 备份重命名失败: %s", err.Error())
		return err
	}

	// 备份容器的重启策略强制更改为 no，防止物理机关机重启时两个同名宿主容器冲突启动
	_, _ = cli.ContainerUpdate(ctx, oldID, container.UpdateConfig{
		RestartPolicy: container.RestartPolicy{Name: "no"},
	})

	// 4. 重建并加载原始配置
	logChan <- "[INFO] 正在使用新镜像重建容器..."

	// 重建网络结构
	networkingConfig := &network.NetworkingConfig{
		EndpointsConfig: make(map[string]*network.EndpointSettings),
	}
	var primaryNet string
	for netName, netConfig := range inspect.NetworkSettings.Networks {
		primaryNet = netName
		// 复制网络参数，保留原有静态 IP 别名
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
		break // 优先附加第一个网络
	}

	// 创建新容器
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
		return err
	}

	// 绑定其他网络
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
				logChan <- fmt.Sprintf("[WARNING] 连接附加网络 %s 失败: %s", netName, err.Error())
			}
		}
	}

	// 5. 启动并校验状态 (自愈回滚逻辑)
	logChan <- "[INFO] 启动新容器..."
	err = cli.ContainerStart(ctx, newContainer.ID, types.ContainerStartOptions{})
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 启动容器失败: %s, 正在触发回滚...", err.Error())
		_rollback(ctx, cli, oldID, name, inspect.HostConfig.RestartPolicy, logChan)
		return err
	}

	// 等待 2 秒校验新容器是否闪退
	time.Sleep(2 * time.Second)
	newInspect, err := cli.ContainerInspect(ctx, name)
	if err != nil || !newInspect.State.Running {
		reason := "容器未持续运行或已退出"
		if err != nil {
			reason = err.Error()
		}
		logChan <- fmt.Sprintf("[ERROR] 新容器健康检查失败: %s, 正在自动回滚...", reason)
		_ = cli.ContainerRemove(ctx, newContainer.ID, types.ContainerRemoveOptions{Force: true})
		_rollback(ctx, cli, oldID, name, inspect.HostConfig.RestartPolicy, logChan)
		return fmt.Errorf("container failed to stay up")
	}

	// 6. 更新成功处理
	logChan <- "[SUCCESS] 容器更新成功并运行中"
	db.DB.Delete(&db.AvailableUpdate{ContainerName: name})

	// 记录成功历史
	history := db.UpdateHistory{
		ContainerName: name,
		Image:         imageName,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Status:        "success",
	}
	db.DB.Create(&history)

	// 处理备份保留期限
	backupEnabled := db.GetSetting("backup_enabled", "false") == "true"
	if backupEnabled {
		backupHours := 24
		fmt.Sscanf(db.GetSetting("backup_hours", "24"), "%d", &backupHours)
		expiresAt := time.Now().Add(time.Duration(backupHours) * time.Hour).UTC().Format(time.RFC3339)

		policyJSON, _ := json.Marshal(inspect.HostConfig.RestartPolicy)
		rollbackMeta := db.RollbackMetadata{
			ContainerName: name,
			BackedUpAt:    time.Now().UTC().Format(time.RFC3339),
			ExpiresAt:     expiresAt,
			RestartPolicy: string(policyJSON),
		}
		db.DB.Save(&rollbackMeta)
		logChan <- fmt.Sprintf("[INFO] 备份 %s 保留 %d 小时", oldNameBackup, backupHours)
	} else {
		logChan <- "[INFO] 正在清理临时备份容器..."
		_ = cli.ContainerRemove(ctx, oldID, types.ContainerRemoveOptions{Force: true})
	}

	// 7. 同步重启同一个 Compose 栈的其他容器 (防抖逻辑)
	restartStack := db.GetSetting("restart_stack", "false") == "true"
	project = inspect.Config.Labels["com.docker.compose.project"]
	if restartStack && project != "" {
		scheduleStackRestart(ctx, cli, project, name, logChan)
	}

	logChan <- "[SUCCESS] 容器升级任务全部完成"
	return nil
}

// _rollback 回滚还原容器
func _rollback(ctx context.Context, cli *client.Client, oldID, name string, policy container.RestartPolicy, logChan chan<- string) {
	logChan <- "[INFO] 正在执行回滚恢复旧容器..."
	_ = cli.ContainerRename(ctx, oldID, name)
	_, _ = cli.ContainerUpdate(ctx, oldID, container.UpdateConfig{RestartPolicy: policy})
	err := cli.ContainerStart(ctx, name, types.ContainerStartOptions{})
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 恢复旧容器启动失败: %s, 需人工介入确认!", err.Error())
	} else {
		logChan <- "[SUCCESS] 回滚恢复成功，原容器已上线运行"
	}
}

// ApplyRollback 一键回滚到备份版本
func ApplyRollback(ctx context.Context, name string, logChan chan<- string) error {
	logChan <- fmt.Sprintf("[INFO] 开始回滚容器: %s", name)

	cli, err := NewLocalClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	oldName := name + "_old"
	backup, err := cli.ContainerInspect(ctx, oldName)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 找不到对应的备份容器 %s: %s", oldName, err.Error())
		return err
	}

	logChan <- "[INFO] 正在停止并删除当前新容器..."
	stopTimeout := 30
	_ = cli.ContainerStop(ctx, name, container.StopOptions{Timeout: &stopTimeout})
	_ = cli.ContainerRemove(ctx, name, types.ContainerRemoveOptions{Force: true})

	logChan <- fmt.Sprintf("[INFO] 正在恢复备份容器名: %s -> %s", oldName, name)
	err = cli.ContainerRename(ctx, backup.ID, name)
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 备份重命名还原失败: %s", err.Error())
		return err
	}

	// 恢复原有的重启策略
	var policy container.RestartPolicy
	var meta db.RollbackMetadata
	if err := db.DB.First(&meta, "container_name = ?", name).Error; err == nil {
		_ = json.Unmarshal([]byte(meta.RestartPolicy), &policy)
	} else {
		policy = container.RestartPolicy{Name: "unless-stopped"}
	}

	_, _ = cli.ContainerUpdate(ctx, backup.ID, container.UpdateConfig{RestartPolicy: policy})

	logChan <- "[INFO] 启动备份还原容器..."
	err = cli.ContainerStart(ctx, backup.ID, types.ContainerStartOptions{})
	if err != nil {
		logChan <- fmt.Sprintf("[ERROR] 启动还原容器失败: %s", err.Error())
		return err
	}

	logChan <- "[SUCCESS] 容器回滚成功"
	db.DB.Delete(&db.RollbackMetadata{ContainerName: name})

	// 写入历史
	history := db.UpdateHistory{
		ContainerName: name,
		Image:         "rollback",
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
		Status:        "success",
	}
	db.DB.Create(&history)

	logChan <- "[SUCCESS] 容器回滚任务全部完成"
	return nil
}

// scheduleStackRestart 安排延迟防抖重启同一 Compose 栈的兄弟容器
func scheduleStackRestart(ctx context.Context, cli *client.Client, project string, excludedContainer string, logChan chan<- string) {
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

		containers, err := cli.ContainerList(ctx, types.ContainerListOptions{})
		if err != nil {
			return
		}

		for _, c := range containers {
			name := strings.TrimPrefix(c.Names[0], "/")
			if name == "" || strings.HasSuffix(name, "_old") || updated[name] {
				continue
			}
			if c.Labels["com.docker.compose.project"] == project {
				fmt.Printf("[INFO] 重启 Compose 栈同僚: %s\n", name)
				stopTimeout := 30
				_ = cli.ContainerRestart(ctx, c.ID, container.StopOptions{Timeout: &stopTimeout})
			}
		}
	})
	stackTimers[project] = timer
}

// CleanExpiredBackups 定时物理清除过期的 _old 备份容器
func CleanExpiredBackups(ctx context.Context) {
	cli, err := NewLocalClient()
	if err != nil {
		return
	}
	defer cli.Close()

	var expired []db.RollbackMetadata
	nowStr := time.Now().UTC().Format(time.RFC3339)
	if err := db.DB.Find(&expired, "expires_at <= ?", nowStr).Error; err != nil || len(expired) == 0 {
		return
	}

	for _, meta := range expired {
		backupName := meta.ContainerName + "_old"
		fmt.Printf("[INFO] 正在清理已过期备份: %s\n", backupName)
		_ = cli.ContainerRemove(ctx, backupName, types.ContainerRemoveOptions{Force: true})
		db.DB.Delete(&meta)
	}
}

// runComposeCommand 通过外部命令行在宿主机上运行 docker compose 指令并抓取日志流
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
	go readerFunc(stderr, "[PULL]")

	wg.Wait()
	return cmd.Wait()
}

// parseDockerHubImage 识别官方 Docker Hub 镜像，并返回它在加速源里拼接时所需的完整子路径（如 library/nginx:latest）
func parseDockerHubImage(image string) (bool, string) {
	cleanImage := image
	cleanImage = strings.TrimPrefix(cleanImage, "docker.io/")
	cleanImage = strings.TrimPrefix(cleanImage, "registry-1.docker.io/")

	parts := strings.Split(cleanImage, "/")
	if len(parts) == 0 {
		return false, ""
	}

	// 检查第一部分是否为域名
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

// pullImageWithMirrors 使用“临时加速源”拉取官方镜像，并重新 Tag 转换回官方原标签名
func pullImageWithMirrors(ctx context.Context, cli *client.Client, imageName string, logChan chan<- string) (io.ReadCloser, error) {
	isOfficial, fullName := parseDockerHubImage(imageName)

	if isOfficial {
		tempMirrorsStr := db.GetSetting("temp_mirrors", "[]")
		var tempMirrors []string
		_ = json.Unmarshal([]byte(tempMirrorsStr), &tempMirrors)

		if len(tempMirrors) > 0 {
			for _, mirror := range tempMirrors {
				mirror = strings.TrimSpace(mirror)
				if mirror == "" {
					continue
				}

				mirrorHost := mirror
				mirrorHost = strings.TrimPrefix(mirrorHost, "https://")
				mirrorHost = strings.TrimPrefix(mirrorHost, "http://")
				mirrorHost = strings.TrimSuffix(mirrorHost, "/")

				tempImageName := fmt.Sprintf("%s/%s", mirrorHost, fullName)
				logChan <- fmt.Sprintf("[INFO] 检测到官方镜像，尝试通过临时镜像源 %s 加速拉取...", mirrorHost)

				reader, err := cli.ImagePull(ctx, tempImageName, types.ImagePullOptions{})
				if err != nil {
					logChan <- fmt.Sprintf("[WARNING] 通过加速源 %s 拉取失败: %s. 尝试下一个...", mirrorHost, err.Error())
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
							logChan <- fmt.Sprintf("[PULL] %s %s", statusMsg, progressDetail)
						} else {
							logChan <- fmt.Sprintf("[PULL] %s", statusMsg)
						}
					}
				}
				_ = reader.Close()

				logChan <- fmt.Sprintf("[INFO] 临时加速拉取成功，正在为镜像打标回官方原名: %s", imageName)
				if err := cli.ImageTag(ctx, tempImageName, imageName); err != nil {
					logChan <- fmt.Sprintf("[WARNING] 打标回原名失败: %s", err.Error())
					continue
				}

				logChan <- fmt.Sprintf("[INFO] 正在清理临时镜像源冗余标签: %s", tempImageName)
				_, _ = cli.ImageRemove(ctx, tempImageName, types.ImageRemoveOptions{PruneChildren: true})

				return io.NopCloser(strings.NewReader(`{"status":"Success"}`)), nil
			}
			logChan <- "[WARNING] 所有配置的临时镜像源均已尝试拉取失败，将降级为官方直接拉取..."
		}
	}

	return cli.ImagePull(ctx, imageName, types.ImagePullOptions{})
}

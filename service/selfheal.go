package service

import (
	"context"
	"encoding/json"
	"strings"

	"docker-updater/db"
	"docker-updater/dockerclient"
	"docker-updater/utils"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
)

// SelfHealInterruptedOperations 执行启动时容器状态恢复。
func SelfHealInterruptedOperations() {
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		utils.LogWarning("自愈模块无法连接 Docker 引擎: %s", err.Error())
		return
	}
	defer cli.Close()

	ctx := context.Background()
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		utils.LogWarning("自愈模块获取本地容器列表失败: %s", err.Error())
		return
	}

	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if name == "" || !strings.HasSuffix(name, "_old") {
			continue
		}

		baseName := strings.TrimSuffix(name, "_old")
		utils.LogInfo("发现未完成备份记录: %s", name)

		primaryRunning := false
		for _, o := range containers {
			oName := strings.TrimPrefix(o.Names[0], "/")
			if oName == baseName && o.State == "running" {
				primaryRunning = true
				break
			}
		}

		if primaryRunning {
			var meta db.RollbackMetadata
			if err := db.DB.First(&meta, "container_name = ?", baseName).Error; err != nil {
				utils.LogInfo("自动清除多余孤儿备份容器: %s", name)
				if removeErr := cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true}); removeErr != nil {
					utils.LogWarning("清除残留孤儿备份容器 %s 失败: %s", name, removeErr.Error())
				}
			}
			continue
		}

		utils.LogInfo("主容器缺失或非运行中，启动自愈回退恢复: %s", baseName)
		if removeErr := cli.ContainerRemove(ctx, baseName, types.ContainerRemoveOptions{Force: true}); removeErr != nil {
			utils.LogWarning("自愈回退时尝试移除损坏新版主容器 %s 失败: %s", baseName, removeErr.Error())
		}

		err = cli.ContainerRename(ctx, c.ID, baseName)
		if err != nil {
			utils.LogError("自愈回退时重命名备份容器 %s 还原为原容器名 %s 失败: %s", name, baseName, err.Error())
			continue
		}

		var meta db.RollbackMetadata
		var policy container.RestartPolicy
		if err := db.DB.First(&meta, "container_name = ?", baseName).Error; err == nil {
			_ = json.Unmarshal([]byte(meta.RestartPolicy), &policy)
			db.DB.Delete(&meta)
		} else {
			policy = container.RestartPolicy{Name: "unless-stopped"}
		}
		if _, updateErr := cli.ContainerUpdate(ctx, c.ID, container.UpdateConfig{RestartPolicy: policy}); updateErr != nil {
			utils.LogWarning("自愈回退时更新容器 %s 的重启策略失败: %s", baseName, updateErr.Error())
		}
		if startErr := cli.ContainerStart(ctx, baseName, types.ContainerStartOptions{}); startErr != nil {
			utils.LogError("自愈回退时启动已复原容器 %s 失败: %s", baseName, startErr.Error())
		} else {
			utils.LogSuccess("成功将容器 %s 从前次崩溃更新中复原并上线运行", baseName)
		}
	}
}

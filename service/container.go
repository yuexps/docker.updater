package service

import (
	"context"
	"strings"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"

	"github.com/docker/docker/api/types"
)

// GetContainerStatusData 获取容器状态数据。
func GetContainerStatusData(ctx context.Context) (map[string]interface{}, error) {
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	var available []db.AvailableUpdate
	db.DB.Find(&available)
	availMap := make(map[string]db.AvailableUpdate)
	for _, a := range available {
		availMap[a.ContainerName] = a
	}

	var deferred []db.DeferredUpdate
	db.DB.Find(&deferred)
	deferMap := make(map[string]db.DeferredUpdate)
	for _, d := range deferred {
		deferMap[d.ContainerName] = d
	}

	var rollbacks []db.RollbackMetadata
	db.DB.Find(&rollbacks)
	rbMap := make(map[string]db.RollbackMetadata)
	for _, r := range rollbacks {
		rbMap[r.ContainerName] = r
	}

	today := time.Now().Format("2006-01-02")
	var result []map[string]interface{}

	for _, containerItem := range containers {
		name := ""
		if len(containerItem.Names) > 0 {
			name = strings.TrimPrefix(containerItem.Names[0], "/")
		}
		if name == "" || strings.HasSuffix(name, "_old") {
			continue
		}

		inspect, err := cli.ContainerInspect(ctx, containerItem.ID)
		if err != nil {
			continue
		}

		imageName := inspect.Config.Image
		_, _, inspectErr := cli.ImageInspectWithRaw(ctx, inspect.Image)
		if inspectErr != nil {
			continue
		}

		status := "ok"
		var checkedAt string
		var deferUntil *string

		info, hasUpdate := availMap[name]
		if hasUpdate {
			checkedAt = info.CheckedAt
			if d, isDeferred := deferMap[name]; isDeferred && (d.Until == "forever" || d.Until > today) {
				status = "deferred"
				val := d.Until
				deferUntil = &val
			}
			if status != "deferred" {
				status = "update"
			}
		}

		rb, hasRollback := rbMap[name]
		var rollbackExpires *string
		if hasRollback {
			if _, err := cli.ContainerInspect(ctx, name+"_old"); err == nil {
				val := rb.ExpiresAt
				rollbackExpires = &val
			} else {
				hasRollback = false
			}
		}

		localDigest := ""
		remoteDigest := ""
		if hasUpdate {
			localDigest = info.LocalDigest
			remoteDigest = info.RemoteDigest
		}

		result = append(result, map[string]interface{}{
			"name":             name,
			"image":            imageName,
			"status":           status,
			"defer_until":      deferUntil,
			"checked_at":       checkedAt,
			"has_rollback":     hasRollback,
			"rollback_expires": rollbackExpires,
			"compose_project":  containerItem.Labels["com.docker.compose.project"],
			"running":          containerItem.State == "running",
			"local_digest":     localDigest,
			"remote_digest":    remoteDigest,
		})
	}

	var history []db.UpdateHistory
	db.DB.Order("updated_at desc").Limit(100).Find(&history)

	lastCheck := db.GetSetting("last_check_time", "")
	queued, active := GlobalQueue.GetQueueState()
	return map[string]interface{}{
		"containers": result,
		"last_check": lastCheck,
		"history":    history,
		"active":     active,
		"queued":     queued,
	}, nil
}

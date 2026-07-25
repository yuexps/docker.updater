package service

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"

	"github.com/docker/docker/api/types"
)

var isScanning atomic.Bool

// TryStartScan 尝试抢占更新扫描任务状态锁
func TryStartScan() bool {
	return isScanning.CompareAndSwap(false, true)
}

// EndScan 释放扫描状态锁
func EndScan() {
	isScanning.Store(false)
}

// GetContainerStatusData 获取容器状态数据
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

	today := time.Now().Format("2006-01-02")
	var result []map[string]interface{}

	// 1. 筛选目标容器
	type targetContainer struct {
		item types.Container
		name string
	}
	var targets []targetContainer
	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if name == "" {
			continue
		}
		targets = append(targets, targetContainer{item: c, name: name})
	}

	// 2. 并发拉取 Inspect 信息 (限制最大并发度为 8)
	type inspectResult struct {
		name         string
		container    types.Container
		inspectData  types.ContainerJSON
		imageInspect types.ImageInspect
		imageMissing bool
		err          error
	}

	resultCh := make(chan inspectResult, len(targets))
	sem := make(chan struct{}, 8)
	var wg sync.WaitGroup

	for _, t := range targets {
		wg.Add(1)
		go func(tc targetContainer) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			inspect, err := cli.ContainerInspect(ctx, tc.item.ID)
			if err != nil {
				resultCh <- inspectResult{name: tc.name, err: err}
				return
			}

			imgInspect, _, inspectErr := cli.ImageInspectWithRaw(ctx, inspect.Image)
			imageMissing := inspectErr != nil

			resultCh <- inspectResult{
				name:         tc.name,
				container:    tc.item,
				inspectData:  inspect,
				imageInspect: imgInspect,
				imageMissing: imageMissing,
			}
		}(t)
	}

	wg.Wait()
	close(resultCh)

	// 3. 构建结果列表
	for res := range resultCh {
		if res.err != nil {
			continue
		}

		name := res.name
		containerItem := res.container
		inspect := res.inspectData
		imageInspect := res.imageInspect
		imageName := inspect.Config.Image

		status := "ok"
		var checkedAt string
		var deferUntil *string

		if d, isDeferred := deferMap[name]; isDeferred && (d.Until == "forever" || d.Until > today) {
			status = "deferred"
			val := d.Until
			deferUntil = &val
		}

		info, hasInfo := availMap[name]
		if hasInfo {
			checkedAt = info.CheckedAt
			if info.HasUpdate && status != "deferred" {
				status = "update"
			}
		}

		localDigest := ""
		remoteDigest := ""
		localVersion := ""
		remoteVersion := ""
		checkMethod := ""

		if hasInfo {
			if info.HasUpdate && !res.imageMissing {
				localDigest = info.LocalDigest
				if localDigest == "" && inspect.Image != "" {
					localDigest = inspect.Image
				}
				remoteDigest = info.RemoteDigest
				localVersion = info.LocalVersion
				remoteVersion = info.RemoteVersion
				checkMethod = info.CheckMethod
			}
		}

		if checkMethod == "" {
			if len(imageInspect.RepoDigests) > 0 {
				for _, rd := range imageInspect.RepoDigests {
					if parts := strings.SplitN(rd, "@", 2); len(parts) == 2 {
						localDigest = parts[1]
						checkMethod = "RepoDigest"
						break
					}
				}
			}
			if localDigest == "" && inspect.Image != "" {
				localDigest = inspect.Image
				checkMethod = "ConfigDigest"
			}
			localVersion = dockerclient.ExtractSemanticVersion(inspect.Config.Labels, inspect.Config.Env, imageName)
		}

		if checkedAt == "" {
			checkedAt = db.GetSetting("last_check_time", "")
		}

		result = append(result, map[string]interface{}{
			"name":            name,
			"image":           imageName,
			"status":          status,
			"defer_until":     deferUntil,
			"checked_at":      checkedAt,
			"compose_project": containerItem.Labels["com.docker.compose.project"],
			"running":         containerItem.State == "running",
			"local_digest":    localDigest,
			"remote_digest":   remoteDigest,
			"local_version":   localVersion,
			"remote_version":  remoteVersion,
			"image_missing":   res.imageMissing,
			"check_method":    checkMethod,
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

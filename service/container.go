package service

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"
	"docker-updater/utils"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
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

// ComposeContainerItem Compose 项目内关联容器结构
type ComposeContainerItem struct {
	Name          string `json:"name"`
	Service       string `json:"service"`
	Image         string `json:"image"`
	State         string `json:"state"`
	HasUpdate     bool   `json:"has_update"`
	LocalVersion  string `json:"local_version"`
	RemoteVersion string `json:"remote_version"`
}

// ComposeProjectItem Compose 项目聚合数据结构
type ComposeProjectItem struct {
	Name          string                 `json:"name"`
	WorkingDir    string                 `json:"working_dir"`
	ConfigFiles   string                 `json:"config_files"`
	Containers    []ComposeContainerItem `json:"containers"`
	Status        string                 `json:"status"` // running, partial, stopped
	ServicesCount int                    `json:"services_count"`
	HasUpdate     bool                   `json:"has_update"`
}

// GetComposeProjectsData 提取并聚合宿主机 Compose 项目
func GetComposeProjectsData(ctx context.Context) ([]ComposeProjectItem, error) {
	statusData, err := GetContainerStatusData(ctx)
	if err != nil {
		return nil, err
	}

	containers, ok := statusData["containers"].([]map[string]interface{})
	if !ok {
		return []ComposeProjectItem{}, nil
	}

	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	type inspectResult struct {
		cMap        map[string]interface{}
		serviceName string
		workingDir  string
		configFiles string
		state       string
	}

	var composeContainers []map[string]interface{}
	for _, c := range containers {
		if proj, _ := c["compose_project"].(string); proj != "" {
			composeContainers = append(composeContainers, c)
		}
	}

	results := make([]inspectResult, len(composeContainers))
	var wg sync.WaitGroup
	sem := make(chan struct{}, 8)

	for i, c := range composeContainers {
		wg.Add(1)
		go func(idx int, containerData map[string]interface{}) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			name, _ := containerData["name"].(string)

			serviceName := ""
			workingDir := ""
			configFiles := ""
			state := "stopped"

			inspect, err := cli.ContainerInspect(ctx, name)
			if err == nil {
				serviceName = inspect.Config.Labels["com.docker.compose.service"]
				workingDir = inspect.Config.Labels["com.docker.compose.project.working_dir"]
				configFiles = inspect.Config.Labels["com.docker.compose.project.config_files"]
				if inspect.State != nil && inspect.State.Running {
					state = "running"
				}
			} else {
				if running, _ := containerData["running"].(bool); running {
					state = "running"
				}
			}

			results[idx] = inspectResult{
				cMap:        containerData,
				serviceName: serviceName,
				workingDir:  workingDir,
				configFiles: configFiles,
				state:       state,
			}
		}(i, c)
	}

	wg.Wait()

	projectMap := make(map[string]*ComposeProjectItem)
	var projectNames []string

	for _, res := range results {
		c := res.cMap
		name, _ := c["name"].(string)
		composeProj, _ := c["compose_project"].(string)
		status, _ := c["status"].(string)
		hasUp := (status == "update")
		locVer, _ := c["local_version"].(string)
		remVer, _ := c["remote_version"].(string)
		image, _ := c["image"].(string)

		cItem := ComposeContainerItem{
			Name:          name,
			Service:       res.serviceName,
			Image:         image,
			State:         res.state,
			HasUpdate:     hasUp,
			LocalVersion:  locVer,
			RemoteVersion: remVer,
		}

		if proj, exists := projectMap[composeProj]; exists {
			proj.Containers = append(proj.Containers, cItem)
			if hasUp {
				proj.HasUpdate = true
			}
			if res.workingDir != "" && proj.WorkingDir == "" {
				proj.WorkingDir = res.workingDir
			}
			if res.configFiles != "" && proj.ConfigFiles == "" {
				proj.ConfigFiles = res.configFiles
			}
		} else {
			proj := &ComposeProjectItem{
				Name:        composeProj,
				WorkingDir:  res.workingDir,
				ConfigFiles: res.configFiles,
				Containers:  []ComposeContainerItem{cItem},
				HasUpdate:   hasUp,
			}
			projectMap[composeProj] = proj
			projectNames = append(projectNames, composeProj)
		}
	}

	var result []ComposeProjectItem
	for _, pName := range projectNames {
		proj := projectMap[pName]
		proj.ServicesCount = len(proj.Containers)

		// 组内关联容器按名称字典序稳定排序
		sort.Slice(proj.Containers, func(i, j int) bool {
			return proj.Containers[i].Name < proj.Containers[j].Name
		})

		runningCount := 0
		for _, item := range proj.Containers {
			if item.State == "running" {
				runningCount++
			}
		}

		switch runningCount {
		case proj.ServicesCount:
			proj.Status = "running"
		case 0:
			proj.Status = "stopped"
		default:
			proj.Status = "partial"
		}

		result = append(result, *proj)
	}

	// 强制按 Compose 项目名称字典序稳定排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result, nil
}

// GetComposeProjectConfig 获取指定 Compose 项目的配置内容与路径
func GetComposeProjectConfig(ctx context.Context, projectName string) (string, string, error) {
	projects, err := GetComposeProjectsData(ctx)
	if err != nil {
		return "", "", err
	}

	var targetProj *ComposeProjectItem
	for i := range projects {
		if projects[i].Name == projectName {
			targetProj = &projects[i]
			break
		}
	}

	if targetProj == nil || targetProj.ConfigFiles == "" {
		return "", "", fmt.Errorf("未找到项目 %s 的配置文件", projectName)
	}

	filePath := targetProj.ConfigFiles
	files := strings.Split(filePath, ",")
	if len(files) > 0 {
		filePath = strings.TrimSpace(files[0])
	}

	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", filePath, err
	}

	return string(content), filePath, nil
}

// SaveComposeProjectConfig 保存修改后的 Compose 配置文件
func SaveComposeProjectConfig(ctx context.Context, projectName string, content string) error {
	_, filePath, err := GetComposeProjectConfig(ctx, projectName)
	if filePath == "" {
		if err != nil {
			return err
		}
		return fmt.Errorf("未找到项目 %s 的配置文件", projectName)
	}

	cleanPath := strings.ToLower(filePath)
	if !strings.HasSuffix(cleanPath, ".yml") && !strings.HasSuffix(cleanPath, ".yaml") {
		return fmt.Errorf("无效的配置文件格式，仅允许 .yml 或 .yaml 文件: %s", filePath)
	}

	return os.WriteFile(filePath, []byte(content), 0644)
}

// BatchComposeLifecycle 批量操作 Compose 项目关联容器生命周期 (start, stop, restart)
func BatchComposeLifecycle(ctx context.Context, projectName string, action string) error {
	projects, err := GetComposeProjectsData(ctx)
	if err != nil {
		return err
	}

	var targetProj *ComposeProjectItem
	for i := range projects {
		if projects[i].Name == projectName {
			targetProj = &projects[i]
			break
		}
	}

	if targetProj == nil || len(targetProj.Containers) == 0 {
		return fmt.Errorf("未找到 Compose 项目 %s 或该项目未包含任何服务容器", projectName)
	}

	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		return err
	}
	defer cli.Close()

	actionText := action
	switch action {
	case "start":
		actionText = "启动"
	case "stop":
		actionText = "停止"
	case "restart":
		actionText = "重启"
	}

	timeoutVal := 10
	var wg sync.WaitGroup
	var mu sync.Mutex
	var errMsgs []string

	for _, item := range targetProj.Containers {
		containerName := item.Name
		if t := GlobalQueue.GetTask(containerName); t != nil && (t.Status == "running" || t.Status == "waiting") {
			utils.LogWarning("容器 %s 当前正处于升级队列中，跳过生命周期操作", containerName)
			continue
		}

		wg.Add(1)
		go func(cName string) {
			defer wg.Done()

			var opErr error
			switch action {
			case "start":
				opErr = cli.ContainerStart(ctx, cName, types.ContainerStartOptions{})
			case "stop":
				opErr = cli.ContainerStop(ctx, cName, container.StopOptions{Timeout: &timeoutVal})
			case "restart":
				opErr = cli.ContainerRestart(ctx, cName, container.StopOptions{Timeout: &timeoutVal})
			default:
				opErr = fmt.Errorf("不支持的生命周期操作类型: %s", action)
			}

			mu.Lock()
			defer mu.Unlock()
			if opErr != nil {
				utils.LogError("Compose 项目 [%s] 的容器 %s 执行 %s 失败: %v", projectName, cName, actionText, opErr)
				errMsgs = append(errMsgs, fmt.Sprintf("%s: %v", cName, opErr))
			} else {
				utils.LogSuccess("Compose 项目 [%s] 的容器 %s 已成功执行 %s", projectName, cName, actionText)
			}
		}(containerName)
	}

	wg.Wait()

	if len(errMsgs) > 0 {
		return fmt.Errorf("对项目 %s 执行%s操作时部分服务失败: %s", projectName, actionText, strings.Join(errMsgs, "; "))
	}

	return nil
}


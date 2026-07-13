package api

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"
	"docker-updater/service"
	"docker-updater/utils"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/gin-gonic/gin"
)

// WebFS 前端静态资源 FS，在 main.go 中注入
var WebFS embed.FS

// CustomGinLogger 自定义 Gin 访问日志中间件
func CustomGinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		// 过滤 WebSocket 高频日志以降低噪声
		if strings.HasSuffix(path, "/api/ws") {
			return
		}

		latency := time.Since(start)
		status := c.Writer.Status()
		method := c.Request.Method
		clientIP := c.ClientIP()

		if status >= 400 {
			utils.LogWarning("HTTP | %3d | %13v | %15s | %s | %s %s",
				status, latency, clientIP, method, path, query)
		} else {
			utils.LogInfo("HTTP | %3d | %13v | %15s | %s | %s %s",
				status, latency, clientIP, method, path, query)
		}
	}
}

// InitRoutes 初始化 Gin 路由
func InitRoutes(r *gin.Engine) {
	r.Use(CustomGinLogger())

	// 所有服务强制挂载于 /app/docker-updater 前缀下
	group := r.Group("/app/docker-updater")

	// 1. 前端静态资源托管
	assetsFS, err := fs.Sub(WebFS, "frontend/dist/assets")
	if err != nil {
		utils.LogWarning("加载前端静态资源子目录失败: %s", err.Error())
	} else {
		// 挂载资源文件
		group.StaticFS("/assets", http.FS(assetsFS))
	}

	// 首页托管与 SPA 路由 Fallback
	group.GET("/", func(c *gin.Context) {
		serveIndex(c)
	})
	group.GET("/containers", func(c *gin.Context) {
		serveIndex(c)
	})
	group.GET("/history", func(c *gin.Context) {
		serveIndex(c)
	})
	group.GET("/images", func(c *gin.Context) {
		serveIndex(c)
	})
	group.GET("/settings", func(c *gin.Context) {
		serveIndex(c)
	})

	// 2. API 路由接口
	api := group.Group("/api")
	{
		api.GET("/ws", HandleWebSocket)
		api.GET("/status", apiStatus)
		api.POST("/check", apiCheck)
		api.GET("/update/:name", apiUpdate)
		api.GET("/rollback/:name", apiRollback)
		api.DELETE("/backup/:name", apiBackupDelete)
		api.GET("/settings", apiSettingsGet)
		api.POST("/settings", apiSettingsPost)
		api.POST("/defer/:name", apiDefer)
		api.POST("/undefer/:name", apiUndefer)
		api.GET("/container/:name/logs", apiContainerLogs)
		api.GET("/update-log/:name", apiUpdateLogGet)
		api.DELETE("/update-log/:name", apiUpdateLogDelete)
		api.GET("/images", apiImagesGet)
		api.DELETE("/image", apiImageDelete)
		api.POST("/images/prune", apiImagesPrune)
		api.GET("/tasks", apiTasksGet)
		api.POST("/tasks/cancel/:name", apiTaskCancel)
		api.GET("/system/logs", apiSystemLogsGet)
		api.DELETE("/system/logs", apiSystemLogsDelete)
		api.DELETE("/history", apiHistoryClear)
		api.DELETE("/history/:id", apiHistoryDelete)

		// 容器生命周期控制
		api.POST("/container/:name/start", apiContainerStart)
		api.POST("/container/:name/stop", apiContainerStop)
		api.POST("/container/:name/restart", apiContainerRestart)

		// 私有仓凭据管理 REST API
		api.GET("/registries", apiRegistriesGet)
		api.POST("/registries", apiRegistriesPost)
		api.DELETE("/registries/:id", apiRegistriesDelete)


		api.POST("/settings/test-email", apiSettingsTestEmail)
		api.GET("/image/tags", apiImageTags)
	}

	// 其他路径返回 404
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
}

func serveIndex(c *gin.Context) {
	data, err := WebFS.ReadFile("frontend/dist/index.html")
	if err != nil {
		c.String(http.StatusNotFound, "index.html 缺失")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

// apiStatus 返回当前本机所有容器运行与版本更新状态
func apiStatus(c *gin.Context) {
	data, err := service.GetContainerStatusData(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}


// apiCheck 触发手动更新比对
func apiCheck(c *gin.Context) {
	if !service.TryStartScan() {
		c.JSON(http.StatusOK, gin.H{"ok": true, "message": "已有更新比对检测任务在后台运行中，请勿重复触发"})
		return
	}

	utils.LogInfo("手动镜像更新比对比对检查已被用户触发...")
	go func() {
		defer service.EndScan()
		ctx := context.Background()
		results, err := dockerclient.ScanLocalHostForUpdates(ctx)
		if err == nil {
			for _, res := range results {
				if res.HasUpdate {
					updateEntry := db.AvailableUpdate{
						ContainerName:  res.ContainerName,
						Image:          res.Image,
						LocalDigest:    res.LocalDigest,
						RemoteDigest:   res.RemoteDigest,
						CheckedAt:      res.CheckedAt,
						ComposeProject: res.ComposeProject,
					}
					db.DB.Save(&updateEntry)
				} else {
					db.DB.Delete(&db.AvailableUpdate{ContainerName: res.ContainerName})
				}
			}
			if service.GlobalObserver != nil {
				service.GlobalObserver.OnStatusChange()
			}
		}
		_ = db.SetSetting("last_check_time", time.Now().UTC().Format(time.RFC3339))
	}()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiUpdate 升级容器，加入任务队列并输出流式日志进度
func apiUpdate(c *gin.Context) {
	name := c.Param("name")
	targetImage := c.Query("target_image")
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// 1. 将任务添加至全局队列。若已存在，则返回已有的 Task 实例。
	task := service.GlobalQueue.AddTask(name, service.TaskUpdate, targetImage, false)

	// 2. 初始化通道并注册监听器
	logChan := make(chan string, 50)
	task.AddListener(logChan)
	defer task.RemoveListener(logChan)

	// 3. 发送当前任务已有的全部历史日志，以支持重连后的回显。
	historicalLogs := task.GetLogs()
	for _, line := range historicalLogs {
		c.SSEvent("message", line)
	}

	// 4. 定时发送周期心跳包以维持连接活性，防范代理超时中断。
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 5. 订阅并分发实时日志流
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-logChan:
			if ok {
				c.SSEvent("message", msg)
				return true
			}
			return false
		case <-ticker.C:
			c.SSEvent("ping", "keepalive")
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// apiRollback 容器一键回滚，加入任务队列并以 SSE 输出日志流
func apiRollback(c *gin.Context) {
	name := c.Param("name")
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")
	c.Header("Transfer-Encoding", "chunked")

	// 1. 将任务添加至全局队列。
	task := service.GlobalQueue.AddTask(name, service.TaskRollback, "", false)

	// 2. 初始化通道并注册监听器
	logChan := make(chan string, 50)
	task.AddListener(logChan)
	defer task.RemoveListener(logChan)

	// 3. 发送当前任务已有的全部历史日志，以支持重连后的回显。
	historicalLogs := task.GetLogs()
	for _, line := range historicalLogs {
		c.SSEvent("message", line)
	}

	// 4. 定时发送周期心跳包以维持连接活性，防范代理超时中断。
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 5. 订阅并分发实时日志流
	c.Stream(func(w io.Writer) bool {
		select {
		case msg, ok := <-logChan:
			if ok {
				c.SSEvent("message", msg)
				return true
			}
			return false
		case <-ticker.C:
			c.SSEvent("ping", "keepalive")
			return true
		case <-c.Request.Context().Done():
			return false
		}
	})
}

// apiBackupDelete 删除旧备份释放存储
func apiBackupDelete(c *gin.Context) {
	name := c.Param("name")
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	backupName := name + "_backup_docker_updater"
	err = cli.ContainerRemove(c, backupName, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		utils.LogError("手动删除备份容器 %s 失败: %s", backupName, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	db.DB.Delete(&db.RollbackMetadata{ContainerName: name})
	utils.LogSuccess("手动删除备份容器 %s 成功", backupName)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiSettingsGet 获取全局配置
func apiSettingsGet(c *gin.Context) {
	backupEnabled := db.GetSetting("backup_enabled", "false") == "true"
	backupHours, _ := strconv.Atoi(db.GetSetting("backup_hours", "24"))
	restartStack := db.GetSetting("restart_stack", "false") == "true"
	autoUpdateEnabled := db.GetSetting("auto_update_enabled", "false") == "true"
	checkType := db.GetSetting("check_type", "day")
	checkValueStr := db.GetSetting("check_value", "1")
	checkValue, _ := strconv.Atoi(checkValueStr)
	if checkValue <= 0 {
		checkValue = 1
	}

	tempMirrorsStr := db.GetSetting("temp_mirrors", "[]")
	var tempMirrors []string
	_ = json.Unmarshal([]byte(tempMirrorsStr), &tempMirrors)
	if tempMirrors == nil {
		tempMirrors = []string{}
	}

	notifyEnabled := db.GetSetting("notify_enabled", "")
	if notifyEnabled == "" {
		notifyEnabled = db.GetSetting("smtp_enabled", "false")
	}
	notifyType := db.GetSetting("notify_type", "email")

	smtpEnabled := notifyEnabled == "true"
	smtpHost := db.GetSetting("smtp_host", "")
	smtpPort := db.GetSetting("smtp_port", "465")
	smtpUsername := db.GetSetting("smtp_username", "")
	smtpPassword := db.GetSetting("smtp_password", "")
	smtpSSL := db.GetSetting("smtp_ssl", "true") == "true"
	smtpTo := db.GetSetting("smtp_to", "")
	smtpSubjectTemplate := db.GetSetting("smtp_subject_template", utils.DefaultSMTPSubject)
	smtpBodyTemplate := db.GetSetting("smtp_body_template", utils.DefaultSMTPBody)
	smtpSubjectTemplateCheck := db.GetSetting("smtp_subject_template_check", utils.DefaultSMTPSubjectCheck)
	smtpBodyTemplateCheck := db.GetSetting("smtp_body_template_check", utils.DefaultSMTPBodyCheck)

	webhookURL := db.GetSetting("webhook_url", "")
	webhookMethod := db.GetSetting("webhook_method", "POST")
	webhookTemplate := db.GetSetting("webhook_template", utils.DefaultWebhookTemplate)
	webhookTemplateCheck := db.GetSetting("webhook_template_check", utils.DefaultWebhookTemplateCheck)

	c.JSON(http.StatusOK, gin.H{
		"backup_enabled":              backupEnabled,
		"backup_hours":                backupHours,
		"restart_stack":               restartStack,
		"auto_update_enabled":         autoUpdateEnabled,
		"temp_mirrors":                tempMirrors,
		"check_type":                  checkType,
		"check_value":                 checkValue,
		"notify_enabled":              notifyEnabled == "true",
		"notify_type":                 notifyType,
		"smtp_enabled":                smtpEnabled,
		"smtp_host":                   smtpHost,
		"smtp_port":                   smtpPort,
		"smtp_username":               smtpUsername,
		"smtp_password":               smtpPassword,
		"smtp_ssl":                    smtpSSL,
		"smtp_to":                     smtpTo,
		"smtp_subject_template":       smtpSubjectTemplate,
		"smtp_body_template":          smtpBodyTemplate,
		"smtp_subject_template_check": smtpSubjectTemplateCheck,
		"smtp_body_template_check":     smtpBodyTemplateCheck,
		"webhook_url":                 webhookURL,
		"webhook_method":              webhookMethod,
		"webhook_template":            webhookTemplate,
		"webhook_template_check":      webhookTemplateCheck,
	})
}

// apiSettingsPost 更新全局配置
func apiSettingsPost(c *gin.Context) {
	var body service.GlobalSettings
	if err := c.ShouldBindJSON(&body); err != nil {
		utils.LogError("保存全局配置参数解析失败: %s", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := service.SaveGlobalSettings(body); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiSettingsTestEmail 测试发送邮件或 Webhook 通知
func apiSettingsTestEmail(c *gin.Context) {
	var body service.TestNotificationSettings
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	respBody, err := service.SendTestNotification(body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "response": respBody})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true, "response": respBody})
}

// apiImageTags 从对应远端拉取镜像的可用 tags（最多 20 个）
func apiImageTags(c *gin.Context) {
	imageName := c.Query("image")
	if imageName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "image parameter required"})
		return
	}
	tags, err := dockerclient.GetRemoteTags(imageName)
	if err != nil {
		utils.LogWarning("获取镜像 %s 的远端 Tags 失败: %s", imageName, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, tags)
}


// apiDefer 手动延迟更新
func apiDefer(c *gin.Context) {
	name := c.Param("name")
	var body struct {
		Days int `json:"days"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	untilDate := "forever"
	if body.Days > 0 {
		untilDate = time.Now().AddDate(0, 0, body.Days).Format("2006-01-02")
	}
	d := db.DeferredUpdate{
		ContainerName: name,
		Until:         untilDate,
	}
	if err := db.DB.Save(&d).Error; err != nil {
			utils.LogError("延迟更新容器 %s 失败: %s", name, err.Error())
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
	utils.LogSuccess("手动延迟更新容器 %s 成功 (延迟至: %s)", name, untilDate)
	c.JSON(http.StatusOK, gin.H{"ok": true, "until": untilDate})
}

// apiUndefer 撤销延期设置
func apiUndefer(c *gin.Context) {
	name := c.Param("name")
	if err := db.DB.Delete(&db.DeferredUpdate{ContainerName: name}).Error; err != nil {
		utils.LogError("撤销容器 %s 延迟更新设置失败: %s", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.LogSuccess("撤销容器 %s 延迟更新设置成功", name)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiContainerLogs 获取当前运行容器最末 200 行日志
func apiContainerLogs(c *gin.Context) {
	name := c.Param("name")
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	reader, err := cli.ContainerLogs(c, name, types.ContainerLogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Tail:       "200",
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer reader.Close()

	logBytes, err := io.ReadAll(reader)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 移除 docker log 头的 8 字节 frame header
	logs := parseDockerLogs(logBytes)
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// apiUpdateLogGet 获取历史产生的持久化日志
func apiUpdateLogGet(c *gin.Context) {
	name := c.Param("name")
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`, name); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid container name format"})
		return
	}

	// 优先在内存队列中查询该任务
	task := service.GlobalQueue.GetTask(name)
	if task != nil {
		c.JSON(http.StatusOK, gin.H{"found": true, "logs": task.GetLogs()})
		return
	}

	pkgVar := os.Getenv("TRIM_PKGVAR")
	if pkgVar == "" {
		pkgVar = "./data"
	}
	logFilePath := filepath.Join(pkgVar, "logs", fmt.Sprintf("%s.log", name))
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"found": false, "logs": []string{}})
		return
	}

	bytes, err := os.ReadFile(logFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	lines := strings.Split(string(bytes), "\n")
	c.JSON(http.StatusOK, gin.H{"found": true, "logs": lines})
}

// apiUpdateLogDelete 物理清除指定容器的升级日志文件。
func apiUpdateLogDelete(c *gin.Context) {
	name := c.Param("name")
	if matched, _ := regexp.MatchString(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`, name); !matched {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid container name format"})
		return
	}
	pkgVar := os.Getenv("TRIM_PKGVAR")
	if pkgVar == "" {
		pkgVar = "./data"
	}
	logFilePath := filepath.Join(pkgVar, "logs", fmt.Sprintf("%s.log", name))
	if err := os.Remove(logFilePath); err != nil && !os.IsNotExist(err) {
		utils.LogError("物理删除指定容器升级日志文件失败: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.LogSuccess("物理删除指定容器升级日志文件成功: %s", name)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// parseDockerLogs 移除 Docker Stdout/Stderr 日志中特有的 8 字节消息头
func parseDockerLogs(raw []byte) string {
	var builder strings.Builder
	for len(raw) >= 8 {
		// 前 8 字节为 Header (第 0 字节为 Stream 类型 1=stdout, 2=stderr; 4-7 字节为消息长度)
		msgLen := int(raw[4])<<24 | int(raw[5])<<16 | int(raw[6])<<8 | int(raw[7])
		raw = raw[8:]
		if len(raw) < msgLen {
			builder.Write(raw)
			break
		}
		builder.Write(raw[:msgLen])
		raw = raw[msgLen:]
	}
	return builder.String()
}

// apiImagesGet 获取宿主机本地所有镜像列表
func apiImagesGet(c *gin.Context) {
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	// 1. 获取所有容器建立镜像 ID ↔ 容器名映射
	containers, err := cli.ContainerList(c, types.ContainerListOptions{All: true})
	imageToContainers := make(map[string][]string)
	if err == nil {
		for _, containerItem := range containers {
			name := ""
			if len(containerItem.Names) > 0 {
				name = strings.TrimPrefix(containerItem.Names[0], "/")
			}
			if name != "" && containerItem.ImageID != "" {
				imageToContainers[containerItem.ImageID] = append(imageToContainers[containerItem.ImageID], name)
			}
		}
	}

	// 2. 获取镜像列表
	list, err := cli.ImageList(c, types.ImageListOptions{All: true})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var result []gin.H
	for _, item := range list {
		tags := item.RepoTags
		if len(tags) == 0 {
			tags = []string{"<none>:<none>"}
		}
		
		// 绑定占用该镜像的容器名
		associated := imageToContainers[item.ID]
		if associated == nil {
			associated = []string{}
		}

		arch := ""
		inspect, _, err := cli.ImageInspectWithRaw(c, item.ID)
		if err == nil {
			arch = inspect.Architecture
			if inspect.Variant != "" {
				arch = arch + "/" + inspect.Variant
			}
		}

		result = append(result, gin.H{
			"id":           item.ID,
			"tags":         tags,
			"size":         item.Size,
			"created":      item.Created,
			"containers":   associated,
			"architecture": arch,
		})
	}

	c.JSON(http.StatusOK, result)
}

// apiImageDelete 物理删除指定镜像
func apiImageDelete(c *gin.Context) {
	id := c.Query("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "id parameter required"})
		return
	}

	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	// 1. 获取所有容器建立镜像 ID ↔ 容器依赖映射 (新增安全依赖校验)
	containers, err := cli.ContainerList(c, types.ContainerListOptions{All: true})
	var associated []string
	if err == nil {
		for _, containerItem := range containers {
			if containerItem.ImageID == id || strings.TrimPrefix(containerItem.ImageID, "sha256:") == strings.TrimPrefix(id, "sha256:") {
				name := ""
				if len(containerItem.Names) > 0 {
					name = strings.TrimPrefix(containerItem.Names[0], "/")
				}
				if name != "" {
					associated = append(associated, name)
				}
			}
		}
	}

	forceStr := c.DefaultQuery("force", "false")
	force := forceStr == "true"

	if len(associated) > 0 && !force {
		c.JSON(http.StatusConflict, gin.H{
			"error":      fmt.Sprintf("镜像正在被容器 [%s] 使用，无法删除。请先停止并删除关联容器，或使用强制删除。", strings.Join(associated, ", ")),
			"associated": associated,
		})
		return
	}

	// 只有在明确传递 force=true 时才调用 ImageRemove 并指定 Force
	_, err = cli.ImageRemove(c, id, types.ImageRemoveOptions{Force: force, PruneChildren: true})
	if err != nil {
		utils.LogError("手动物理删除镜像 %s 失败: %s", id, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.LogSuccess("手动物理删除镜像 %s 成功", id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiImagesPrune 清理所有虚悬 (dangling=true) 的旧镜像垃圾
func apiImagesPrune(c *gin.Context) {
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	pruneFilters := filters.NewArgs()
	pruneFilters.Add("dangling", "true")
	// 默认只清理 24 小时前的虚悬镜像，防止误删用户刚刚拉取或正在 build 的中间层
	pruneFilters.Add("until", "24h")

	report, err := cli.ImagesPrune(c, pruneFilters)
	if err != nil {
		utils.LogError("清理无用虚悬镜像失败: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	utils.LogSuccess("清理无用虚悬镜像成功，共释放空间: %d 字节, 清除镜像数: %d", report.SpaceReclaimed, len(report.ImagesDeleted))
	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"space_reclaimed": report.SpaceReclaimed,
		"deleted_count":   len(report.ImagesDeleted),
	})
}

// apiTasksGet 获取当前后台排队队列状态
func apiTasksGet(c *gin.Context) {
	queued, active := service.GlobalQueue.GetQueueState()
	c.JSON(http.StatusOK, gin.H{
		"queued": queued,
		"active": active,
	})
}

// apiTaskCancel 取消某个还在排队中（未执行）的升级任务
func apiTaskCancel(c *gin.Context) {
	name := c.Param("name")
	success := service.GlobalQueue.CancelTask(name)
	if success {
		utils.LogSuccess("手动取消容器 %s 的排队升级任务成功", name)
	} else {
		utils.LogWarning("手动取消容器 %s 的排队升级任务失败 (任务不存在或已在执行)", name)
	}
	c.JSON(http.StatusOK, gin.H{"success": success})
}

// apiSystemLogsGet 获取守护进程自身的 info.log 运行日志（最末 400 行）
func apiSystemLogsGet(c *gin.Context) {
	pkgVar := os.Getenv("TRIM_PKGVAR")
	if pkgVar == "" {
		pkgVar = "./data"
	}
	logFilePath := filepath.Join(pkgVar, "info.log")
	if _, err := os.Stat(logFilePath); os.IsNotExist(err) {
		c.JSON(http.StatusOK, gin.H{"logs": []string{}})
		return
	}

	bytes, err := os.ReadFile(logFilePath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	allLines := strings.Split(string(bytes), "\n")
	// 先过滤空行，保留非空日志
	var nonEmptyLines []string
	for _, l := range allLines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			nonEmptyLines = append(nonEmptyLines, l)
		}
	}

	// 再截取最后 400 行有效日志
	start := 0
	if len(nonEmptyLines) > 400 {
		start = len(nonEmptyLines) - 400
	}
	result := nonEmptyLines[start:]

	c.JSON(http.StatusOK, gin.H{"logs": result})

}

// apiSystemLogsDelete 物理清空全局 info.log 日志内容
func apiSystemLogsDelete(c *gin.Context) {
	pkgVar := os.Getenv("TRIM_PKGVAR")
	if pkgVar == "" {
		pkgVar = "./data"
	}
	logFilePath := filepath.Join(pkgVar, "info.log")
	// 使用 O_TRUNC 打开以截断清空内容，防止文件写句柄失效
	err := os.WriteFile(logFilePath, []byte(""), 0644)
	if err != nil {
		utils.LogError("物理清空系统日志失败: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.LogSuccess("物理清空系统日志成功")
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiRegistriesGet 获取私有仓凭证列表（密码脱敏）
func apiRegistriesGet(c *gin.Context) {
	var list []db.RegistryCredential
	if err := db.DB.Find(&list).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	for i := range list {
		list[i].Password = "******"
	}
	c.JSON(http.StatusOK, list)
}

// apiRegistriesPost 保存/编辑私有仓凭证
func apiRegistriesPost(c *gin.Context) {
	var body struct {
		ID       uint   `json:"id"`
		Registry string `json:"registry" binding:"required"`
		Username string `json:"username" binding:"required"`
		Password string `json:"password" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	reg := strings.TrimSpace(body.Registry)
	reg = strings.TrimPrefix(reg, "https://")
	reg = strings.TrimPrefix(reg, "http://")
	reg = strings.TrimSuffix(reg, "/")

	var cred db.RegistryCredential
	if body.ID > 0 {
		if err := db.DB.First(&cred, body.ID).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "credential not found"})
			return
		}
		cred.Registry = reg
		cred.Username = body.Username
		if body.Password != "******" {
			cred.Password = body.Password
		}
	} else {
		cred.Registry = reg
		cred.Username = body.Username
		cred.Password = body.Password
	}
	cred.UpdatedAt = time.Now().Format(time.RFC3339)

	if err := db.DB.Save(&cred).Error; err != nil {
		utils.LogError("保存私有仓凭证 %s 失败: %s", reg, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.LogSuccess("保存私有仓凭证 %s 成功 (用户名: %s)", reg, body.Username)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiRegistriesDelete 删除私有仓凭证
func apiRegistriesDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}
	if err := db.DB.Delete(&db.RegistryCredential{}, id).Error; err != nil {
		utils.LogError("删除私有仓凭证 ID %d 失败: %s", id, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.LogSuccess("删除私有仓凭证 ID %d 成功", id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiContainerStart 启动已停止的容器
func apiContainerStart(c *gin.Context) {
	name := c.Param("name")

	// 校验队列排他锁
	if t := service.GlobalQueue.GetTask(name); t != nil && (t.Status == "running" || t.Status == "waiting") {
		c.JSON(http.StatusConflict, gin.H{"error": "该容器当前正处于队列任务中，无法执行生命周期操作"})
		return
	}

	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	if err := cli.ContainerStart(c, name, types.ContainerStartOptions{}); err != nil {
		utils.LogError("手动启动容器 %s 失败: %s", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.LogSuccess("手动启动容器 %s 成功", name)
	go GlobalHub.BroadcastStatus()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiContainerStop 停止运行中的容器
func apiContainerStop(c *gin.Context) {
	name := c.Param("name")

	// 校验队列排他锁
	if t := service.GlobalQueue.GetTask(name); t != nil && (t.Status == "running" || t.Status == "waiting") {
		c.JSON(http.StatusConflict, gin.H{"error": "该容器当前正处于队列任务中，无法执行生命周期操作"})
		return
	}

	timeoutStr := c.DefaultQuery("timeout", "10")
	timeoutVal, err := strconv.Atoi(timeoutStr)
	if err != nil || timeoutVal < 0 {
		timeoutVal = 10
	}

	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	if err := cli.ContainerStop(c, name, container.StopOptions{Timeout: &timeoutVal}); err != nil {
		utils.LogError("手动停止容器 %s 失败: %s", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.LogSuccess("手动停止容器 %s 成功", name)
	go GlobalHub.BroadcastStatus()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiContainerRestart 重启容器
func apiContainerRestart(c *gin.Context) {
	name := c.Param("name")

	// 校验队列排他锁
	if t := service.GlobalQueue.GetTask(name); t != nil && (t.Status == "running" || t.Status == "waiting") {
		c.JSON(http.StatusConflict, gin.H{"error": "该容器当前正处于队列任务中，无法执行生命周期操作"})
		return
	}

	timeoutStr := c.DefaultQuery("timeout", "10")
	timeoutVal, err := strconv.Atoi(timeoutStr)
	if err != nil || timeoutVal < 0 {
		timeoutVal = 10
	}

	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	if err := cli.ContainerRestart(c, name, container.StopOptions{Timeout: &timeoutVal}); err != nil {
		utils.LogError("手动重启容器 %s 失败: %s", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	utils.LogSuccess("手动重启容器 %s 成功", name)
	go GlobalHub.BroadcastStatus()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiHistoryClear 清空所有容器升级历史记录及相关的本地部署日志文件
func apiHistoryClear(c *gin.Context) {
	// 1. 清空 update_histories 数据库表
	if err := db.DB.Exec("DELETE FROM update_histories").Error; err != nil {
		utils.LogError("清空升级历史记录失败: %s", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 2. 遍历删除 logs 目录下的所有本地升级日志文件
	pkgVar := os.Getenv("TRIM_PKGVAR")
	if pkgVar == "" {
		pkgVar = "./data"
	}
	logsDir := filepath.Join(pkgVar, "logs")
	_ = filepath.Walk(logsDir, func(path string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() && strings.HasSuffix(info.Name(), ".log") {
			_ = os.Remove(path)
		}
		return nil
	})

	utils.LogSuccess("清空所有升级历史记录成功")
	// 3. 广播给所有的前端客户端更新状态
	go GlobalHub.BroadcastStatus()

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiHistoryDelete 删除单条升级历史记录及相关的日志文件
func apiHistoryDelete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.Atoi(idStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	// 1. 获取对应的历史记录以获知容器名称
	var hist db.UpdateHistory
	if err := db.DB.First(&hist, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "history not found"})
		return
	}

	// 2. 从数据库删除
	if err := db.DB.Delete(&db.UpdateHistory{}, id).Error; err != nil {
		utils.LogError("手动删除容器 %s 的升级历史记录失败: %s", hist.ContainerName, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 3. 删除对应的物理日志文件
	pkgVar := os.Getenv("TRIM_PKGVAR")
	if pkgVar == "" {
		pkgVar = "./data"
	}
	logFilePath := filepath.Join(pkgVar, "logs", fmt.Sprintf("%s.log", hist.ContainerName))
	_ = os.Remove(logFilePath)

	utils.LogSuccess("手动删除容器 %s 的升级历史记录成功", hist.ContainerName)
	// 4. 广播给所有的前端客户端更新状态
	go GlobalHub.BroadcastStatus()

	c.JSON(http.StatusOK, gin.H{"ok": true})
}



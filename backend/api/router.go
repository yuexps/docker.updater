package api

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"
	"docker-updater/scheduler"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/gin-gonic/gin"
)

// WebFS 前端静态资源 FS，在 main.go 中注入
var WebFS embed.FS

// InitRoutes 初始化 Gin 路由
func InitRoutes(r *gin.Engine) {
	// 所有服务强制挂载于 /app/docker-updater 前缀下
	group := r.Group("/app/docker-updater")

	// 1. 前端静态资源托管
	assetsFS, err := fs.Sub(WebFS, "dist/assets")
	if err == nil {
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

		// 镜像加速源只读 API
		api.GET("/settings/system-mirrors", apiSettingsSystemMirrorsGet)
		api.POST("/settings/test-email", apiSettingsTestEmail)
	}

	// 其他路径返回 404
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
	})
}

func serveIndex(c *gin.Context) {
	data, err := WebFS.ReadFile("dist/index.html")
	if err != nil {
		c.String(http.StatusNotFound, "index.html 缺失")
		return
	}
	c.Data(http.StatusOK, "text/html; charset=utf-8", data)
}

// apiStatus 返回当前本机所有容器运行与版本更新状态
func apiStatus(c *gin.Context) {
	data, err := GetStatusData(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, data)
}

// GetStatusData 抽离的系统与容器状态数据获取公共函数，供 REST API 和 WebSocket 共用
func GetStatusData(ctx context.Context) (gin.H, error) {
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return nil, err
	}

	// 载入数据库状态
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
	var result []gin.H

	for _, containerItem := range containers {
		name := ""
		if len(containerItem.Names) > 0 {
			name = strings.TrimPrefix(containerItem.Names[0], "/")
		}
		if name == "" || strings.HasSuffix(name, "_old") {
			continue // 忽略备份容器
		}

		inspect, err := cli.ContainerInspect(ctx, containerItem.ID)
		if err != nil {
			continue
		}

		imageName := inspect.Config.Image
		// 检查本地镜像元数据
		imageInspect, _, err := cli.ImageInspectWithRaw(ctx, inspect.Image)
		if err != nil || len(imageInspect.RepoDigests) == 0 {
			continue // 忽略本地直接构建且无 Registry digest 镜像
		}

		status := "ok"
		var checkedAt string
		var deferUntil *string

		// 检查是否有新版本
		info, hasUpdate := availMap[name]
		if hasUpdate {
			checkedAt = info.CheckedAt
			// 检查是否已被延迟
			if d, isDeferred := deferMap[name]; isDeferred && d.Until > today {
				status = "deferred"
				val := d.Until
				deferUntil = &val
			} else if d, isDeferred := deferMap[name]; isDeferred && d.Until <= today {
				// 已过期延迟，自动从数据库移去
				db.DB.Delete(&d)
			}
			if status != "deferred" {
				status = "update"
			}
		}

		// 检查是否有可用备份回滚点
		rb, hasRollback := rbMap[name]
		var rollbackExpires *string
		if hasRollback {
			// 检查物理容器是否存在
			if _, err := cli.ContainerInspect(ctx, name+"_old"); err == nil {
				val := rb.ExpiresAt
				rollbackExpires = &val
			} else {
				hasRollback = false
				db.DB.Delete(&rb)
			}
		}

		localDigest := ""
		remoteDigest := ""
		if hasUpdate {
			localDigest = info.LocalDigest
			remoteDigest = info.RemoteDigest
		}

		result = append(result, gin.H{
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

	// 获取最近历史
	var history []db.UpdateHistory
	db.DB.Order("updated_at desc").Limit(100).Find(&history)

	lastCheck := db.GetSetting("last_check_time", "")
	queued, active := dockerclient.GlobalQueue.GetQueueState()
	return gin.H{
		"containers": result,
		"last_check": lastCheck,
		"history":    history,
		"active":     active,
		"queued":     queued,
	}, nil
}

// apiCheck 触发手动更新比对
func apiCheck(c *gin.Context) {
	log.Printf("[INFO] 手动镜像更新比对比对检查已被用户触发...\n")
	go func() {
		ctx := context.Background()
		_ = dockerclient.ScanLocalHostForUpdates(ctx)
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

	// 1. 加入队列中（如果已在队列则直接返回已有 Task）
	task := dockerclient.GlobalQueue.AddTask(name, dockerclient.TaskUpdate, targetImage, false)

	// 2. 建立监听者
	logChan := make(chan string, 50)
	task.AddListener(logChan)
	defer task.RemoveListener(logChan)

	// 3. 首先推送当前任务已经累积的全部历史日志（用以支持中途重连/页面刷新回显）
	historicalLogs := task.GetLogs()
	for _, line := range historicalLogs {
		c.SSEvent("message", line)
	}

	// 4. 定时发送 ping 维持连接活跃，防范反向代理网关超时断开
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 5. 实时订阅新日志
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

	// 1. 加入队列中
	task := dockerclient.GlobalQueue.AddTask(name, dockerclient.TaskRollback, "", false)

	// 2. 建立监听者
	logChan := make(chan string, 50)
	task.AddListener(logChan)
	defer task.RemoveListener(logChan)

	// 3. 首先推送当前任务已经累积的全部历史日志
	historicalLogs := task.GetLogs()
	for _, line := range historicalLogs {
		c.SSEvent("message", line)
	}

	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	// 4. 实时订阅新日志
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

	backupName := name + "_old"
	err = cli.ContainerRemove(c, backupName, types.ContainerRemoveOptions{Force: true})
	if err != nil {
		log.Printf("[ERROR] 手动删除备份容器 %s 失败: %s\n", backupName, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	db.DB.Delete(&db.RollbackMetadata{ContainerName: name})
	log.Printf("[SUCCESS] 手动删除备份容器 %s 成功\n", backupName)
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

	smtpEnabled := db.GetSetting("smtp_enabled", "false") == "true"
	smtpHost := db.GetSetting("smtp_host", "")
	smtpPort := db.GetSetting("smtp_port", "465")
	smtpUsername := db.GetSetting("smtp_username", "")
	smtpPassword := db.GetSetting("smtp_password", "")
	smtpSSL := db.GetSetting("smtp_ssl", "true") == "true"
	smtpTo := db.GetSetting("smtp_to", "")
	smtpSubjectTemplate := db.GetSetting("smtp_subject_template", dockerclient.DefaultSMTPSubject)
	smtpBodyTemplate := db.GetSetting("smtp_body_template", dockerclient.DefaultSMTPBody)

	c.JSON(http.StatusOK, gin.H{
		"backup_enabled":         backupEnabled,
		"backup_hours":           backupHours,
		"restart_stack":          restartStack,
		"auto_update_enabled":    autoUpdateEnabled,
		"temp_mirrors":           tempMirrors,
		"check_type":             checkType,
		"check_value":            checkValue,
		"smtp_enabled":           smtpEnabled,
		"smtp_host":              smtpHost,
		"smtp_port":              smtpPort,
		"smtp_username":          smtpUsername,
		"smtp_password":          smtpPassword,
		"smtp_ssl":               smtpSSL,
		"smtp_to":                smtpTo,
		"smtp_subject_template":  smtpSubjectTemplate,
		"smtp_body_template":     smtpBodyTemplate,
	})
}

// apiSettingsPost 更新全局配置
func apiSettingsPost(c *gin.Context) {
	var body struct {
		BackupEnabled       bool     `json:"backup_enabled"`
		BackupHours         int      `json:"backup_hours"`
		RestartStack        bool     `json:"restart_stack"`
		AutoUpdateEnabled   bool     `json:"auto_update_enabled"`
		TempMirrors         []string `json:"temp_mirrors"`
		CheckType           string   `json:"check_type"`
		CheckValue          int      `json:"check_value"`
		SMTPEnabled         bool     `json:"smtp_enabled"`
		SMTPHost            string   `json:"smtp_host"`
		SMTPPort            string   `json:"smtp_port"`
		SMTPUsername        string   `json:"smtp_username"`
		SMTPPassword        string   `json:"smtp_password"`
		SMTPSSL             bool     `json:"smtp_ssl"`
		SMTPTo              string   `json:"smtp_to"`
		SMTPSubjectTemplate string   `json:"smtp_subject_template"`
		SMTPBodyTemplate    string   `json:"smtp_body_template"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		log.Printf("[ERROR] 保存全局配置参数解析失败: %s\n", err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_ = db.SetSetting("backup_enabled", strconv.FormatBool(body.BackupEnabled))
	_ = db.SetSetting("backup_hours", strconv.Itoa(body.BackupHours))
	_ = db.SetSetting("restart_stack", strconv.FormatBool(body.RestartStack))
	_ = db.SetSetting("auto_update_enabled", strconv.FormatBool(body.AutoUpdateEnabled))
	_ = db.SetSetting("smtp_enabled", strconv.FormatBool(body.SMTPEnabled))
	_ = db.SetSetting("smtp_host", body.SMTPHost)
	_ = db.SetSetting("smtp_port", body.SMTPPort)
	_ = db.SetSetting("smtp_username", body.SMTPUsername)
	_ = db.SetSetting("smtp_password", body.SMTPPassword)
	_ = db.SetSetting("smtp_ssl", strconv.FormatBool(body.SMTPSSL))
	_ = db.SetSetting("smtp_to", body.SMTPTo)
	_ = db.SetSetting("smtp_subject_template", body.SMTPSubjectTemplate)
	_ = db.SetSetting("smtp_body_template", body.SMTPBodyTemplate)

	if body.CheckType != "" {
		_ = db.SetSetting("check_type", body.CheckType)
	}
	if body.CheckValue > 0 {
		_ = db.SetSetting("check_value", strconv.Itoa(body.CheckValue))
	}

	if body.TempMirrors == nil {
		body.TempMirrors = []string{}
	}
	mirrorsBytes, _ := json.Marshal(body.TempMirrors)
	_ = db.SetSetting("temp_mirrors", string(mirrorsBytes))

	log.Printf("[SUCCESS] 保存全局配置成功 (备份使能: %t, 备份保留: %d小时, 重启Stack: %t, 邮件配置: %t)\n",
		body.BackupEnabled, body.BackupHours, body.RestartStack, body.SMTPEnabled)

	// 配置变更后重新加载定时任务调度器
	scheduler.ReloadScheduler()

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiSettingsTestEmail 测试发送邮件通知
func apiSettingsTestEmail(c *gin.Context) {
	var body struct {
		SMTPHost            string `json:"smtp_host"`
		SMTPPort            string `json:"smtp_port"`
		SMTPUsername        string `json:"smtp_username"`
		SMTPPassword        string `json:"smtp_password"`
		SMTPSSL             bool   `json:"smtp_ssl"`
		SMTPTo              string `json:"smtp_to"`
		SMTPSubjectTemplate string `json:"smtp_subject_template"`
		SMTPBodyTemplate    string `json:"smtp_body_template"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	subjectTpl := body.SMTPSubjectTemplate
	if subjectTpl == "" {
		subjectTpl = dockerclient.DefaultSMTPSubject
	}
	bodyTpl := body.SMTPBodyTemplate
	if bodyTpl == "" {
		bodyTpl = dockerclient.DefaultSMTPBody
	}

	r := strings.NewReplacer(
		"{container_name}", "test-mysql",
		"{action_type}", "版本修改",
		"{status}", "测试成功",
		"{time}", time.Now().Local().Format("2006-01-02 15:04:05"),
		"{logs}", "[PULL] Pulling image mysql:8.0\n[INFO] Stopping old container\n[INFO] Starting new container\n[SUCCESS] Container updated successfully",
	)

	subject := r.Replace(subjectTpl)
	bodyText := r.Replace(bodyTpl)

	err := dockerclient.SendEmailRaw(
		body.SMTPHost,
		body.SMTPPort,
		body.SMTPUsername,
		body.SMTPPassword,
		body.SMTPTo,
		body.SMTPSSL,
		subject,
		bodyText,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type DockerDaemonConfig struct {
	RegistryMirrors []string `json:"registry-mirrors"`
}

// apiSettingsSystemMirrorsGet 只读获取宿主机 daemon.json 加速源
func apiSettingsSystemMirrorsGet(c *gin.Context) {
	filePath := "/etc/docker/daemon.json"
	fileBytes, err := os.ReadFile(filePath)
	if err != nil {
		c.JSON(http.StatusOK, []string{})
		return
	}

	var config DockerDaemonConfig
	if err := json.Unmarshal(fileBytes, &config); err != nil {
		c.JSON(http.StatusOK, []string{})
		return
	}

	if config.RegistryMirrors == nil {
		c.JSON(http.StatusOK, []string{})
		return
	}

	c.JSON(http.StatusOK, config.RegistryMirrors)
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
	if body.Days <= 0 {
		body.Days = 7
	}

	untilDate := time.Now().AddDate(0, 0, body.Days).Format("2006-01-02")
	d := db.DeferredUpdate{
		ContainerName: name,
		Until:         untilDate,
	}
	if err := db.DB.Save(&d).Error; err != nil {
		log.Printf("[ERROR] 延迟更新容器 %s 失败: %s\n", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[SUCCESS] 手动延迟更新容器 %s 成功 (延迟至: %s)\n", name, untilDate)
	c.JSON(http.StatusOK, gin.H{"ok": true, "until": untilDate})
}

// apiUndefer 撤销延期设置
func apiUndefer(c *gin.Context) {
	name := c.Param("name")
	if err := db.DB.Delete(&db.DeferredUpdate{ContainerName: name}).Error; err != nil {
		log.Printf("[ERROR] 撤销容器 %s 延迟更新设置失败: %s\n", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[SUCCESS] 撤销容器 %s 延迟更新设置成功\n", name)
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

// apiUpdateLogDelete 删除特定容器的升级日志文件
func apiUpdateLogDelete(c *gin.Context) {
	name := c.Param("name")
	pkgVar := os.Getenv("TRIM_PKGVAR")
	if pkgVar == "" {
		pkgVar = "./data"
	}
	logFilePath := filepath.Join(pkgVar, "logs", fmt.Sprintf("%s.log", name))
	err := os.Remove(logFilePath)
	if err != nil && !os.IsNotExist(err) {
		log.Printf("[ERROR] 删除容器 %s 升级日志文件失败: %s\n", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[SUCCESS] 删除容器 %s 升级日志文件成功\n", name)
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

	_, err = cli.ImageRemove(c, id, types.ImageRemoveOptions{Force: true, PruneChildren: true})
	if err != nil {
		log.Printf("[ERROR] 手动物理删除镜像 %s 失败: %s\n", id, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[SUCCESS] 手动物理删除镜像 %s 成功\n", id)
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

	report, err := cli.ImagesPrune(c, pruneFilters)
	if err != nil {
		log.Printf("[ERROR] 清理无用虚悬镜像失败: %s\n", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("[SUCCESS] 清理无用虚悬镜像成功，共释放空间: %d 字节, 清除镜像数: %d\n", report.SpaceReclaimed, len(report.ImagesDeleted))
	c.JSON(http.StatusOK, gin.H{
		"ok":              true,
		"space_reclaimed": report.SpaceReclaimed,
		"deleted_count":   len(report.ImagesDeleted),
	})
}

// apiTasksGet 获取当前后台排队队列状态
func apiTasksGet(c *gin.Context) {
	queued, active := dockerclient.GlobalQueue.GetQueueState()
	c.JSON(http.StatusOK, gin.H{
		"queued": queued,
		"active": active,
	})
}

// apiTaskCancel 取消某个还在排队中（未执行）的升级任务
func apiTaskCancel(c *gin.Context) {
	name := c.Param("name")
	success := dockerclient.GlobalQueue.CancelTask(name)
	if success {
		log.Printf("[SUCCESS] 手动取消容器 %s 的排队升级任务成功\n", name)
	} else {
		log.Printf("[WARNING] 手动取消容器 %s 的排队升级任务失败 (任务不存在或已在执行)\n", name)
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
	// 截取最后 400 行以优化前端渲染性能
	start := 0
	if len(allLines) > 400 {
		start = len(allLines) - 400
	}
	lines := allLines[start:]

	// 去除空行
	var result []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			result = append(result, l)
		}
	}

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
		log.Printf("[ERROR] 物理清空系统日志失败: %s\n", err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[SUCCESS] 物理清空系统日志成功\n")
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
		log.Printf("[ERROR] 保存私有仓凭证 %s 失败: %s\n", reg, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[SUCCESS] 保存私有仓凭证 %s 成功 (用户名: %s)\n", reg, body.Username)
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
		log.Printf("[ERROR] 删除私有仓凭证 ID %d 失败: %s\n", id, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[SUCCESS] 删除私有仓凭证 ID %d 成功\n", id)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiContainerStart 启动已停止的容器
func apiContainerStart(c *gin.Context) {
	name := c.Param("name")
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	if err := cli.ContainerStart(c, name, types.ContainerStartOptions{}); err != nil {
		log.Printf("[ERROR] 手动启动容器 %s 失败: %s\n", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[SUCCESS] 手动启动容器 %s 成功\n", name)
	go GlobalHub.BroadcastStatus()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiContainerStop 停止运行中的容器
func apiContainerStop(c *gin.Context) {
	name := c.Param("name")
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	if err := cli.ContainerStop(c, name, container.StopOptions{}); err != nil {
		log.Printf("[ERROR] 手动停止容器 %s 失败: %s\n", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[SUCCESS] 手动停止容器 %s 成功\n", name)
	go GlobalHub.BroadcastStatus()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiContainerRestart 重启容器
func apiContainerRestart(c *gin.Context) {
	name := c.Param("name")
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer cli.Close()

	if err := cli.ContainerRestart(c, name, container.StopOptions{}); err != nil {
		log.Printf("[ERROR] 手动重启容器 %s 失败: %s\n", name, err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[SUCCESS] 手动重启容器 %s 成功\n", name)
	go GlobalHub.BroadcastStatus()
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// apiHistoryClear 清空所有容器升级历史记录及相关的本地部署日志文件
func apiHistoryClear(c *gin.Context) {
	// 1. 清空 update_histories 数据库表
	if err := db.DB.Exec("DELETE FROM update_histories").Error; err != nil {
		log.Printf("[ERROR] 清空升级历史记录失败: %s\n", err.Error())
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

	log.Printf("[SUCCESS] 清空所有升级历史记录成功\n")
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
		log.Printf("[ERROR] 手动删除容器 %s 的升级历史记录失败: %s\n", hist.ContainerName, err.Error())
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

	log.Printf("[SUCCESS] 手动删除容器 %s 的升级历史记录成功\n", hist.ContainerName)
	// 4. 广播给所有的前端客户端更新状态
	go GlobalHub.BroadcastStatus()

	c.JSON(http.StatusOK, gin.H{"ok": true})
}



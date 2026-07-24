package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"
	"docker-updater/service"
	"docker-updater/utils"

	"github.com/robfig/cron/v3"
)

var (
	CronScheduler    *cron.Cron
	schedulerMu      sync.Mutex
	initialCheckOnce sync.Once
)

// StartScheduler 初始化并启动定时任务调度器。
func StartScheduler() {
	schedulerMu.Lock()
	defer schedulerMu.Unlock()

	// 停止现有调度器
	if CronScheduler != nil {
		CronScheduler.Stop()
	}

	checkType := db.GetSetting("check_type", "day")
	checkValueStr := db.GetSetting("check_value", "1")
	checkValue, err := strconv.Atoi(checkValueStr)
	if err != nil || checkValue <= 0 {
		checkValue = 1
	}

	cronExpr := "0 3 */1 * *" // 默认基准策略：每日 03:00 执行
	switch checkType {
	case "hour":
		if checkValue > 23 {
			days := checkValue / 24
			if days <= 0 {
				days = 1
			}
			cronExpr = fmt.Sprintf("0 3 */%d * *", days)
		} else {
			cronExpr = fmt.Sprintf("0 */%d * * *", checkValue)
		}
	case "day":
		cronExpr = fmt.Sprintf("0 3 */%d * *", checkValue)
	case "week":
		days := checkValue * 7
		cronExpr = fmt.Sprintf("0 3 */%d * *", days)
	case "month":
		cronExpr = fmt.Sprintf("0 3 1 */%d *", checkValue)
	default:
		// 异常策略回退：默认解析为每日 03:00
		cronExpr = "0 3 * * *"
	}

	CronScheduler = cron.New(cron.WithLocation(time.Local))

	// 注册容器镜像更新检测定时任务
	_, err = CronScheduler.AddFunc(cronExpr, func() {
		RunScheduledCheck(false)
	})
	if err != nil {
		utils.LogError("注册镜像检测任务失败: %s", err.Error())
	}

	// 注册备份容器及约束记录清理任务(周期: 每小时)
	_, err = CronScheduler.AddFunc("0 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// 1. 清理物理过期备份容器及其元数据
		var expired []db.RollbackMetadata
		nowStr := time.Now().UTC().Format(time.RFC3339)
		if dbErr := db.DB.Find(&expired, "expires_at != '' AND expires_at != 'forever' AND expires_at <= ?", nowStr).Error; dbErr == nil && len(expired) > 0 {
			var backupNames []string
			for _, meta := range expired {
				backupNames = append(backupNames, meta.ContainerName+"_backup_docker_updater")
			}
			dockerclient.CleanExpiredBackups(ctx, backupNames)
			db.DB.Delete(&expired)
		}

		// 2. 清理失效的延迟更新保护配置
		today := time.Now().Format("2006-01-02")
		db.DB.Where("until <= ?", today).Delete(&db.DeferredUpdate{})

		// 3. 校验容器状态，清理无效备份元数据
		var rollbacks []db.RollbackMetadata
		if err := db.DB.Find(&rollbacks).Error; err == nil {
			cli, cliErr := dockerclient.NewLocalClient()
			if cliErr == nil {
				defer cli.Close()
				for _, rb := range rollbacks {
					backupName := rb.ContainerName + "_backup_docker_updater"
					if _, inspectErr := cli.ContainerInspect(ctx, backupName); inspectErr != nil {
						db.DB.Delete(&rb)
					}
				}
			}
		}
	})
	if err != nil {
		utils.LogError("注册备份清理任务失败: %s", err.Error())
	}

	CronScheduler.Start()
	utils.LogInfo("定时任务加载完毕，定时规则: %s", cronExpr)

	// 启动初始化触发器: 延迟 3 秒完成组件加载后执行首轮检测
	initialCheckOnce.Do(func() {
		go func() {
			time.Sleep(3 * time.Second)
			RunScheduledCheck(true)
		}()
	})
}

// RunScheduledCheck 执行镜像更新扫描及下游业务调度(isInitial: 是否为服务初始化扫描)
func RunScheduledCheck(isInitial bool) {
	if !service.TryStartScan() {
		if isInitial {
			utils.LogInfo("首次启动更新检测已触发，但后台已有扫描任务正在运行，本次扫描略过...")
		} else {
			utils.LogInfo("定时更新检测已触发，但检测到后台已有扫描任务正在运行，本次定时任务略过...")
		}
		return
	}
	defer service.EndScan()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	if isInitial {
		utils.LogInfo("启动服务初始化镜像更新检查...")
	} else {
		utils.LogInfo("启动定时镜像更新检查...")
	}

	// 获取扫描前待更新记录基数
	var countBefore int64
	db.DB.Model(&db.AvailableUpdate{}).Count(&countBefore)

	results, err := dockerclient.ScanLocalHostForUpdates(ctx)
	if err != nil {
		utils.LogError("镜像更新检查扫描失败: %s", err.Error())
		return
	}

	for _, res := range results {
		if res.HasUpdate {
			updateEntry := db.AvailableUpdate{
				ContainerName:  res.ContainerName,
				Image:          res.Image,
				LocalDigest:    res.LocalDigest,
				RemoteDigest:   res.RemoteDigest,
				LocalVersion:   res.LocalVersion,
				RemoteVersion:  res.RemoteVersion,
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

	var countAfter int64
	db.DB.Model(&db.AvailableUpdate{}).Where("local_digest != remote_digest").Count(&countAfter)

	if countAfter > 0 && countAfter > countBefore {
		// 汇总待更新容器特征明细
		var list []db.AvailableUpdate
		db.DB.Find(&list)
		var names []string
		var details []string
		for _, item := range list {
			names = append(names, item.ContainerName)
			locVerPart := ""
			if item.LocalVersion != "" {
				locVerPart = "version:" + item.LocalVersion + " "
			}
			remVerPart := ""
			if item.RemoteVersion != "" {
				remVerPart = "version:" + item.RemoteVersion + " "
			}
			locHash := strings.TrimPrefix(item.LocalDigest, "sha256:")
			remHash := strings.TrimPrefix(item.RemoteDigest, "sha256:")
			if len(locHash) > 12 {
				locHash = locHash[:12]
			}
			if len(remHash) > 12 {
				remHash = remHash[:12]
			}
			details = append(details, fmt.Sprintf("- %s (本地: %ssha256:%s, 远端: %ssha256:%s)", item.ContainerName, locVerPart, locHash, remVerPart, remHash))
		}
		utils.LogInfo("检测到有 %d 个服务需要升级更新: %s", len(names), strings.Join(names, ", "))

		// 触发更新预警通知(仅在启用通知且禁用自动更新时生效)
		notifyEnabled := db.GetSetting("notify_enabled", "")
		if notifyEnabled == "" {
			notifyEnabled = db.GetSetting("smtp_enabled", "false")
		}
		if notifyEnabled == "true" && db.GetSetting("auto_update_enabled", "false") != "true" {
			go func(namesList []string, detailList []string) {
				detailText := strings.Join(detailList, "\n")
				containerLabel := ""
				if len(namesList) == 1 {
					containerLabel = namesList[0]
				} else {
					containerLabel = fmt.Sprintf("多个容器 (%d个)", len(namesList))
				}
				service.SendNotification(
					containerLabel,
					service.NotifyActionUpdateCheck,
					"发现新版本",
					detailText,
				)
			}(names, details)
		}
	}

	// 入队自动更新任务(过滤延迟保护期容器)
	if db.GetSetting("auto_update_enabled", "false") == "true" {
		var list []db.AvailableUpdate
		db.DB.Find(&list)

		var deferred []db.DeferredUpdate
		db.DB.Find(&deferred)
		deferMap := make(map[string]bool)
		today := time.Now().Format("2006-01-02")
		for _, d := range deferred {
			if d.Until == "forever" || d.Until > today {
				deferMap[d.ContainerName] = true
			}
		}

		for _, item := range list {
			if item.LocalDigest != item.RemoteDigest && !deferMap[item.ContainerName] {
				utils.LogInfo("自动更新: 将容器 %s 加入升级队列...", item.ContainerName)
				service.GlobalQueue.AddTask(item.ContainerName, service.TaskUpdate, "", true)
			}
		}
	}
}

// ReloadScheduler 热重载定时任务调度器。
func ReloadScheduler() {
	StartScheduler()
}

func init() {
	service.OnSettingsReload = ReloadScheduler
}

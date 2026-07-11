package scheduler

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"
	"docker-updater/service"
	"docker-updater/utils"

	"github.com/robfig/cron/v3"
)

var CronScheduler *cron.Cron

// StartScheduler 启动定时任务调度
func StartScheduler() {
	checkType := db.GetSetting("check_type", "day")
	checkValueStr := db.GetSetting("check_value", "1")
	checkValue, err := strconv.Atoi(checkValueStr)
	if err != nil || checkValue <= 0 {
		checkValue = 1
	}

	cronExpr := "0 3 */1 * *" // 默认每日凌晨 3 点执行
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
		// 非标准配置下默认每日凌晨 3 点执行
		cronExpr = "0 3 * * *"
	}

	CronScheduler = cron.New(cron.WithLocation(time.Local))

	// 注册定时镜像更新检测任务
	_, err = CronScheduler.AddFunc(cronExpr, func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		utils.LogInfo("启动定时镜像更新检查...")

		// 获取更新前的待更新记录数
		var countBefore int64
		db.DB.Model(&db.AvailableUpdate{}).Count(&countBefore)

		results, err := dockerclient.ScanLocalHostForUpdates(ctx)
		if err != nil {
			utils.LogError("定时镜像更新检查扫描失败: %s", err.Error())
		} else {
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

			var countAfter int64
			db.DB.Model(&db.AvailableUpdate{}).Where("local_digest != remote_digest").Count(&countAfter)

			if countAfter > 0 && countAfter > countBefore {
				// 获取变动列表
				var list []db.AvailableUpdate
				db.DB.Find(&list)
				var names []string
				var details []string
				for _, item := range list {
					names = append(names, item.ContainerName)
					details = append(details, fmt.Sprintf("- %s (远程最新镜像 Digest: %s)", item.ContainerName, item.RemoteDigest))
				}
				utils.LogInfo("检测到有 %d 个服务需要升级更新: %s", len(names), strings.Join(names, ", "))

				// 若启用通知且未开启自动更新，则发送版本更新预警报告。
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

			// 执行定时自动更新任务
			if db.GetSetting("auto_update_enabled", "false") == "true" {
				var list []db.AvailableUpdate
				db.DB.Find(&list)

				// 获取所有设置了延迟更新的容器列表以进行过滤
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
						utils.LogInfo("定时自动更新: 将容器 %s 加入升级队列...", item.ContainerName)
						service.GlobalQueue.AddTask(item.ContainerName, service.TaskUpdate, "", true)
					}
				}
			}
		}
	})
	if err != nil {
		utils.LogError("注册镜像检测任务失败: %s", err.Error())
	}

	// 注册过期备份的定时清理任务（周期为每小时一次）。
	_, err = CronScheduler.AddFunc("0 * * * *", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		// 1. 查询已过期的备份元数据并物理清除对应的备份容器。
		var expired []db.RollbackMetadata
		nowStr := time.Now().UTC().Format(time.RFC3339)
		if dbErr := db.DB.Find(&expired, "expires_at <= ?", nowStr).Error; dbErr == nil && len(expired) > 0 {
			var backupNames []string
			for _, meta := range expired {
				backupNames = append(backupNames, meta.ContainerName+"_backup_docker_updater")
			}
			dockerclient.CleanExpiredBackups(ctx, backupNames)
			db.DB.Delete(&expired)
		}

		// 2. 清理过期的延迟更新约束记录。
		today := time.Now().Format("2006-01-02")
		db.DB.Where("until <= ?", today).Delete(&db.DeferredUpdate{})

		// 3. 校验物理容器状态，清理数据库中失效的备份元数据记录。
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
}

// ReloadScheduler 重新加载定时任务调度器 (在配置更新后被自动触发热重载)
func ReloadScheduler() {
	if CronScheduler != nil {
		CronScheduler.Stop()
	}
	StartScheduler()
}

func init() {
	service.OnSettingsReload = ReloadScheduler
}

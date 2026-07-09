package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"

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

	cronExpr := "0 3 */1 * *" // 默认每天凌晨3点
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
		// 兼容老的环境变量配置
		checkTime := os.Getenv("CHECK_TIME")
		if checkTime == "" {
			checkTime = "03:00"
		}
		parts := strings.Split(checkTime, ":")
		if len(parts) != 2 {
			parts = []string{"03", "00"}
		}
		cronExpr = fmt.Sprintf("%s %s * * *", parts[1], parts[0])
	}

	CronScheduler = cron.New(cron.WithLocation(time.Local))

	// 注册定时镜像检测
	_, err = CronScheduler.AddFunc(cronExpr, func() {
		ctx := context.Background()
		log.Println("[INFO] 启动定时镜像更新检查...")

		// 记录旧可用更新数量
		var countBefore int64
		db.DB.Model(&db.AvailableUpdate{}).Count(&countBefore)

		if err := dockerclient.ScanLocalHostForUpdates(ctx); err == nil {
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
				log.Printf("[INFO] 检测到有 %d 个服务需要升级更新: %s\n", len(names), strings.Join(names, ", "))

				// 如果开启了邮件通知，但未开启自动更新，此时发送可用更新预警报告
				if db.GetSetting("smtp_enabled", "false") == "true" && db.GetSetting("auto_update_enabled", "false") != "true" {
					go func(namesList []string, detailList []string) {
						subject := fmt.Sprintf("[Docker Updater] 检测到 %d 个服务有可用新镜像", len(namesList))
						body := fmt.Sprintf("项目名：Docker Updater\n通知类型：可用版本更新预警 (手动升级模式)\n通知时间：%s\n\n检测到以下容器有最新镜像，请登录管理后台手动执行修改版本升级：\n----------------------------------------\n%s\n----------------------------------------",
							time.Now().Local().Format("2006-01-02 15:04:05"), strings.Join(detailList, "\n"))
						_ = dockerclient.SendNotificationEmail(subject, body)
					}(names, details)
				}
			}

			// 定时自动更新逻辑
			if db.GetSetting("auto_update_enabled", "false") == "true" {
				var list []db.AvailableUpdate
				db.DB.Find(&list)

				// 获取所有暂挂的容器以进行过滤
				var deferred []db.DeferredUpdate
				db.DB.Find(&deferred)
				deferMap := make(map[string]bool)
				today := time.Now().Format("2006-01-02")
				for _, d := range deferred {
					if d.Until > today {
						deferMap[d.ContainerName] = true
					}
				}

				for _, item := range list {
					if item.LocalDigest != item.RemoteDigest && !deferMap[item.ContainerName] {
						log.Printf("[INFO] 定时自动更新: 将容器 %s 加入升级队列...\n", item.ContainerName)
						dockerclient.GlobalQueue.AddTask(item.ContainerName, dockerclient.TaskUpdate, "", true)
					}
				}
			}
		}
	})
	if err != nil {
		log.Printf("[ERROR] 注册镜像检测任务失败: %s\n", err.Error())
	}

	// 注册定时清理过期备份 (每小时执行一次)
	_, err = CronScheduler.AddFunc("0 * * * *", func() {
		dockerclient.CleanExpiredBackups(context.Background())
	})
	if err != nil {
		log.Printf("[ERROR] 注册备份清理任务失败: %s\n", err.Error())
	}

	CronScheduler.Start()
	log.Printf("[INFO] 定时任务加载完毕，定时规则: %s\n", cronExpr)
}

// ReloadScheduler 重新加载定时任务调度器 (在配置更新后被自动触发热重载)
func ReloadScheduler() {
	if CronScheduler != nil {
		CronScheduler.Stop()
	}
	StartScheduler()
}

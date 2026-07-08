package scheduler

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"

	"github.com/robfig/cron/v3"
)

var CronScheduler *cron.Cron

// StartScheduler 启动定时任务调度
func StartScheduler() {
	checkTime := os.Getenv("CHECK_TIME")
	if checkTime == "" {
		checkTime = "03:00" // 默认凌晨3点检测
	}

	parts := strings.Split(checkTime, ":")
	if len(parts) != 2 {
		parts = []string{"03", "00"}
	}
	cronExpr := fmt.Sprintf("%s %s * * *", parts[1], parts[0]) // 分 时 * * *

	CronScheduler = cron.New(cron.WithLocation(time.Local))

	// 注册定时镜像检测
	_, err := CronScheduler.AddFunc(cronExpr, func() {
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
				for _, item := range list {
					names = append(names, item.ContainerName)
				}
				log.Printf("[INFO] 检测到有 %d 个服务需要升级更新: %s\n", len(names), strings.Join(names, ", "))
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

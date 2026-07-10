package service

import (
	"strings"
	"time"

	"docker-updater/db"
	"docker-updater/utils"
)

// SendNotification 发送统一通知报告（邮件或 Webhook）
func SendNotification(containerName, actionType, statusName, logContent string) {
	// 兼容原 smtp_enabled
	enabled := db.GetSetting("notify_enabled", "")
	if enabled == "" {
		enabled = db.GetSetting("smtp_enabled", "false")
	}
	if enabled != "true" {
		return
	}

	notifyType := db.GetSetting("notify_type", "email")
	switch notifyType {
	case "email":
		// 邮件逻辑
		subjectTpl := db.GetSetting("smtp_subject_template", utils.DefaultSMTPSubject)
		bodyTpl := db.GetSetting("smtp_body_template", utils.DefaultSMTPBody)

		r := strings.NewReplacer(
			"{container_name}", containerName,
			"{action_type}", actionType,
			"{status}", statusName,
			"{time}", time.Now().Local().Format("2006-01-02 15:04:05"),
			"{logs}", logContent,
		)

		subject := r.Replace(subjectTpl)
		body := r.Replace(bodyTpl)

		cfg := utils.SMTPConfig{
			Host:     db.GetSetting("smtp_host", ""),
			Port:     db.GetSetting("smtp_port", "465"),
			Username: db.GetSetting("smtp_username", ""),
			Password: db.GetSetting("smtp_password", ""),
			SSL:      db.GetSetting("smtp_ssl", "true") == "true",
			To:       db.GetSetting("smtp_to", ""),
		}
		go func() {
			_ = utils.SendNotificationEmail(cfg, subject, body)
		}()
	case "webhook":
		// Webhook 逻辑
		url := db.GetSetting("webhook_url", "")
		method := db.GetSetting("webhook_method", "POST")
		template := db.GetSetting("webhook_template", utils.DefaultWebhookTemplate)

		// 规整 logs：如果是 JSON Webhook，日志中的换行符 \n 需要转义，否则会破坏 JSON 格式
		escapedLogs := logContent
		escapedLogs = strings.ReplaceAll(escapedLogs, `\`, `\\`)
		escapedLogs = strings.ReplaceAll(escapedLogs, `"`, `\"`)
		escapedLogs = strings.ReplaceAll(escapedLogs, "\n", `\n`)
		escapedLogs = strings.ReplaceAll(escapedLogs, "\r", `\r`)
		escapedLogs = strings.ReplaceAll(escapedLogs, "\t", `\t`)

		r := strings.NewReplacer(
			"{container_name}", containerName,
			"{action_type}", actionType,
			"{status}", statusName,
			"{time}", time.Now().Local().Format("2006-01-02 15:04:05"),
			"{logs}", escapedLogs,
		)
		payload := r.Replace(template)

		go func() {
			_ = utils.SendWebhookNotification(url, method, payload)
		}()
	}
}

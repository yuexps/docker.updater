package service

import (
	"strings"
	"time"

	"docker-updater/db"
	"docker-updater/utils"
)

const (
	NotifyActionUpdateCheck = "可用版本更新提醒"
	NotifyActionUpdate      = "容器升级"
	NotifyActionRollback    = "回滚恢复"
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
	isCheck := actionType == NotifyActionUpdateCheck

	switch notifyType {
	case "email":
		// 邮件逻辑
		var subjectTpl, bodyTpl string
		if isCheck {
			subjectTpl = db.GetSetting("smtp_subject_template_check", utils.DefaultSMTPSubjectCheck)
			bodyTpl = db.GetSetting("smtp_body_template_check", utils.DefaultSMTPBodyCheck)
		} else {
			subjectTpl = db.GetSetting("smtp_subject_template", utils.DefaultSMTPSubject)
			bodyTpl = db.GetSetting("smtp_body_template", utils.DefaultSMTPBody)
		}

		subject := renderTemplate(subjectTpl, containerName, actionType, statusName, logContent, false)
		body := renderTemplate(bodyTpl, containerName, actionType, statusName, logContent, false)

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
		webhookType := db.GetSetting("webhook_type", "custom")
		if webhookType == "" {
			webhookType = "custom"
		}

		var template string
		if isCheck {
			template = db.GetSetting("webhook_template_check_"+webhookType, "")
			if template == "" {
				template = utils.GetDefaultWebhookTemplate(webhookType, true)
			}
		} else {
			template = db.GetSetting("webhook_template_"+webhookType, "")
			if template == "" {
				template = utils.GetDefaultWebhookTemplate(webhookType, false)
			}
		}

		payload := renderTemplate(template, containerName, actionType, statusName, logContent, true)

		go func() {
			_, _ = utils.SendWebhookNotification(url, method, payload, webhookType)
		}()
	}
}

// escapeJSONString 转义 JSON 中的特殊字符与换行符。
func escapeJSONString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	return s
}

// renderTemplate 渲染通知模板。
func renderTemplate(tpl, containerName, actionType, statusName, logContent string, escapeJSON bool) string {
	logs := logContent
	if escapeJSON {
		logs = escapeJSONString(logs)
	}
	r := strings.NewReplacer(
		"{container_name}", containerName,
		"{action_type}", actionType,
		"{status}", statusName,
		"{time}", time.Now().Local().Format("2006-01-02 15:04:05"),
		"{logs}", logs,
	)
	return r.Replace(tpl)
}


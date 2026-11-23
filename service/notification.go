package service

import (
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"docker-updater/db"
	"docker-updater/utils"
)

const (
	NotifyActionUpdateCheck  = "可用版本更新提醒"
	NotifyActionUpdate       = "容器升级"
	maxWebhookLogBytes       = 2800
	webhookLogTruncateSuffix = "\n...(日志已截断，完整内容请在管理面板查看)"
)

// notifyDispatcher 通知发送函数变量，便于单元测试拦截
var notifyDispatcher = SendNotification

// SendNotification 发送统一通知报告（邮件或 Webhook）
func SendNotification(containerName, actionType, statusName, logContent string) {
	if db.GetSetting("notify_enabled", "false") != "true" {
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

// escapeJSONString 转义 JSON 中的特殊字符、换行符与控制字符
func escapeJSONString(s string) string {
	var sb strings.Builder
	sb.Grow(len(s))
	for _, r := range s {
		switch r {
		case '\\':
			sb.WriteString(`\\`)
		case '"':
			sb.WriteString(`\"`)
		case '\n':
			sb.WriteString(`\n`)
		case '\r':
			sb.WriteString(`\r`)
		case '\t':
			sb.WriteString(`\t`)
		case '\b':
			sb.WriteString(`\b`)
		case '\f':
			sb.WriteString(`\f`)
		default:
			if r < 0x20 {
				sb.WriteString(fmt.Sprintf(`\u%04x`, r))
			} else {
				sb.WriteRune(r)
			}
		}
	}
	return sb.String()
}

// truncateUTF8 按最大字节数安全截断 UTF-8 字符串并附加提示后缀
func truncateUTF8(s string, maxBytes int, suffix string) string {
	if maxBytes <= 0 || len(s) <= maxBytes {
		return s
	}
	limit := maxBytes - len(suffix)
	if limit <= 0 {
		limit = maxBytes
		suffix = ""
	}
	cut := 0
	for idx, r := range s {
		runeLen := utf8.RuneLen(r)
		if runeLen < 0 {
			runeLen = 1
		}
		if idx+runeLen > limit {
			break
		}
		cut = idx + runeLen
	}
	return s[:cut] + suffix
}

// renderTemplate 渲染通知模板
func renderTemplate(tpl, containerName, actionType, statusName, logContent string, escapeJSON bool) string {
	logs := logContent
	if escapeJSON {
		logs = truncateUTF8(logs, maxWebhookLogBytes, webhookLogTruncateSuffix)
		containerName = escapeJSONString(containerName)
		actionType = escapeJSONString(actionType)
		statusName = escapeJSONString(statusName)
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


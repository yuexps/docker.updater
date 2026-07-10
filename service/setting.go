package service

import (
	"encoding/json"
	"strconv"
	"strings"
	"time"

	"docker-updater/db"
	"docker-updater/utils"
)

// OnSettingsReload 配置重载回调函数。
var OnSettingsReload func()

// GlobalSettings 全局配置结构体。
type GlobalSettings struct {
	BackupEnabled            bool     `json:"backup_enabled"`
	BackupHours              int      `json:"backup_hours"`
	RestartStack             bool     `json:"restart_stack"`
	AutoUpdateEnabled        bool     `json:"auto_update_enabled"`
	TempMirrors              []string `json:"temp_mirrors"`
	CheckType                string   `json:"check_type"`
	CheckValue               int      `json:"check_value"`
	NotifyEnabled            bool     `json:"notify_enabled"`
	NotifyType               string   `json:"notify_type"`
	SMTPEnabled              bool     `json:"smtp_enabled"` // 保持兼容
	SMTPHost                 string   `json:"smtp_host"`
	SMTPPort                 string   `json:"smtp_port"`
	SMTPUsername             string   `json:"smtp_username"`
	SMTPPassword             string   `json:"smtp_password"`
	SMTPSSL                  bool     `json:"smtp_ssl"`
	SMTPTo                   string   `json:"smtp_to"`
	SMTPSubjectTemplate      string   `json:"smtp_subject_template"`
	SMTPBodyTemplate         string   `json:"smtp_body_template"`
	SMTPSubjectTemplateCheck string   `json:"smtp_subject_template_check"`
	SMTPBodyTemplateCheck    string   `json:"smtp_body_template_check"`
	WebhookURL               string   `json:"webhook_url"`
	WebhookMethod            string   `json:"webhook_method"`
	WebhookTemplate          string   `json:"webhook_template"`
	WebhookTemplateCheck     string   `json:"webhook_template_check"`
}

// TestNotificationSettings 通知配置测试结构体。
type TestNotificationSettings struct {
	NotifyType          string `json:"notify_type"`
	SMTPHost            string `json:"smtp_host"`
	SMTPPort            string `json:"smtp_port"`
	SMTPUsername        string `json:"smtp_username"`
	SMTPPassword        string `json:"smtp_password"`
	SMTPSSL             bool   `json:"smtp_ssl"`
	SMTPTo              string `json:"smtp_to"`
	SMTPSubjectTemplate string `json:"smtp_subject_template"`
	SMTPBodyTemplate    string `json:"smtp_body_template"`
	WebhookURL          string `json:"webhook_url"`
	WebhookMethod       string `json:"webhook_method"`
	WebhookTemplate     string `json:"webhook_template"`
}

// SaveGlobalSettings 保存全局配置。
func SaveGlobalSettings(body GlobalSettings) error {
	_ = db.SetSetting("backup_enabled", strconv.FormatBool(body.BackupEnabled))
	_ = db.SetSetting("backup_hours", strconv.Itoa(body.BackupHours))
	_ = db.SetSetting("restart_stack", strconv.FormatBool(body.RestartStack))
	_ = db.SetSetting("auto_update_enabled", strconv.FormatBool(body.AutoUpdateEnabled))
	_ = db.SetSetting("notify_enabled", strconv.FormatBool(body.NotifyEnabled))
	_ = db.SetSetting("notify_type", body.NotifyType)
	_ = db.SetSetting("smtp_enabled", strconv.FormatBool(body.NotifyEnabled)) // 兼容原 smtp_enabled
	_ = db.SetSetting("smtp_host", body.SMTPHost)
	_ = db.SetSetting("smtp_port", body.SMTPPort)
	_ = db.SetSetting("smtp_username", body.SMTPUsername)
	if body.SMTPPassword != "******" {
		_ = db.SetSetting("smtp_password", body.SMTPPassword)
	}
	_ = db.SetSetting("smtp_ssl", strconv.FormatBool(body.SMTPSSL))
	_ = db.SetSetting("smtp_to", body.SMTPTo)
	_ = db.SetSetting("smtp_subject_template", body.SMTPSubjectTemplate)
	_ = db.SetSetting("smtp_body_template", body.SMTPBodyTemplate)
	_ = db.SetSetting("smtp_subject_template_check", body.SMTPSubjectTemplateCheck)
	_ = db.SetSetting("smtp_body_template_check", body.SMTPBodyTemplateCheck)
	_ = db.SetSetting("webhook_url", body.WebhookURL)
	_ = db.SetSetting("webhook_method", body.WebhookMethod)
	_ = db.SetSetting("webhook_template", body.WebhookTemplate)
	_ = db.SetSetting("webhook_template_check", body.WebhookTemplateCheck)

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

	utils.LogSuccess("保存全局配置成功")

	if OnSettingsReload != nil {
		OnSettingsReload()
	}
	return nil
}

// SendTestNotification 发送测试通知。
func SendTestNotification(body TestNotificationSettings) error {
	if body.NotifyType == "webhook" {
		url := body.WebhookURL
		method := body.WebhookMethod
		template := body.WebhookTemplate
		if template == "" {
			template = utils.DefaultWebhookTemplate
		}

		rawLogs := "[PULL] Pulling image mysql:8.0\n[INFO] Stopping old container\n[INFO] Starting new container\n[SUCCESS] Container updated successfully"
		// 规整 logs：如果是 JSON Webhook，日志中的换行符 \n 需要转义，否则会破坏 JSON 格式
		escapedLogs := rawLogs
		escapedLogs = strings.ReplaceAll(escapedLogs, `\`, `\\`)
		escapedLogs = strings.ReplaceAll(escapedLogs, `"`, `\"`)
		escapedLogs = strings.ReplaceAll(escapedLogs, "\n", `\n`)
		escapedLogs = strings.ReplaceAll(escapedLogs, "\r", `\r`)
		escapedLogs = strings.ReplaceAll(escapedLogs, "\t", `\t`)

		r := strings.NewReplacer(
			"{container_name}", "test-mysql",
			"{action_type}", "测试通知",
			"{status}", "测试成功",
			"{time}", time.Now().Local().Format("2006-01-02 15:04:05"),
			"{logs}", escapedLogs,
		)
		payload := r.Replace(template)
		return utils.SendWebhookNotification(url, method, payload)
	}

	subjectTpl := body.SMTPSubjectTemplate
	if subjectTpl == "" {
		subjectTpl = utils.DefaultSMTPSubject
	}
	bodyTpl := body.SMTPBodyTemplate
	if bodyTpl == "" {
		bodyTpl = utils.DefaultSMTPBody
	}

	r := strings.NewReplacer(
		"{container_name}", "test-mysql",
		"{action_type}", "测试通知",
		"{status}", "测试成功",
		"{time}", time.Now().Local().Format("2006-01-02 15:04:05"),
		"{logs}", "[PULL] Pulling image mysql:8.0\n[INFO] Stopping old container\n[INFO] Starting new container\n[SUCCESS] Container updated successfully",
	)

	subject := r.Replace(subjectTpl)
	bodyText := r.Replace(bodyTpl)

	return utils.SendEmailRaw(
		body.SMTPHost,
		body.SMTPPort,
		body.SMTPUsername,
		body.SMTPPassword,
		body.SMTPTo,
		body.SMTPSSL,
		subject,
		bodyText,
	)
}

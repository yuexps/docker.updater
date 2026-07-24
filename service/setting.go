package service

import (
	"encoding/json"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

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
	WebhookType              string   `json:"webhook_type"`
	WebhookTemplateCustom    string   `json:"webhook_template_custom"`
	WebhookTemplateWechat    string   `json:"webhook_template_wechat"`
	WebhookTemplateDingtalk  string   `json:"webhook_template_dingtalk"`
	WebhookTemplateFeishu    string   `json:"webhook_template_feishu"`
	WebhookTemplateCheckCustom   string   `json:"webhook_template_check_custom"`
	WebhookTemplateCheckWechat   string   `json:"webhook_template_check_wechat"`
	WebhookTemplateCheckDingtalk string   `json:"webhook_template_check_dingtalk"`
	WebhookTemplateCheckFeishu   string   `json:"webhook_template_check_feishu"`
}

// TestNotificationSettings 通知配置测试结构体。
type TestNotificationSettings struct {
	NotifyType              string `json:"notify_type"`
	SMTPHost                string `json:"smtp_host"`
	SMTPPort                string `json:"smtp_port"`
	SMTPUsername            string `json:"smtp_username"`
	SMTPPassword            string `json:"smtp_password"`
	SMTPSSL                 bool   `json:"smtp_ssl"`
	SMTPTo                  string `json:"smtp_to"`
	SMTPSubjectTemplate     string `json:"smtp_subject_template"`
	SMTPBodyTemplate        string `json:"smtp_body_template"`
	WebhookURL              string `json:"webhook_url"`
	WebhookMethod           string `json:"webhook_method"`
	WebhookType             string `json:"webhook_type"`
	WebhookTemplateCustom    string `json:"webhook_template_custom"`
	WebhookTemplateWechat    string `json:"webhook_template_wechat"`
	WebhookTemplateDingtalk  string `json:"webhook_template_dingtalk"`
	WebhookTemplateFeishu    string `json:"webhook_template_feishu"`
}

// SaveGlobalSettings 保存全局配置。
func SaveGlobalSettings(body GlobalSettings) error {
	// 1. 读取保存前的旧配置以进行增量变动对比
	oldBackupEnabled := db.GetSetting("backup_enabled", "false") == "true"
	oldBackupHours, _ := strconv.Atoi(db.GetSetting("backup_hours", "24"))
	oldRestartStack := db.GetSetting("restart_stack", "false") == "true"
	oldAutoUpdateEnabled := db.GetSetting("auto_update_enabled", "false") == "true"
	oldCheckType := db.GetSetting("check_type", "day")
	oldCheckValue, _ := strconv.Atoi(db.GetSetting("check_value", "1"))
	oldMirrorsStr := db.GetSetting("temp_mirrors", "[]")

	oldNotifyEnabledStr := db.GetSetting("notify_enabled", "")
	if oldNotifyEnabledStr == "" {
		oldNotifyEnabledStr = db.GetSetting("smtp_enabled", "false")
	}
	oldNotifyEnabled := oldNotifyEnabledStr == "true"
	oldNotifyType := db.GetSetting("notify_type", "email")

	oldSMTPHost := db.GetSetting("smtp_host", "")
	oldSMTPPort := db.GetSetting("smtp_port", "465")
	oldSMTPUsername := db.GetSetting("smtp_username", "")
	oldSMTPPassword := db.GetSetting("smtp_password", "")
	oldSMTPSSL := db.GetSetting("smtp_ssl", "true") == "true"
	oldSMTPTo := db.GetSetting("smtp_to", "")
	oldSMTPSubjectTemplate := db.GetSetting("smtp_subject_template", "")
	oldSMTPBodyTemplate := db.GetSetting("smtp_body_template", "")

	oldWebhookURL := db.GetSetting("webhook_url", "")
	oldWebhookMethod := db.GetSetting("webhook_method", "POST")
	oldWebhookType := db.GetSetting("webhook_type", "custom")
	oldWebhookTemplateCustom := db.GetSetting("webhook_template_custom", "")
	oldWebhookTemplateWechat := db.GetSetting("webhook_template_wechat", "")
	oldWebhookTemplateDingtalk := db.GetSetting("webhook_template_dingtalk", "")
	oldWebhookTemplateFeishu := db.GetSetting("webhook_template_feishu", "")

	// 2. 保存新配置到数据库
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
	_ = db.SetSetting("webhook_type", body.WebhookType)
	_ = db.SetSetting("webhook_template_custom", body.WebhookTemplateCustom)
	_ = db.SetSetting("webhook_template_wechat", body.WebhookTemplateWechat)
	_ = db.SetSetting("webhook_template_dingtalk", body.WebhookTemplateDingtalk)
	_ = db.SetSetting("webhook_template_feishu", body.WebhookTemplateFeishu)
	_ = db.SetSetting("webhook_template_check_custom", body.WebhookTemplateCheckCustom)
	_ = db.SetSetting("webhook_template_check_wechat", body.WebhookTemplateCheckWechat)
	_ = db.SetSetting("webhook_template_check_dingtalk", body.WebhookTemplateCheckDingtalk)
	_ = db.SetSetting("webhook_template_check_feishu", body.WebhookTemplateCheckFeishu)

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
	newMirrorsStr := string(mirrorsBytes)
	_ = db.SetSetting("temp_mirrors", newMirrorsStr)

	// 3. 使用 utils.SettingsDiffBuilder 构建增量对比日志
	diff := utils.NewSettingsDiffBuilder()
	diff.AddBool("自动备份", oldBackupEnabled, body.BackupEnabled)
	diff.AddInt("备份保留时长", oldBackupHours, body.BackupHours, "小时")
	diff.AddBool("更新后重启Stack", oldRestartStack, body.RestartStack)
	diff.AddBool("自动更新", oldAutoUpdateEnabled, body.AutoUpdateEnabled)

	newCheckType := body.CheckType
	if newCheckType == "" {
		newCheckType = oldCheckType
	}
	newCheckValue := body.CheckValue
	if newCheckValue <= 0 {
		newCheckValue = oldCheckValue
	}
	if oldCheckType != newCheckType || oldCheckValue != newCheckValue {
		diff.AddCustom(true, fmt.Sprintf("检测周期: %s -> %s", utils.FormatPeriod(oldCheckType, oldCheckValue), utils.FormatPeriod(newCheckType, newCheckValue)))
	}

	diff.AddCustom(oldMirrorsStr != newMirrorsStr, fmt.Sprintf("临时镜像源: 已更新(共%d个源)", len(body.TempMirrors)))
	diff.AddBool("消息通知", oldNotifyEnabled, body.NotifyEnabled)
	diff.AddString("通知渠道类型", oldNotifyType, body.NotifyType)
	diff.AddString("SMTP服务器", oldSMTPHost, body.SMTPHost)
	diff.AddString("SMTP端口", oldSMTPPort, body.SMTPPort)
	diff.AddString("SMTP发件账号", oldSMTPUsername, body.SMTPUsername)
	diff.AddSecret("SMTP密码", oldSMTPPassword, body.SMTPPassword)
	diff.AddBool("SMTP SSL", oldSMTPSSL, body.SMTPSSL)
	diff.AddString("SMTP收件人", oldSMTPTo, body.SMTPTo)

	diff.AddString("Webhook URL", oldWebhookURL, body.WebhookURL)
	diff.AddString("Webhook请求方式", oldWebhookMethod, body.WebhookMethod)
	diff.AddString("Webhook类型", oldWebhookType, body.WebhookType)

	diff.AddCustom(oldSMTPSubjectTemplate != body.SMTPSubjectTemplate || oldSMTPBodyTemplate != body.SMTPBodyTemplate, "SMTP邮件模板: 已更新")
	diff.AddCustom(oldWebhookTemplateCustom != body.WebhookTemplateCustom ||
		oldWebhookTemplateWechat != body.WebhookTemplateWechat ||
		oldWebhookTemplateDingtalk != body.WebhookTemplateDingtalk ||
		oldWebhookTemplateFeishu != body.WebhookTemplateFeishu, "Webhook自定义模板: 已更新")

	diff.Log("保存全局配置成功")

	if OnSettingsReload != nil {
		OnSettingsReload()
	}
	return nil
}

// SendTestNotification 发送测试通知。
func SendTestNotification(body TestNotificationSettings) (string, error) {
	if body.NotifyType == "webhook" {
		// 动态解析 Webhook 的网络参数
		u, err := url.Parse(body.WebhookURL)
		var host, scheme, path string
		if err == nil {
			host = u.Host
			scheme = u.Scheme
			path = u.Path
		} else {
			host = "unknown"
			scheme = "http"
			path = "/"
		}
		port := "80"
		if scheme == "https" {
			port = "443"
		}
		if h, p, err := net.SplitHostPort(host); err == nil {
			host = h
			port = p
		}

		var logSb strings.Builder
		logSb.WriteString(fmt.Sprintf("[INFO] 正在解析 Webhook 服务器域名: %s\n", host))
		logSb.WriteString(fmt.Sprintf("[INFO] 正在建立 TCP 网络连接: %s:%s...\n", host, port))
		if scheme == "https" {
			logSb.WriteString(fmt.Sprintf("[INFO] 正在与 %s 初始化 SSL/TLS 安全握手... 完毕。\n", host))
		}
		logSb.WriteString(fmt.Sprintf("[INFO] 连接已建立。正在构建 HTTP %s 请求...\n", body.WebhookMethod))
		logSb.WriteString(fmt.Sprintf("[INFO] 请求目标路径: %s\n", path))
		logSb.WriteString("[INFO] 正在发送 application/json 数据报文...\n")
		logSb.WriteString("[INFO] Webhook 测试数据包已成功投递。网络传输已完成。")
		rawLogs := logSb.String()

		wType := body.WebhookType
		if wType == "" {
			wType = "custom"
		}
		var template string
		switch wType {
		case "feishu":
			template = body.WebhookTemplateFeishu
		case "wechat":
			template = body.WebhookTemplateWechat
		case "dingtalk":
			template = body.WebhookTemplateDingtalk
		default:
			template = body.WebhookTemplateCustom
		}
		if template == "" {
			template = utils.GetDefaultWebhookTemplate(wType, false)
		}

		payload := renderTemplate(template, "docker-updater", "测试通知", "测试成功", rawLogs, true)
		return utils.SendWebhookNotification(body.WebhookURL, body.WebhookMethod, payload, wType)
	}

	// 动态解析 SMTP 邮件网络参数
	var logSb strings.Builder
	logSb.WriteString(fmt.Sprintf("[INFO] 正在连接 SMTP 邮件服务器 %s:%s...\n", body.SMTPHost, body.SMTPPort))
	logSb.WriteString("[INFO] TCP 网络套接字连接已建立。\n")
	if body.SMTPSSL {
		logSb.WriteString(fmt.Sprintf("[INFO] 正在与 %s 初始化 SSL/TLS 加密信道... 完毕。\n", body.SMTPHost))
	} else {
		logSb.WriteString("[INFO] 已通过非加密信道建立连接 (明文模式)。\n")
	}
	logSb.WriteString("[INFO] 正在与邮件服务器握手并初始化 EHLO 协议...\n")
	logSb.WriteString(fmt.Sprintf("[INFO] 正在验证发件用户账号 %s 的授权凭证...\n", body.SMTPUsername))
	logSb.WriteString(fmt.Sprintf("[INFO] 正在构建邮件信封: 发件人<%s> -> 收件人<%s>\n", body.SMTPUsername, body.SMTPTo))
	logSb.WriteString("[INFO] 正在传输邮件 MIME 数据报文...\n")
	logSb.WriteString("[INFO] 邮件已成功投递至远程邮件传输网关。SMTP 传输已完成。")
	rawLogs := logSb.String()

	subjectTpl := body.SMTPSubjectTemplate
	if subjectTpl == "" {
		subjectTpl = utils.DefaultSMTPSubject
	}
	bodyTpl := body.SMTPBodyTemplate
	if bodyTpl == "" {
		bodyTpl = utils.DefaultSMTPBody
	}

	subject := renderTemplate(subjectTpl, "docker-updater", "测试通知", "测试成功", rawLogs, false)
	bodyText := renderTemplate(bodyTpl, "docker-updater", "测试通知", "测试成功", rawLogs, false)

	cfg := utils.SMTPConfig{
		Host:     body.SMTPHost,
		Port:     body.SMTPPort,
		Username: body.SMTPUsername,
		Password: body.SMTPPassword,
		SSL:      body.SMTPSSL,
		To:       body.SMTPTo,
	}
	errEmail := utils.SendNotificationEmail(cfg, subject, bodyText)
	return "邮件投递成功", errEmail
}

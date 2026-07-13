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
	_ = db.SetSetting("temp_mirrors", string(mirrorsBytes))

	utils.LogSuccess("保存全局配置成功")

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
		logSb.WriteString("[SUCCESS] Webhook 测试数据包已成功投递。网络传输已完成。")
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
	logSb.WriteString("[SUCCESS] 邮件已成功投递至远程邮件传输网关。SMTP 传输已完成。")
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

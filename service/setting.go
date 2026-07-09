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
	BackupEnabled       bool     `json:"backup_enabled"`
	BackupHours         int      `json:"backup_hours"`
	RestartStack        bool     `json:"restart_stack"`
	AutoUpdateEnabled   bool     `json:"auto_update_enabled"`
	TempMirrors         []string `json:"temp_mirrors"`
	CheckType           string   `json:"check_type"`
	CheckValue          int      `json:"check_value"`
	SMTPEnabled         bool     `json:"smtp_enabled"`
	SMTPHost            string   `json:"smtp_host"`
	SMTPPort            string   `json:"smtp_port"`
	SMTPUsername        string   `json:"smtp_username"`
	SMTPPassword        string   `json:"smtp_password"`
	SMTPSSL             bool     `json:"smtp_ssl"`
	SMTPTo              string   `json:"smtp_to"`
	SMTPSubjectTemplate string   `json:"smtp_subject_template"`
	SMTPBodyTemplate    string   `json:"smtp_body_template"`
}

// TestEmailSettings 邮件配置测试结构体。
type TestEmailSettings struct {
	SMTPHost            string `json:"smtp_host"`
	SMTPPort            string `json:"smtp_port"`
	SMTPUsername        string `json:"smtp_username"`
	SMTPPassword        string `json:"smtp_password"`
	SMTPSSL             bool   `json:"smtp_ssl"`
	SMTPTo              string `json:"smtp_to"`
	SMTPSubjectTemplate string `json:"smtp_subject_template"`
	SMTPBodyTemplate    string `json:"smtp_body_template"`
}

// SaveGlobalSettings 保存全局配置。
func SaveGlobalSettings(body GlobalSettings) error {
	_ = db.SetSetting("backup_enabled", strconv.FormatBool(body.BackupEnabled))
	_ = db.SetSetting("backup_hours", strconv.Itoa(body.BackupHours))
	_ = db.SetSetting("restart_stack", strconv.FormatBool(body.RestartStack))
	_ = db.SetSetting("auto_update_enabled", strconv.FormatBool(body.AutoUpdateEnabled))
	_ = db.SetSetting("smtp_enabled", strconv.FormatBool(body.SMTPEnabled))
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

// SendTestEmail 发送测试邮件。
func SendTestEmail(body TestEmailSettings) error {
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
		"{action_type}", "容器升级",
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

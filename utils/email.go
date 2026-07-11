package utils

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strings"
)

// DefaultSMTPSubject 默认的邮件主题模板
const DefaultSMTPSubject = "【{status}】{container_name} {action_type} (Docker Updater)"

// DefaultSMTPSubjectCheck 默认的更新提醒邮件主题模板
const DefaultSMTPSubjectCheck = "【发现新版本】{container_name} 可升级 (Docker Updater)"

// DefaultSMTPBody 默认的邮件正文模板
const DefaultSMTPBody = "容器名称：{container_name}\n操作类型：{action_type}\n执行状态：{status}\n执行时间：{time}\n\n最近 20 行运行日志：\n----------------------------------------\n{logs}\n----------------------------------------"

// DefaultSMTPBodyCheck 默认的更新提醒邮件正文模板
const DefaultSMTPBodyCheck = "检测对象：{container_name}\n通知类型：{action_type}\n状态：{status}\n检测时间：{time}\n\n可升级镜像明细：\n----------------------------------------\n{logs}\n----------------------------------------"

// SMTPConfig 邮件发送配置参数，解耦数据库调用
type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	SSL      bool
	To       string
}

// SendNotificationEmail 使用传入的 SMTP 配置发送邮件报告
func SendNotificationEmail(cfg SMTPConfig, subject, body string) error {
	if cfg.Host == "" || cfg.Username == "" || cfg.Password == "" || cfg.To == "" {
		LogWarning("邮件通知发送取消: SMTP 配置不完整")
		return fmt.Errorf("SMTP 配置不完整")
	}

	LogInfo("正在发送任务邮件通知 (收件人: %s, 主题: %s)", cfg.To, subject)
	err := SendEmailRaw(cfg.Host, cfg.Port, cfg.Username, cfg.Password, cfg.To, cfg.SSL, subject, body)
	if err != nil {
		LogError("邮件通知发送失败: %s", err.Error())
		return err
	}

	LogSuccess("邮件通知发送成功 (收件人: %s)", cfg.To)
	return nil
}

// SendEmailRaw 执行底层的 SMTP 协议握手和邮件发送，支持 SSL 与普通 TLS 模式
func SendEmailRaw(host, portStr, username, password, to string, ssl bool, subject, body string) error {
	addr := host + ":" + portStr
	auth := smtp.PlainAuth("", username, password, host)

	header := make(map[string]string)
	header["From"] = username
	header["To"] = to
	header["Subject"] = subject
	header["Content-Type"] = "text/plain; charset=UTF-8"

	// 拼接邮件正文
	var sb strings.Builder
	for k, v := range header {
		sb.WriteString(fmt.Sprintf("%s: %s\r\n", k, v))
	}
	sb.WriteString("\r\n")
	sb.WriteString(body)
	message := sb.String()

	if ssl {
		// SSL 协议加密信道 (通常 465 端口)
		tlsconfig := &tls.Config{
			InsecureSkipVerify: true,
			ServerName:         host,
		}
		conn, err := tls.Dial("tcp", addr, tlsconfig)
		if err != nil {
			return err
		}
		defer conn.Close()

		c, err := smtp.NewClient(conn, host)
		if err != nil {
			return err
		}
		defer c.Quit()

		if err = c.Auth(auth); err != nil {
			return err
		}
		if err = c.Mail(username); err != nil {
			return err
		}
		if err = c.Rcpt(to); err != nil {
			return err
		}
		w, err := c.Data()
		if err != nil {
			return err
		}
		defer w.Close()
		_, err = w.Write([]byte(message))
		if err != nil {
			return err
		}
		return nil
	}

	// 非 SSL 信道模式 (通常 25 端口，或通过 STARTTLS)
	return smtp.SendMail(addr, auth, username, []string{to}, []byte(message))
}

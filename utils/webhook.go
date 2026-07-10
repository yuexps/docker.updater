package utils

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultWebhookTemplate 默认的 Webhook JSON 模板
const DefaultWebhookTemplate = `{
  "event": "docker_update",
  "container": "{container_name}",
  "action": "{action_type}",
  "status": "{status}",
  "time": "{time}",
  "logs": "{logs}"
}`

// SendWebhookNotification 发送 Webhook 通知。
func SendWebhookNotification(url string, method string, payload string) error {
	if url == "" {
		LogWarning("Webhook 通知发送取消: URL 为空")
		return fmt.Errorf("Webhook URL 为空")
	}

	if method == "" {
		method = "POST"
	}

	LogInfo("正在向 Webhook 地址发送通知: %s (Method: %s)", url, method)

	var reqBody io.Reader
	if method != "GET" && method != "DELETE" {
		reqBody = bytes.NewBuffer([]byte(payload))
	}

	req, err := http.NewRequest(method, url, reqBody)
	if err != nil {
		LogError("创建 Webhook 请求失败: %s", err.Error())
		return err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		LogError("发送 Webhook 失败: %s", err.Error())
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		LogError("Webhook 发送返回非成功状态码: %s", resp.Status)
		return fmt.Errorf("HTTP error: %s", resp.Status)
	}

	LogSuccess("Webhook 通知发送成功")
	return nil
}

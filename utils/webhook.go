package utils

import (
	"bytes"
	"encoding/json"
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

// DefaultWebhookTemplateCheck 默认的更新提醒 Webhook JSON 模板
const DefaultWebhookTemplateCheck = `{
  "event": "docker_update_check",
  "container": "{container_name}",
  "action": "{action_type}",
  "status": "{status}",
  "time": "{time}",
  "logs": "{logs}"
}`

// SendWebhookNotification 发送 Webhook 通知。
func SendWebhookNotification(url string, method string, payload string) (string, error) {
	if url == "" {
		LogWarning("Webhook 通知发送取消: URL 为空")
		return "", fmt.Errorf("Webhook URL 为空")
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
		return "", err
	}

	req.Header.Set("Content-Type", "application/json; charset=utf-8")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		LogError("发送 Webhook 失败: %s", err.Error())
		return "", err
	}
	defer resp.Body.Close()

	// 读取响应体内容（限定最大读取 2KB）
	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	bodyStr := string(bodyBytes)

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		LogError("Webhook 发送返回非成功状态码: %s, 响应: %s", resp.Status, bodyStr)
		if len(bodyStr) > 0 {
			if len(bodyStr) > 200 {
				bodyStr = bodyStr[:200] + "..."
			}
			return bodyStr, fmt.Errorf("HTTP error: %s (%s)", resp.Status, bodyStr)
		}
		return "", fmt.Errorf("HTTP error: %s", resp.Status)
	}

	// 即使返回 2xx，也需要校验钉钉、企业微信和飞书的错误响应
	if len(bodyBytes) > 0 {
		var jsonMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &jsonMap); err == nil {
			// 1. 钉钉/企业微信结构: {"errcode": 300001, "errmsg": "..."}
			if errcodeVal, ok := jsonMap["errcode"]; ok {
				var errcode int64
				switch v := errcodeVal.(type) {
				case float64:
					errcode = int64(v)
				case int:
					errcode = int64(v)
				case int64:
					errcode = v
				}
				if errcode != 0 {
					errmsg := ""
					if msg, ok := jsonMap["errmsg"].(string); ok {
						errmsg = msg
					}
					LogError("Webhook 平台返回错误码: %d, 信息: %s", errcode, errmsg)
					return bodyStr, fmt.Errorf("Webhook 平台报错 (errcode=%d): %s", errcode, errmsg)
				}
			}

			// 2. 飞书结构: {"code": 19001, "msg": "..."}
			if codeVal, ok := jsonMap["code"]; ok {
				var code int64
				switch v := codeVal.(type) {
				case float64:
					code = int64(v)
				case int:
					code = int64(v)
				case int64:
					code = v
				}
				if code != 0 {
					msg := ""
					if m, ok := jsonMap["msg"].(string); ok {
						msg = m
					}
					LogError("飞书 Webhook 平台返回错误码: %d, 信息: %s", code, msg)
					return bodyStr, fmt.Errorf("飞书 Webhook 报错 (code=%d): %s", code, msg)
				}
			}
		}
	}

	LogSuccess("Webhook 通知发送成功")
	return bodyStr, nil
}

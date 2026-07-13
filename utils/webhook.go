package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultWebhookTemplateCustom 自定义/通用的 Webhook JSON 模板
const DefaultWebhookTemplateCustom = `{
  "title": "【{status}】{container_name} {action_type}",
  "content": "容器名称: {container_name}\n任务类型: {action_type}\n执行状态: {status}\n执行时间: {time}\n\n最近运行日志:\n{logs}",
  "type": "text"
}`

// DefaultWebhookTemplateCheckCustom 自定义/通用的更新提醒 Webhook JSON 模板
const DefaultWebhookTemplateCheckCustom = `{
  "title": "【发现新版本】{container_name} 可升级",
  "content": "镜像名称: {container_name}\n通知类型: {action_type}\n当前状态: {status}\n检测时间: {time}\n\n可升级镜像明细:\n{logs}",
  "type": "text"
}`

// DefaultWebhookTemplateWechat 企业微信默认的 Webhook JSON 模板
const DefaultWebhookTemplateWechat = `{
  "msgtype": "markdown",
  "markdown": {
    "content": "### 【{status}】{container_name} {action_type}\n> 容器名称: <font color=\"info\">{container_name}</font>\n> 任务类型: <font color=\"comment\">{action_type}</font>\n> 执行状态: {status}\n> 执行时间: <font color=\"comment\">{time}</font>\n\n最近运行日志:\n` + "```" + `\n{logs}\n` + "```" + `"
  }
}`

// DefaultWebhookTemplateCheckWechat 企业微信默认的更新提醒 Webhook JSON 模板
const DefaultWebhookTemplateCheckWechat = `{
  "msgtype": "markdown",
  "markdown": {
    "content": "### 【发现新版本】{container_name} 可升级\n> 镜像名称: <font color=\"info\">{container_name}</font>\n> 通知类型: <font color=\"comment\">{action_type}</font>\n> 当前状态: {status}\n> 检测时间: <font color=\"comment\">{time}</font>\n\n可升级镜像明细:\n` + "```" + `\n{logs}\n` + "```" + `"
  }
}`

// DefaultWebhookTemplateDingtalk 钉钉默认的 Webhook JSON 模板
const DefaultWebhookTemplateDingtalk = `{
  "msgtype": "markdown",
  "markdown": {
    "title": "【{status}】{container_name}",
    "text": "### 【{status}】{container_name} {action_type}\n- **容器名称**: {container_name}\n- **任务类型**: {action_type}\n- **执行状态**: {status}\n- **执行时间**: {time}\n\n最近运行日志:\n` + "```" + `\n{logs}\n` + "```" + `"
  }
}`

// DefaultWebhookTemplateCheckDingtalk 钉钉默认的更新提醒 Webhook JSON 模板
const DefaultWebhookTemplateCheckDingtalk = `{
  "msgtype": "markdown",
  "markdown": {
    "title": "【发现新版本】{container_name}",
    "text": "### 【发现新版本】{container_name} 可升级\n- **镜像名称**: {container_name}\n- **通知类型**: {action_type}\n- **当前状态**: {status}\n- **检测时间**: {time}\n\n可升级镜像明细:\n` + "```" + `\n{logs}\n` + "```" + `"
  }
}`

// DefaultWebhookTemplateFeishu 飞书默认的 Webhook JSON 模板
const DefaultWebhookTemplateFeishu = `{
  "msg_type": "post",
  "content": {
    "post": {
      "zh_cn": {
        "title": "【{status}】{container_name} {action_type}",
        "content": [
          [
            {"tag": "text", "text": "容器名称: {container_name}\n"},
            {"tag": "text", "text": "任务类型: {action_type}\n"},
            {"tag": "text", "text": "执行状态: {status}\n"},
            {"tag": "text", "text": "执行时间: {time}\n\n"}
          ],
          [
            {"tag": "text", "text": "最近运行日志:\n{logs}"}
          ]
        ]
      }
    }
  }
}`

// DefaultWebhookTemplateCheckFeishu 飞书默认的更新提醒 Webhook JSON 模板
const DefaultWebhookTemplateCheckFeishu = `{
  "msg_type": "post",
  "content": {
    "post": {
      "zh_cn": {
        "title": "【发现新版本】{container_name} 可升级",
        "content": [
          [
            {"tag": "text", "text": "镜像名称: {container_name}\n"},
            {"tag": "text", "text": "通知类型: {action_type}\n"},
            {"tag": "text", "text": "当前状态: {status}\n"},
            {"tag": "text", "text": "检测时间: {time}\n\n"}
          ],
          [
            {"tag": "text", "text": "可升级镜像明细:\n{logs}"}
          ]
        ]
      }
    }
  }
}`

// GetDefaultWebhookTemplate 根据平台类型获取内置的默认 Webhook 模板
func GetDefaultWebhookTemplate(webhookType string, isCheck bool) string {
	if isCheck {
		switch webhookType {
		case "wechat":
			return DefaultWebhookTemplateCheckWechat
		case "dingtalk":
			return DefaultWebhookTemplateCheckDingtalk
		case "feishu":
			return DefaultWebhookTemplateCheckFeishu
		default:
			return DefaultWebhookTemplateCheckCustom
		}
	} else {
		switch webhookType {
		case "wechat":
			return DefaultWebhookTemplateWechat
		case "dingtalk":
			return DefaultWebhookTemplateDingtalk
		case "feishu":
			return DefaultWebhookTemplateFeishu
		default:
			return DefaultWebhookTemplateCustom
		}
	}
}

// SendWebhookNotification 发送 Webhook 通知。
func SendWebhookNotification(url string, method string, payload string, webhookType string) (string, error) {
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

	// 识别平台特征，双重检查当前类型设置以及 URL 域名
	isWechatOrDingtalk := webhookType == "wechat" || webhookType == "dingtalk" ||
		strings.Contains(url, "weixin.qq.com") || strings.Contains(url, "dingtalk.com")
	isFeishu := webhookType == "feishu" ||
		strings.Contains(url, "feishu.cn") || strings.Contains(url, "larksuite.com")

	if len(bodyBytes) > 0 {
		var jsonMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &jsonMap); err == nil {
			// 1. 微信/钉钉结构: {"errcode": 300001, "errmsg": "..."}
			if isWechatOrDingtalk {
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
			}

			// 2. 飞书结构: {"code": 19001, "msg": "..."}
			if isFeishu {
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
	}

	LogSuccess("Webhook 通知发送成功")
	return bodyStr, nil
}

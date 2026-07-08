package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"docker-updater/dockerclient"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// HubObserver 桥接 WsHub 以适配 dockerclient 的解耦事件观察接口
type HubObserver struct{}

func (HubObserver) OnLog(containerName string, taskType string, message string) {
	GlobalHub.BroadcastLog(containerName, taskType, message)
}

func (HubObserver) OnStatusChange() {
	GlobalHub.BroadcastStatus()
}

func init() {
	dockerclient.GlobalObserver = HubObserver{}
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		// FNOS 反代理环境下，Origin 检查放宽以适应不同网关转发配置
		return true
	},
}

// Client 维护单个活跃的 WebSocket 长连接状态
type Client struct {
	hub           *WsHub
	conn          *websocket.Conn
	send          chan []byte
	subscriptions map[string]bool // 订阅的主题集（如：logs:nginx-web）
	mu            sync.Mutex
}

// WsHub 全局 WebSocket 连接与消息分发管理中心
type WsHub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// GlobalHub 全局 Hub 单例
var GlobalHub = &WsHub{
	clients:    make(map[*Client]bool),
	register:   make(chan *Client),
	unregister: make(chan *Client),
}

// Run 启动 Hub 的主协程监听循环
func (h *WsHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			// 连接成功后，立即下发一次系统整体状态
			h.SendInitialState(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastStatus 推送当前最新容器状态与更新历史给所有已连接客户端
func (h *WsHub) BroadcastStatus() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.clients) == 0 {
		return
	}

	data, err := GetStatusData(context.Background())
	if err != nil {
		return
	}

	payload, _ := json.Marshal(map[string]interface{}{
		"type":    "status",
		"payload": data,
	})

	for client := range h.clients {
		select {
		case client.send <- payload:
		default:
			go func(c *Client) { h.unregister <- c }(client)
		}
	}
}

// SendInitialState 单播初始状态给指定客户端
func (h *WsHub) SendInitialState(client *Client) {
	data, err := GetStatusData(context.Background())
	if err != nil {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"type":    "status",
		"payload": data,
	})
	select {
	case client.send <- payload:
	default:
		go func(c *Client) { h.unregister <- c }(client)
	}
}

// BroadcastLog 向订阅了对应容器日志主题的所有客户端分发实时日志
func (h *WsHub) BroadcastLog(containerName string, taskType string, message string) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	topic := "logs:" + containerName
	payload, _ := json.Marshal(map[string]interface{}{
		"type": "log",
		"payload": map[string]string{
			"container": containerName,
			"task":      taskType,
			"message":   message,
		},
	})

	for client := range h.clients {
		client.mu.Lock()
		subscribed := client.subscriptions[topic]
		client.mu.Unlock()

		if subscribed {
			select {
			case client.send <- payload:
			default:
				go func(c *Client) { h.unregister <- c }(client)
			}
		}
	}
}

// BroadcastSysLog 向所有已连接客户端推送系统运行日志新行
func (h *WsHub) BroadcastSysLog(line string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.clients) == 0 {
		return
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"type":    "syslog",
		"payload": map[string]string{"line": line},
	})
	for client := range h.clients {
		select {
		case client.send <- payload:
		default:
			go func(c *Client) { h.unregister <- c }(client)
		}
	}
}

// readPump 循环读取客户端发送的控制消息
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(512)
	_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		_ = c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			break
		}

		var req struct {
			Type   string `json:"type"`
			Target string `json:"target"`
		}
		if err := json.Unmarshal(message, &req); err != nil {
			continue
		}

		switch req.Type {
		case "subscribe":
			c.mu.Lock()
			c.subscriptions[req.Target] = true
			c.mu.Unlock()
		case "unsubscribe":
			c.mu.Lock()
			delete(c.subscriptions, req.Target)
			c.mu.Unlock()
		case "ping":
			payload, _ := json.Marshal(map[string]string{"type": "pong"})
			c.send <- payload
		}
	}
}

// writePump 循环向客户端连接写入队列中的消息
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				_ = c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			_, _ = w.Write(message)

			n := len(c.send)
			for i := 0; i < n; i++ {
				_, _ = w.Write([]byte{'\n'})
				_, _ = w.Write(<-c.send)
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// HandleWebSocket 挂载升级 Gin 请求为 WebSocket 路由的入口函数
func HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Printf("[ERROR] WebSocket 升级失败: %s", err.Error())
		return
	}

	client := &Client{
		hub:           GlobalHub,
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]bool),
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

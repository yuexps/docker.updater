package api

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"docker-updater/service"
	"docker-updater/utils"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// HubObserver 实现服务层与 WebSocket 广播服务的事件解耦。
type HubObserver struct{}

// OnLog 实现实时日志的广播。
func (HubObserver) OnLog(containerName string, taskType string, message string) {
	GlobalHub.BroadcastLog(containerName, taskType, message)
}

// OnStatusChange 实现全局状态更新的广播。
func (HubObserver) OnStatusChange() {
	GlobalHub.BroadcastStatus()
}

func init() {
	service.GlobalObserver = HubObserver{}
	utils.RegisterLogListener(func(event utils.LogEvent) {
		if GlobalHub == nil {
			return
		}
		// 广播系统运行日志
		GlobalHub.BroadcastSysLog(event.Format())

		// 广播容器任务日志至订阅窗口
		if event.Container != "" {
			GlobalHub.BroadcastLog(event.Container, "task", event.TaskLogFormat())
		}
	})
}

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// Client 维护 WebSocket 连接对应的状态数据。
type Client struct {
	hub           *WsHub
	conn          *websocket.Conn
	send          chan []byte
	subscriptions map[string]bool // 已订阅的主题映射表。
	mu            sync.Mutex
	done          chan struct{}
	closeOnce     sync.Once
}

// Close 安全地关闭 Client 相关的连接和通道。
func (c *Client) Close() {
	c.closeOnce.Do(func() {
		close(c.done)
		c.conn.Close()
	})
}

// WsHub 全局 WebSocket 连接与广播事务控制中心。
type WsHub struct {
	clients    map[*Client]bool
	register   chan *Client
	unregister chan *Client
	mu         sync.RWMutex
}

// GlobalHub 导出全局唯一广播中心实例。
var GlobalHub = &WsHub{
	clients:    make(map[*Client]bool),
	register:   make(chan *Client, 256),
	unregister: make(chan *Client, 256),
}

// Run 启动广播中心的消息监听循环。
func (h *WsHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			h.SendInitialState(client)

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.Close()
			}
			h.mu.Unlock()
		}
	}
}

// BroadcastStatus 广播当前最新容器状态和任务历史至所有连接。
func (h *WsHub) BroadcastStatus() {
	h.mu.RLock()
	defer h.mu.RUnlock()
	if len(h.clients) == 0 {
		return
	}

	data, err := service.GetContainerStatusData(context.Background())
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
			select {
			case h.unregister <- client:
			default:
				client.conn.Close()
			}
		}
	}
}

// SendInitialState 单播初始状态至指定的客户端。
func (h *WsHub) SendInitialState(client *Client) {
	data, err := service.GetContainerStatusData(context.Background())
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
		select {
		case h.unregister <- client:
		default:
			client.conn.Close()
		}
	}
}

// BroadcastLog 广播指定容器的任务执行日志至订阅客户端。
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
				select {
				case h.unregister <- client:
				default:
					client.conn.Close()
				}
			}
		}
	}
}

// BroadcastSysLog 广播系统日志至所有连接。
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
			select {
			case h.unregister <- client:
			default:
				client.conn.Close()
			}
		}
	}
}

// readPump 循环读取客户端 data。
func (c *Client) readPump() {
	defer func() {
		if r := recover(); r != nil {
			utils.LogError("websocket readPump 异常恢复: %v", r)
		}
		c.hub.unregister <- c
		c.Close()
	}()

	c.conn.SetReadLimit(4096)
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
			select {
			case c.send <- payload:
			default:
			}
		}
	}
}

// writePump 循环向连接发送消息。
func (c *Client) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	defer func() {
		ticker.Stop()
		c.Close()
	}()

	for {
		select {
		case message := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
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

		case <-c.done:
			return

		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// HandleWebSocket 将 Gin 上下文升级为 WebSocket 协议并初始化客户端长连接。
func HandleWebSocket(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		utils.LogError("WebSocket 升级失败: %s", err.Error())
		return
	}

	client := &Client{
		hub:           GlobalHub,
		conn:          conn,
		send:          make(chan []byte, 256),
		subscriptions: make(map[string]bool),
		done:          make(chan struct{}),
	}

	client.hub.register <- client

	go client.writePump()
	go client.readPump()
}

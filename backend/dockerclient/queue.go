package dockerclient

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"docker-updater/db"
)

type TaskType string

const (
	TaskUpdate   TaskType = "update"
	TaskRollback TaskType = "rollback"
)

// QueueObserver 声明队列状态与日志事件观察者接口，防止包循环引用
type QueueObserver interface {
	OnLog(containerName string, taskType string, message string)
	OnStatusChange()
}

var GlobalObserver QueueObserver

type Task struct {
	mu            sync.Mutex
	ContainerName string    `json:"container_name"`
	Type          TaskType  `json:"type"`
	TargetImage   string    `json:"target_image"`
	IsAuto        bool      `json:"is_auto"`
	Status        string    `json:"status"` // "waiting", "running", "success", "failed", "cancelled"
	AddedAt       string    `json:"added_at"`
	Logs          []string  `json:"-"`
	listeners     []chan string
}

func (t *Task) AddLog(msg string) {
	t.mu.Lock()
	t.Logs = append(t.Logs, msg)
	t.mu.Unlock()

	// 1. 本地监听管道通知
	for _, ch := range t.listeners {
		select {
		case ch <- msg:
		default:
		}
	}

	// 2. 触发全局 WebSocket 观察者实时广播
	if GlobalObserver != nil {
		GlobalObserver.OnLog(t.ContainerName, string(t.Type), msg)
	}
}

func (t *Task) AddListener(ch chan string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.listeners = append(t.listeners, ch)
}

func (t *Task) RemoveListener(ch chan string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, l := range t.listeners {
		if l == ch {
			t.listeners = append(t.listeners[:i], t.listeners[i+1:]...)
			break
		}
	}
}

type QueueManager struct {
	mu      sync.RWMutex
	tasks   []*Task
	active  *Task
	jobChan chan *Task
}

var GlobalQueue *QueueManager

// InitQueueManager 初始化全局任务队列管理器并启动后台 Worker 协程
func InitQueueManager() {
	GlobalQueue = &QueueManager{
		tasks:   make([]*Task, 0),
		jobChan: make(chan *Task, 100),
	}
	go GlobalQueue.worker()
}

func (q *QueueManager) AddTask(name string, tType TaskType, targetImage string, isAuto bool) *Task {
	q.mu.Lock()
	needBroadcast := false
	defer func() {
		q.mu.Unlock()
		if needBroadcast && GlobalObserver != nil {
			GlobalObserver.OnStatusChange()
		}
	}()

	// 1. 如果当前正在执行该容器的任务，直接返回
	if q.active != nil && q.active.ContainerName == name && q.active.Type == tType {
		return q.active
	}

	// 2. 如果任务已经在排队队列中，直接返回
	for _, t := range q.tasks {
		if t.ContainerName == name && t.Type == tType {
			return t
		}
	}

	// 3. 新建任务入队
	t := &Task{
		ContainerName: name,
		Type:          tType,
		TargetImage:   targetImage,
		IsAuto:        isAuto,
		Status:        "waiting",
		AddedAt:       time.Now().UTC().Format(time.RFC3339),
		Logs:          make([]string, 0),
		listeners:     make([]chan string, 0),
	}
	q.tasks = append(q.tasks, t)
	q.jobChan <- t
	needBroadcast = true
	return t
}

func (q *QueueManager) CancelTask(name string) bool {
	q.mu.Lock()
	needBroadcast := false
	defer func() {
		q.mu.Unlock()
		if needBroadcast && GlobalObserver != nil {
			GlobalObserver.OnStatusChange()
		}
	}()

	for i, t := range q.tasks {
		if t.ContainerName == name && t.Status == "waiting" {
			// 从列表中移除
			q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
			t.Status = "cancelled"
			needBroadcast = true
			return true
		}
	}
	return false
}

func (q *QueueManager) GetQueueState() ([]*Task, *Task) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	queued := make([]*Task, len(q.tasks))
	copy(queued, q.tasks)
	return queued, q.active
}

func (q *QueueManager) GetTask(name string) *Task {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.active != nil && q.active.ContainerName == name {
		return q.active
	}
	for _, t := range q.tasks {
		if t.ContainerName == name {
			return t
		}
	}
	return nil
}

func (q *QueueManager) worker() {
	for task := range q.jobChan {
		// 1. 检查任务是否已取消
		q.mu.Lock()
		if task.Status == "cancelled" {
			q.mu.Unlock()
			continue
		}
		// 2. 移出等待队列，设为 active 状态
		for i, t := range q.tasks {
			if t == task {
				q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
				break
			}
		}
		task.Status = "running"
		q.active = task
		q.mu.Unlock()

		if GlobalObserver != nil {
			GlobalObserver.OnStatusChange()
		}

		// 3. 执行任务操作
		ctx := context.Background()
		streamChan := make(chan string, 10)
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			for msg := range streamChan {
				task.AddLog(msg)
			}
		}()

		var err error
		if task.Type == TaskUpdate {
			err = ApplyUpdate(ctx, task.ContainerName, task.TargetImage, streamChan)
		} else {
			err = ApplyRollback(ctx, task.ContainerName, streamChan)
		}
		close(streamChan)
		wg.Wait()

		// 4. 更新任务状态并回收
		q.mu.Lock()
		if err != nil {
			task.Status = "failed"
		} else {
			task.Status = "success"
		}
		q.active = nil
		q.mu.Unlock()

		if GlobalObserver != nil {
			GlobalObserver.OnStatusChange()
		}

		// 5. 触发邮件通知 (仅针对后台自动检测的静默升级任务发送邮件通知，手动操作不发送以防冗余骚扰)
		if db.GetSetting("smtp_enabled", "false") == "true" && task.IsAuto {
			go func(taskName string, tType TaskType, status string, logs []string) {
				typeName := "版本修改"
				if tType == TaskRollback {
					typeName = "回滚恢复"
				}
				
				statusName := "执行成功"
				switch status {
				case "failed":
					statusName = "执行失败"
				case "cancelled":
					statusName = "已取消"
				}

				// 提取最近的 20 行日志以展示在邮件中
				logLen := len(logs)
				startIdx := logLen - 20
				if startIdx < 0 {
					startIdx = 0
				}
				recentLogs := logs[startIdx:]
				logContent := strings.Join(recentLogs, "\n")

				subjectTpl := db.GetSetting("smtp_subject_template", DefaultSMTPSubject)
				bodyTpl := db.GetSetting("smtp_body_template", DefaultSMTPBody)

				r := strings.NewReplacer(
					"{container_name}", taskName,
					"{action_type}", typeName,
					"{status}", statusName,
					"{time}", time.Now().Local().Format("2006-01-02 15:04:05"),
					"{logs}", logContent,
				)

				subject := r.Replace(subjectTpl)
				body := r.Replace(bodyTpl)
				
				_ = SendNotificationEmail(subject, body)
			}(task.ContainerName, task.Type, task.Status, task.GetLogs())
		}

		// 6. 持久化日志到本地文件
		pkgVar := os.Getenv("TRIM_PKGVAR")
		if pkgVar == "" {
			pkgVar = "./data"
		}
		logDir := filepath.Join(pkgVar, "logs")
		_ = os.MkdirAll(logDir, 0755)
		logFilePath := filepath.Join(logDir, fmt.Sprintf("%s.log", task.ContainerName))
		_ = os.WriteFile(logFilePath, []byte(strings.Join(task.Logs, "\n")), 0644)
		log.Printf("[INFO] 任务队列: 容器 %s 升级流日志已持久化保存 (%d 行)\n", task.ContainerName, len(task.Logs))
	}
}

func (t *Task) GetLogs() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	logs := make([]string, len(t.Logs))
	copy(logs, t.Logs)
	return logs
}

// MarshalJSON 序列化过滤掉互斥锁等内部不可读字段
func (t *Task) MarshalJSON() ([]byte, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	type Alias Task
	return json.Marshal(&struct {
		*Alias
	}{
		Alias: (*Alias)(t),
	})
}

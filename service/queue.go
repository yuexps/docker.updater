package service

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"docker-updater/db"
	"docker-updater/dockerclient"
	"docker-updater/utils"
)

type TaskType string

const (
	TaskUpdate TaskType = "update"
)

// QueueObserver 队列状态观察接口
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
	Status        string    `json:"status"`
	AddedAt       string    `json:"added_at"`
	Logs          []string  `json:"-"`
	listeners     []chan string
}

func (t *Task) AddLog(msg string) {
	rawMsg := strings.TrimSpace(msg)
	if rawMsg == "" {
		return
	}

	level := utils.LevelInfo
	cleanMsg := rawMsg

	if strings.HasPrefix(rawMsg, "[ERROR]") {
		level = utils.LevelError
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(rawMsg, "[ERROR]"))
	} else if strings.HasPrefix(rawMsg, "[WARN]") {
		level = utils.LevelWarn
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(rawMsg, "[WARN]"))
	} else if strings.HasPrefix(rawMsg, "[INFO]") {
		level = utils.LevelInfo
		cleanMsg = strings.TrimSpace(strings.TrimPrefix(rawMsg, "[INFO]"))
	}

	// 触发 CentralLogger 核心调度分发
	utils.Log(level, t.ContainerName, "%s", cleanMsg)

	// 记录在内存 Task 实例中供 HTTP API 快照读取
	taskLogLine := fmt.Sprintf("%s [%s] %s", time.Now().Format("2006-01-02 15:04:05"), level, cleanMsg)

	t.mu.Lock()
	t.Logs = append(t.Logs, taskLogLine)
	listenersCopy := make([]chan string, len(t.listeners))
	copy(listenersCopy, t.listeners)
	t.mu.Unlock()

	for _, ch := range listenersCopy {
		select {
		case ch <- taskLogLine:
		default:
		}
	}
}

func (t *Task) AddListener(ch chan string) {
	t.mu.Lock()
	t.listeners = append(t.listeners, ch)
	t.mu.Unlock()
}

func (t *Task) RemoveListener(ch chan string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	for i, l := range t.listeners {
		if l == ch {
			copy(t.listeners[i:], t.listeners[i+1:])
			t.listeners[len(t.listeners)-1] = nil
			t.listeners = t.listeners[:len(t.listeners)-1]
			break
		}
	}
}

func (t *Task) SetStatus(status string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Status = status
}

func (t *Task) GetStatus() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.Status
}

type QueueManager struct {
	mu      sync.RWMutex
	tasks   []*Task
	active  *Task
	jobChan chan *Task
}

var GlobalQueue *QueueManager

// InitQueueManager 初始化任务队列管理器
func InitQueueManager() {
	GlobalQueue = &QueueManager{
		tasks:   make([]*Task, 0),
		jobChan: make(chan *Task, 100),
	}
	go GlobalQueue.worker()
}

func (q *QueueManager) AddTask(name string, tType TaskType, targetImage string, isAuto bool) *Task {
	q.mu.Lock()
	if q.active != nil && q.active.ContainerName == name {
		q.mu.Unlock()
		return q.active
	}

	for _, t := range q.tasks {
		if t.ContainerName == name {
			q.mu.Unlock()
			return t
		}
	}

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
	q.mu.Unlock()

	q.jobChan <- t

	if GlobalObserver != nil {
		GlobalObserver.OnStatusChange()
	}
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
		if t.ContainerName == name && t.GetStatus() == "waiting" {
			q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
			t.SetStatus("cancelled")
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
		q.mu.Lock()
		if task.GetStatus() == "cancelled" {
			q.mu.Unlock()
			continue
		}
		for i, t := range q.tasks {
			if t == task {
				q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
				break
			}
		}
		task.SetStatus("running")
		q.active = task
		q.mu.Unlock()

		utils.LogInfo("任务队列: 开始执行容器 %s 的升级更新...", task.ContainerName)

		if GlobalObserver != nil {
			GlobalObserver.OnStatusChange()
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		streamChan := make(chan string, 10)
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer wg.Done()
			for msg := range streamChan {
				task.AddLog(msg)
			}
		}()

		var dockerhubMirrors []string
		var ghcrMirrors []string
		dockerhubMirrorsStr := db.GetSetting("dockerhub_mirrors", "")
		if dockerhubMirrorsStr == "" {
			dockerhubMirrorsStr = db.GetSetting("temp_mirrors", "[]")
		}
		_ = json.Unmarshal([]byte(dockerhubMirrorsStr), &dockerhubMirrors)
		ghcrMirrorsStr := db.GetSetting("ghcr_mirrors", "[]")
		_ = json.Unmarshal([]byte(ghcrMirrorsStr), &ghcrMirrors)

		opts := dockerclient.UpdateOptions{
			TempMirrors:      dockerhubMirrors,
			DockerhubMirrors: dockerhubMirrors,
			GHCRMirrors:      ghcrMirrors,
		}

		err := dockerclient.ApplyUpdate(ctx, task.ContainerName, task.TargetImage, opts, streamChan)

		if err == nil {
			db.DB.Delete(&db.AvailableUpdate{ContainerName: task.ContainerName})

			history := db.UpdateHistory{
				ContainerName: task.ContainerName,
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
				Status:        "success",
			}
			cli, cliErr := dockerclient.NewLocalClient()
			if cliErr == nil {
				defer cli.Close()
				if inspect, inspectErr := cli.ContainerInspect(ctx, task.ContainerName); inspectErr == nil {
					history.Image = inspect.Config.Image
				}
			}
			if history.Image == "" {
				history.Image = task.TargetImage
			}
			db.DB.Create(&history)
		}
		close(streamChan)
		wg.Wait()
		cancel()

		q.mu.Lock()
		if err != nil {
			task.SetStatus("failed")
			utils.LogError("任务队列: 容器 %s 升级更新失败: %v", task.ContainerName, err)
			if task.Type == TaskUpdate && task.IsAuto {
				d := db.DeferredUpdate{
					ContainerName: task.ContainerName,
					Until:         "forever",
				}
				_ = db.DB.Save(&d).Error
				task.AddLog("[ERROR] 自动升级失败，为了防止后台陷入死循环，已自动将该容器设为 [永久暂挂]")
			}
		} else {
			task.SetStatus("success")
			utils.LogSuccess("任务队列: 容器 %s 升级更新成功", task.ContainerName)
		}
		q.active = nil
		q.mu.Unlock()

		if GlobalObserver != nil {
			GlobalObserver.OnStatusChange()
		}

		if task.IsAuto {
			go func(taskName string, tType TaskType, status string, logs []string) {
				typeName := NotifyActionUpdate

				statusName := "执行成功"
				switch status {
				case "failed":
					statusName = "执行失败"
				case "cancelled":
					statusName = "已取消"
				}

				logLen := len(logs)
				startIdx := logLen - 20
				if startIdx < 0 {
					startIdx = 0
				}
				recentLogs := logs[startIdx:]
				logContent := strings.Join(recentLogs, "\n")

				SendNotification(taskName, typeName, statusName, logContent)
			}(task.ContainerName, task.Type, task.GetStatus(), task.GetLogs())
		}

		logFilePath := utils.GetLogFilePath(task.ContainerName)
		_ = os.MkdirAll(filepath.Dir(logFilePath), 0755)
		_ = os.WriteFile(logFilePath, []byte(strings.Join(task.GetLogs(), "\n")), 0644)
		utils.LogInfo("任务队列: 容器 %s 升级流日志已持久化保存 (%d 行)", task.ContainerName, len(task.Logs))
	}
}

func (t *Task) GetLogs() []string {
	t.mu.Lock()
	defer t.mu.Unlock()
	logs := make([]string, len(t.Logs))
	copy(logs, t.Logs)
	return logs
}

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

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

	"github.com/docker/docker/api/types/container"
)

type TaskType string

const (
	TaskUpdate   TaskType = "update"
	TaskRollback TaskType = "rollback"
)

// QueueObserver 队列状态观察接口。
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
	t.mu.Lock()
	t.Logs = append(t.Logs, msg)
	listenersCopy := make([]chan string, len(t.listeners))
	copy(listenersCopy, t.listeners)
	t.mu.Unlock()

	for _, ch := range listenersCopy {
		select {
		case ch <- msg:
		default:
		}
	}

	if GlobalObserver != nil {
		GlobalObserver.OnLog(t.ContainerName, string(t.Type), msg)
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

// InitQueueManager 初始化任务队列管理器。
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

	if q.active != nil && q.active.ContainerName == name {
		return q.active
	}

	for _, t := range q.tasks {
		if t.ContainerName == name {
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
		q.mu.Lock()
		if task.Status == "cancelled" {
			q.mu.Unlock()
			continue
		}
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

		var err error
		if task.Type == TaskUpdate {
			backupHours := 24
			fmt.Sscanf(db.GetSetting("backup_hours", "24"), "%d", &backupHours)

			var tempMirrors []string
			tempMirrorsStr := db.GetSetting("temp_mirrors", "[]")
			_ = json.Unmarshal([]byte(tempMirrorsStr), &tempMirrors)

			opts := dockerclient.UpdateOptions{
				BackupEnabled: db.GetSetting("backup_enabled", "false") == "true",
				BackupHours:   backupHours,
				RestartStack:  db.GetSetting("restart_stack", "false") == "true",
				TempMirrors:   tempMirrors,
			}

			var policyStr string
			policyStr, err = dockerclient.ApplyUpdate(ctx, task.ContainerName, task.TargetImage, opts, streamChan)

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

				if opts.BackupEnabled && policyStr != "" {
					expiresAt := time.Now().Add(time.Duration(backupHours) * time.Hour).UTC().Format(time.RFC3339)
					rollbackMeta := db.RollbackMetadata{
						ContainerName: task.ContainerName,
						BackedUpAt:    time.Now().UTC().Format(time.RFC3339),
						ExpiresAt:     expiresAt,
						RestartPolicy: policyStr,
					}
					db.DB.Save(&rollbackMeta)
				}
			}
		} else {
			var policy container.RestartPolicy
			var meta db.RollbackMetadata
			if dbErr := db.DB.First(&meta, "container_name = ?", task.ContainerName).Error; dbErr == nil {
				_ = json.Unmarshal([]byte(meta.RestartPolicy), &policy)
			} else {
				policy = container.RestartPolicy{Name: "unless-stopped"}
			}

			err = dockerclient.ApplyRollback(ctx, task.ContainerName, policy, streamChan)

			if err == nil {
				db.DB.Delete(&db.RollbackMetadata{ContainerName: task.ContainerName})

				history := db.UpdateHistory{
					ContainerName: task.ContainerName,
					Image:         "rollback",
					UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
					Status:        "success",
				}
				db.DB.Create(&history)
			}
		}
		close(streamChan)
		wg.Wait()
		cancel()

		q.mu.Lock()
		if err != nil {
			task.Status = "failed"
			if task.Type == TaskUpdate && task.IsAuto {
				d := db.DeferredUpdate{
					ContainerName: task.ContainerName,
					Until:         "forever",
				}
				_ = db.DB.Save(&d).Error
				task.AddLog("[SYSTEM] 自动升级失败，为了防止后台陷入死循环，已自动将该容器设为 [永久暂挂]")
			}
		} else {
			task.Status = "success"
		}
		q.active = nil
		q.mu.Unlock()

		if GlobalObserver != nil {
			GlobalObserver.OnStatusChange()
		}

		if task.IsAuto {
			go func(taskName string, tType TaskType, status string, logs []string) {
				typeName := "容器升级"
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

				logLen := len(logs)
				startIdx := logLen - 20
				if startIdx < 0 {
					startIdx = 0
				}
				recentLogs := logs[startIdx:]
				logContent := strings.Join(recentLogs, "\n")

				SendNotification(taskName, typeName, statusName, logContent)
			}(task.ContainerName, task.Type, task.Status, task.GetLogs())
		}

		pkgVar := os.Getenv("TRIM_PKGVAR")
		if pkgVar == "" {
			pkgVar = "./data"
		}
		logDir := filepath.Join(pkgVar, "logs")
		_ = os.MkdirAll(logDir, 0755)
		logFilePath := filepath.Join(logDir, fmt.Sprintf("%s.log", task.ContainerName))
		_ = os.WriteFile(logFilePath, []byte(strings.Join(task.Logs, "\n")), 0644)
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

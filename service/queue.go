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
	BatchID       string    `json:"batch_id,omitempty"`
	Status        string    `json:"status"`
	AddedAt       string    `json:"added_at"`
	Logs          []string  `json:"-"`
	listeners     []chan string
}

// BatchTaskResult 自动更新批次中单个任务的执行结果
type BatchTaskResult struct {
	ContainerName string
	Status        string
	Image         string
	Logs          []string
}

// UpdateBatch 自动更新批次状态追踪器
type UpdateBatch struct {
	ID        string
	Total     int
	Results   []BatchTaskResult
	StartedAt time.Time
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
	mu       sync.RWMutex
	tasks    []*Task
	active   *Task
	jobChan  chan *Task
	batches  map[string]*UpdateBatch
	batchSeq uint64
}

var GlobalQueue *QueueManager

// InitQueueManager 初始化任务队列管理器
func InitQueueManager() {
	GlobalQueue = &QueueManager{
		tasks:   make([]*Task, 0),
		jobChan: make(chan *Task, 100),
		batches: make(map[string]*UpdateBatch),
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

// EnqueueAutoBatch 将一批自动更新容器原子入队并绑定同一批次 ID
func (q *QueueManager) EnqueueAutoBatch(containerNames []string) []*Task {
	q.mu.Lock()
	if q.batches == nil {
		q.batches = make(map[string]*UpdateBatch)
	}

	existing := make(map[string]bool)
	if q.active != nil {
		existing[q.active.ContainerName] = true
	}
	for _, t := range q.tasks {
		existing[t.ContainerName] = true
	}

	var targets []string
	for _, name := range containerNames {
		if name == "" || existing[name] {
			continue
		}
		existing[name] = true
		targets = append(targets, name)
	}

	if len(targets) == 0 {
		q.mu.Unlock()
		return nil
	}

	q.batchSeq++
	batchID := fmt.Sprintf("auto-%d-%d", time.Now().UnixNano(), q.batchSeq)
	created := make([]*Task, 0, len(targets))
	nowStr := time.Now().UTC().Format(time.RFC3339)

	for _, name := range targets {
		t := &Task{
			ContainerName: name,
			Type:          TaskUpdate,
			IsAuto:        true,
			BatchID:       batchID,
			Status:        "waiting",
			AddedAt:       nowStr,
			Logs:          make([]string, 0),
			listeners:     make([]chan string, 0),
		}
		q.tasks = append(q.tasks, t)
		created = append(created, t)
	}

	q.batches[batchID] = &UpdateBatch{
		ID:        batchID,
		Total:     len(created),
		Results:   make([]BatchTaskResult, 0, len(created)),
		StartedAt: time.Now(),
	}
	q.mu.Unlock()

	for _, t := range created {
		q.jobChan <- t
	}

	if GlobalObserver != nil {
		GlobalObserver.OnStatusChange()
	}
	return created
}

func (q *QueueManager) CancelTask(name string) bool {
	q.mu.Lock()
	needBroadcast := false
	cancelledBatchID := ""
	cancelledName := ""

	for i, t := range q.tasks {
		if t.ContainerName == name && t.GetStatus() == "waiting" {
			q.tasks = append(q.tasks[:i], q.tasks[i+1:]...)
			t.SetStatus("cancelled")
			cancelledBatchID = t.BatchID
			cancelledName = t.ContainerName
			needBroadcast = true
			break
		}
	}
	q.mu.Unlock()

	if cancelledBatchID != "" {
		q.completeBatchTask(cancelledBatchID, BatchTaskResult{
			ContainerName: cancelledName,
			Status:        "cancelled",
		})
	}
	if needBroadcast && GlobalObserver != nil {
		GlobalObserver.OnStatusChange()
	}
	return needBroadcast
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

		updatedImage := task.TargetImage
		if err == nil {
			db.DB.Delete(&db.AvailableUpdate{ContainerName: task.ContainerName})

			history := db.UpdateHistory{
				ContainerName: task.ContainerName,
				UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
				Status:        "success",
			}
			cli, cliErr := dockerclient.NewLocalClient()
			if cliErr == nil {
				if inspect, inspectErr := cli.ContainerInspect(ctx, task.ContainerName); inspectErr == nil {
					history.Image = inspect.Config.Image
				}
				_ = cli.Close()
			}
			if history.Image == "" {
				history.Image = task.TargetImage
			}
			updatedImage = history.Image
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

		if task.BatchID != "" {
			q.completeBatchTask(task.BatchID, BatchTaskResult{
				ContainerName: task.ContainerName,
				Status:        task.GetStatus(),
				Image:         updatedImage,
				Logs:          task.GetLogs(),
			})
		} else if task.IsAuto {
			go func(taskName string, status string, logs []string) {
				statusName := formatTaskStatusName(status)
				logContent := joinRecentLogs(logs, 20)
				notifyDispatcher(taskName, NotifyActionUpdate, statusName, logContent)
			}(task.ContainerName, task.GetStatus(), task.GetLogs())
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

// recordBatchResult 记录批次内单个任务结果，全部完成时移出并返回该批次
func (q *QueueManager) recordBatchResult(batchID string, res BatchTaskResult) *UpdateBatch {
	q.mu.Lock()
	defer q.mu.Unlock()
	batch, ok := q.batches[batchID]
	if !ok {
		return nil
	}
	batch.Results = append(batch.Results, res)
	if len(batch.Results) < batch.Total {
		return nil
	}
	delete(q.batches, batchID)
	return batch
}

// completeBatchTask 上报批次任务终态，并在整批完成时异步发送汇总通知
func (q *QueueManager) completeBatchTask(batchID string, res BatchTaskResult) {
	batch := q.recordBatchResult(batchID, res)
	if batch == nil {
		return
	}
	go func(completed *UpdateBatch) {
		containerLabel, actionType, statusName, logContent := buildBatchNotification(completed)
		notifyDispatcher(containerLabel, actionType, statusName, logContent)
	}(batch)
}

func formatTaskStatusName(status string) string {
	switch status {
	case "failed":
		return "执行失败"
	case "cancelled":
		return "已取消"
	default:
		return "执行成功"
	}
}

func joinRecentLogs(logs []string, limit int) string {
	if len(logs) == 0 {
		return ""
	}
	startIdx := len(logs) - limit
	if startIdx < 0 {
		startIdx = 0
	}
	return strings.Join(logs[startIdx:], "\n")
}

// buildBatchNotification 构建单容器或多容器批次汇总通知参数
func buildBatchNotification(batch *UpdateBatch) (string, string, string, string) {
	total := len(batch.Results)
	if total == 0 {
		return "", NotifyActionUpdate, "执行成功", ""
	}
	if total == 1 {
		r := batch.Results[0]
		logContent := joinRecentLogs(r.Logs, 20)
		if r.Status == "cancelled" && logContent == "" {
			logContent = "任务已在排队期间被手动取消"
		}
		return r.ContainerName, NotifyActionUpdate, formatTaskStatusName(r.Status), logContent
	}

	var successCount, failedCount, cancelledCount int
	for _, r := range batch.Results {
		switch r.Status {
		case "failed":
			failedCount++
		case "cancelled":
			cancelledCount++
		default:
			successCount++
		}
	}

	statusName := buildBatchStatusSummary(total, successCount, failedCount, cancelledCount)
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("本次自动更新汇总：共 %d 个容器（成功 %d，失败 %d，取消 %d）", total, successCount, failedCount, cancelledCount))

	for _, r := range batch.Results {
		sb.WriteString("\n")
		switch r.Status {
		case "failed":
			sb.WriteString(fmt.Sprintf("- [失败] %s", r.ContainerName))
			recent := joinRecentLogs(r.Logs, 6)
			if recent != "" {
				for _, line := range strings.Split(recent, "\n") {
					sb.WriteString("\n  " + line)
				}
			}
		case "cancelled":
			sb.WriteString(fmt.Sprintf("- [取消] %s (排队中被手动取消)", r.ContainerName))
		default:
			if r.Image != "" {
				sb.WriteString(fmt.Sprintf("- [成功] %s (%s)", r.ContainerName, r.Image))
			} else {
				sb.WriteString(fmt.Sprintf("- [成功] %s", r.ContainerName))
			}
		}
	}

	containerLabel := fmt.Sprintf("多个容器 (共 %d 个)", total)
	return containerLabel, NotifyActionUpdate, statusName, sb.String()
}

func buildBatchStatusSummary(total, successCount, failedCount, cancelledCount int) string {
	if successCount == total {
		return fmt.Sprintf("全部成功 (%d/%d)", successCount, total)
	}
	if failedCount == total {
		return fmt.Sprintf("全部失败 (0/%d)", total)
	}
	var parts []string
	if successCount > 0 {
		parts = append(parts, fmt.Sprintf("成功 %d", successCount))
	}
	if failedCount > 0 {
		parts = append(parts, fmt.Sprintf("失败 %d", failedCount))
	}
	if cancelledCount > 0 {
		parts = append(parts, fmt.Sprintf("取消 %d", cancelledCount))
	}
	return fmt.Sprintf("部分完成 (%s)", strings.Join(parts, " / "))
}

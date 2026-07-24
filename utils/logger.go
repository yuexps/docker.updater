package utils

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

type LogLevel string

const (
	LevelInfo  LogLevel = "INFO"
	LevelWarn  LogLevel = "WARN"
	LevelError LogLevel = "ERROR"
)

type LogEvent struct {
	Time      time.Time
	Level     LogLevel
	Container string
	Message   string
}

func (e LogEvent) Format() string {
	timeStr := e.Time.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("%s [%s] %s", timeStr, e.Level, e.Message)
}

func (e LogEvent) TaskLogFormat() string {
	timeStr := e.Time.Format("2006-01-02 15:04:05")
	return fmt.Sprintf("%s [%s] %s", timeStr, e.Level, e.Message)
}

type CentralLogger struct {
	mu           sync.RWMutex
	pkgVar       string
	isFnos       bool
	sysLogWriter *RollingFileWriter
	listeners    []func(event LogEvent)
}

var (
	GlobalLogger     *CentralLogger
	RollingLogger    *RollingFileWriter
	once             sync.Once
	pendingListeners []func(event LogEvent)
	pendingMu        sync.Mutex
)

type RollingFileWriter struct {
	mu         sync.Mutex
	filePath   string
	maxSize    int64
	maxBackups int
	file       *os.File
	size       int64
}

func NewRollingFileWriter(filePath string, maxSize int64, maxBackups int) *RollingFileWriter {
	return &RollingFileWriter{
		filePath:   filePath,
		maxSize:    maxSize,
		maxBackups: maxBackups,
	}
}

func (w *RollingFileWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	writeSize := int64(len(p))
	if w.file == nil {
		if err := w.openFile(); err != nil {
			return 0, err
		}
	}

	if w.size+writeSize > w.maxSize {
		_ = w.rotate()
	}

	n, err = w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *RollingFileWriter) openFile() error {
	dir := filepath.Dir(w.filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	f, err := os.OpenFile(w.filePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	w.file = f
	info, err := f.Stat()
	if err != nil {
		w.size = 0
	} else {
		w.size = info.Size()
	}
	return nil
}

func (w *RollingFileWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}

	dir := filepath.Dir(w.filePath)
	timestamp := time.Now().Format("20060102_150405")
	backupPath := filepath.Join(dir, fmt.Sprintf("info_%s.log", timestamp))

	if _, err := os.Stat(backupPath); err == nil {
		backupPath = filepath.Join(dir, fmt.Sprintf("info_%s_%d.log", timestamp, time.Now().UnixNano()))
	}

	if srcBytes, err := os.ReadFile(w.filePath); err == nil && len(srcBytes) > 0 {
		_ = os.WriteFile(backupPath, srcBytes, 0644)
	}

	_ = os.Truncate(w.filePath, 0)
	w.size = 0

	w.cleanOldBackups(dir)

	return w.openFile()
}

func (w *RollingFileWriter) cleanOldBackups(dir string) {
	files, err := filepath.Glob(filepath.Join(dir, "info_*.log"))
	if err != nil || len(files) <= w.maxBackups {
		return
	}

	type backupItem struct {
		path    string
		modTime time.Time
	}

	var list []backupItem
	for _, f := range files {
		info, err := os.Stat(f)
		if err == nil {
			list = append(list, backupItem{path: f, modTime: info.ModTime()})
		}
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].modTime.Before(list[j].modTime)
	})

	excess := len(list) - w.maxBackups
	for i := 0; i < excess; i++ {
		_ = os.Remove(list[i].path)
	}
}

func (w *RollingFileWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		err := w.file.Close()
		w.file = nil
		return err
	}
	return nil
}

func InitLogger(pkgVar string, isFnos bool) {
	once.Do(func() {
		logFilePath := filepath.Join(pkgVar, "info.log")

		RollingLogger = NewRollingFileWriter(logFilePath, 3*1024*1024, 3)

		if info, err := os.Stat(logFilePath); err == nil && info.Size() >= 3*1024*1024 {
			_ = RollingLogger.rotate()
		}

		pendingMu.Lock()
		GlobalLogger = &CentralLogger{
			pkgVar:       pkgVar,
			isFnos:       isFnos,
			sysLogWriter: RollingLogger,
			listeners:    pendingListeners,
		}
		pendingMu.Unlock()

		log.SetFlags(0)
		log.SetOutput(os.Stdout)
	})
}

func RegisterLogListener(fn func(event LogEvent)) {
	pendingMu.Lock()
	defer pendingMu.Unlock()

	if GlobalLogger == nil {
		pendingListeners = append(pendingListeners, fn)
		return
	}
	GlobalLogger.mu.Lock()
	defer GlobalLogger.mu.Unlock()
	GlobalLogger.listeners = append(GlobalLogger.listeners, fn)
}

func (l *CentralLogger) Dispatch(event LogEvent) {
	formattedLine := event.Format()

	if !l.isFnos || l.sysLogWriter == nil {
		fmt.Println(formattedLine)
	}

	if l.sysLogWriter != nil {
		_, _ = l.sysLogWriter.Write([]byte(formattedLine + "\n"))
	}

	if event.Container != "" {
		logsDir := filepath.Join(l.pkgVar, "logs")
		_ = os.MkdirAll(logsDir, 0755)
		taskLogPath := filepath.Join(logsDir, fmt.Sprintf("%s.log", event.Container))
		f, err := os.OpenFile(taskLogPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err == nil {
			_, _ = f.WriteString(event.TaskLogFormat() + "\n")
			_ = f.Close()
		}
	}

	l.mu.RLock()
	listenersCopy := make([]func(event LogEvent), len(l.listeners))
	copy(listenersCopy, l.listeners)
	l.mu.RUnlock()

	for _, listener := range listenersCopy {
		listener(event)
	}
}

func Log(level LogLevel, container string, format string, v ...interface{}) {
	msg := fmt.Sprintf(format, v...)
	event := LogEvent{
		Time:      time.Now(),
		Level:     level,
		Container: container,
		Message:   msg,
	}

	if GlobalLogger != nil {
		GlobalLogger.Dispatch(event)
	} else {
		fmt.Println(event.Format())
	}

	if level == LevelError && v == nil {
		// noop
	}
}

func LogInfo(format string, v ...interface{}) {
	Log(LevelInfo, "", format, v...)
}

func LogSuccess(format string, v ...interface{}) {
	Log(LevelInfo, "", format, v...)
}

func LogWarn(format string, v ...interface{}) {
	Log(LevelWarn, "", format, v...)
}

func LogWarning(format string, v ...interface{}) {
	Log(LevelWarn, "", format, v...)
}

func LogError(format string, v ...interface{}) {
	Log(LevelError, "", format, v...)
}

func LogFatal(format string, v ...interface{}) {
	Log(LevelError, "", format, v...)
	os.Exit(1)
}

func LogTaskInfo(container string, format string, v ...interface{}) {
	Log(LevelInfo, container, format, v...)
}

func LogTaskSuccess(container string, format string, v ...interface{}) {
	Log(LevelInfo, container, format, v...)
}

func LogTaskWarn(container string, format string, v ...interface{}) {
	Log(LevelWarn, container, format, v...)
}

func LogTaskWarning(container string, format string, v ...interface{}) {
	Log(LevelWarn, container, format, v...)
}

func LogTaskError(container string, format string, v ...interface{}) {
	Log(LevelError, container, format, v...)
}

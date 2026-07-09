package utils

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	// RollingLogger 存储全局滚动日志写入器实例。
	RollingLogger *RollingFileWriter
	extraWriters  []io.Writer
	mu            sync.RWMutex
)

// RollingFileWriter 实现轻量级日志滚动与切分。
type RollingFileWriter struct {
	mu         sync.Mutex
	filePath   string
	maxSize    int64
	maxBackups int
	file       *os.File
	size       int64
}

// NewRollingFileWriter 实例化滚动日志写入器。
func NewRollingFileWriter(filePath string, maxSize int64, maxBackups int) *RollingFileWriter {
	return &RollingFileWriter{
		filePath:   filePath,
		maxSize:    maxSize,
		maxBackups: maxBackups,
	}
}

// Write 向日志文件写入数据，在超过容量限制时触发滚动。
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
		if err := w.rotate(); err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "[WARNING] 日志回滚失败:", err.Error())
		}
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

	for i := w.maxBackups; i >= 1; i-- {
		src := w.filePath
		if i > 1 {
			src = fmt.Sprintf("%s.%d", w.filePath, i-1)
		}
		dst := fmt.Sprintf("%s.%d", w.filePath, i)
		if _, err := os.Stat(src); err == nil {
			_ = os.Remove(dst)
			_ = os.Rename(src, dst)
		}
	}

	return w.openFile()
}

// Close 关闭当前滚动日志文件的文件指针。
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

// InitLogger 初始化全局滚动日志，限制单文件大小并保留指定数目的历史备份。
func InitLogger(pkgVar string) {
	logFilePath := filepath.Join(pkgVar, "info.log")
	RollingLogger = NewRollingFileWriter(logFilePath, 10*1024*1024, 3)
	updateLoggerOutput()
}

// RegisterExtraWriter 注册额外的输出通道。
func RegisterExtraWriter(w io.Writer) {
	mu.Lock()
	defer mu.Unlock()
	extraWriters = append(extraWriters, w)
	updateLoggerOutputUnLocked()
}

// updateLoggerOutput 更新标准 log 库的输出目标。
func updateLoggerOutput() {
	mu.RLock()
	defer mu.RUnlock()
	updateLoggerOutputUnLocked()
}

// updateLoggerOutputUnLocked 在不加锁的情况下更新标准 log 库的输出目标。
func updateLoggerOutputUnLocked() {
	writers := []io.Writer{os.Stdout}
	if RollingLogger != nil {
		writers = append(writers, RollingLogger)
	}
	writers = append(writers, extraWriters...)
	log.SetOutput(io.MultiWriter(writers...))
}

// LogInfo 记录普通运行信息。
func LogInfo(format string, v ...interface{}) {
	log.Printf("[INFO] "+format, v...)
}

// LogWarning 记录运行警告信息。
func LogWarning(format string, v ...interface{}) {
	log.Printf("[WARNING] "+format, v...)
}

// LogError 记录运行错误信息。
func LogError(format string, v ...interface{}) {
	log.Printf("[ERROR] "+format, v...)
}

// LogSuccess 记录操作成功信息。
func LogSuccess(format string, v ...interface{}) {
	log.Printf("[SUCCESS] "+format, v...)
}

// LogFatal 记录致命错误信息并终止执行。
func LogFatal(format string, v ...interface{}) {
	log.Fatalf("[ERROR] "+format, v...)
}

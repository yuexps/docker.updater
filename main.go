package main

import (
	"bytes"
	"embed"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"

	"docker-updater/api"
	"docker-updater/db"
	"docker-updater/scheduler"
	"docker-updater/service"
	"docker-updater/utils"

	"github.com/gin-gonic/gin"
)

//go:embed frontend/dist/*
var embeddedWebFS embed.FS

func main() {
	// 1. 初始化统一日志工具与目录
	pkgVar := os.Getenv("TRIM_PKGVAR")
	if pkgVar == "" {
		pkgVar = "./data"
	}
	utils.InitLogger(pkgVar)

	broadcastWriter := &sysLogBroadcaster{inner: os.Stdout}
	utils.RegisterExtraWriter(broadcastWriter)
	gin.DefaultWriter = os.Stdout
	gin.DefaultErrorWriter = os.Stdout

	utils.LogInfo("正在启动 docker-updater...")

	// 2. 初始化数据库与表模型
	if err := db.InitDB(); err != nil {
		utils.LogFatal("数据库初始化失败: %s", err.Error())
	}

	// 3. 运行启动恢复，自愈断电中断的升级逻辑
	service.SelfHealInterruptedOperations()

	// 3.1 初始化后台升级排队队列管理器
	service.InitQueueManager()

	// 4. 加载定时检查调度
	scheduler.StartScheduler()

	// 4.1 启动全局 WebSocket Hub 监听协程
	go api.GlobalHub.Run()

	// 5. 初始化 HTTP 服务
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())

	// 注入前端静态文件包
	api.WebFS = embeddedWebFS
	api.InitRoutes(r)

	trimAppDest := os.Getenv("TRIM_APPDEST")
	if trimAppDest == "" {
		utils.LogFatal("缺失 TRIM_APPDEST 环境变量，程序终止")
	}

	_ = os.MkdirAll(trimAppDest, 0755)
	socketPath := filepath.Join(trimAppDest, "web.sock")
	_ = os.Remove(socketPath)

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		utils.LogFatal("无法监听 Unix Domain Socket %s: %s", socketPath, err.Error())
	}
	defer listener.Close()

	if err := os.Chmod(socketPath, 0666); err != nil {
		utils.LogWarning("无法配置文件权限: %s", err.Error())
	}

	utils.LogInfo("服务运行于 Unix Domain Socket 模式: %s", socketPath)
	if err := r.RunListener(listener); err != nil {
		utils.LogFatal("服务运行异常中断: %s", err.Error())
	}
}

// sysLogBroadcaster 拦截 log 包的逐行输出，在写入底层 Writer 的同时通过 WebSocket 实时广播给前端
type sysLogBroadcaster struct {
	inner io.Writer
	buf   []byte
	mu    sync.Mutex
}

func (w *sysLogBroadcaster) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n, err = w.inner.Write(p)
	// 逐字节扫描，提取完整行后广播
	for _, b := range p {
		if b == '\n' {
			line := string(bytes.TrimRight(w.buf, "\r\n"))
			if line != "" {
				api.GlobalHub.BroadcastSysLog(line)
			}
			w.buf = w.buf[:0]
		} else {
			w.buf = append(w.buf, b)
		}
	}
	return
}


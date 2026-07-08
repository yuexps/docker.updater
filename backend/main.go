package main

import (
	"bytes"
	"context"
	"embed"
	"encoding/json"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"

	"docker-updater/api"
	"docker-updater/db"
	"docker-updater/dockerclient"
	"docker-updater/scheduler"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/gin-gonic/gin"
)

//go:embed dist/*
var embeddedWebFS embed.FS

func main() {
	// 1. 日志输出至 stdout，文件落地由 fnpack/cmd/main Shell 脚本的 >> 重定向统一负责
	broadcastWriter := &sysLogBroadcaster{inner: os.Stdout}
	log.SetOutput(broadcastWriter)
	gin.DefaultWriter = os.Stdout
	gin.DefaultErrorWriter = os.Stdout

	log.Println("[INFO] 正在启动 docker-updater...")

	// 2. 初始化数据库与表模型
	if err := db.InitDB(); err != nil {
		log.Fatalf("[ERROR] 数据库初始化失败: %s\n", err.Error())
	}

	// 3. 运行启动恢复，自愈断电中断的升级逻辑
	recoverInterruptedOperations()

	// 3.1 初始化后台升级排队队列管理器
	dockerclient.InitQueueManager()

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

	// 6. 确定监听模式：飞牛 Unix Domain Socket 或 TCP 端口本地开发模式
	trimAppDest := os.Getenv("TRIM_APPDEST")
	if trimAppDest != "" {
		_ = os.MkdirAll(trimAppDest, 0755)
		socketPath := filepath.Join(trimAppDest, "web.sock")
		_ = os.Remove(socketPath) // 移除前一次残留套接字

		listener, err := net.Listen("unix", socketPath)
		if err != nil {
			log.Fatalf("[ERROR] 无法监听 Unix Socket %s: %s\n", socketPath, err.Error())
		}
		defer listener.Close()

		// 保证套接字文件的读写权限，供飞牛统一网关进程访问
		_ = os.Chmod(socketPath, 0666)
		log.Printf("[INFO] 服务已运行在无端口 Unix Socket 模式: %s\n", socketPath)
		if err := r.RunListener(listener); err != nil {
			log.Fatalf("[ERROR] 服务中断运行: %s\n", err.Error())
		}
	} else {
		// fallback 本地 TCP 测试模式，监听 9090
		port := "9090"
		log.Printf("[INFO] 未检测到飞牛环境，以开发模式启动 TCP 监听，端口: %s\n", port)
		if err := r.Run(":" + port); err != nil {
			log.Fatalf("[ERROR] 服务中断运行: %s\n", err.Error())
		}
	}
}

// sysLogBroadcaster 拦截 log 包的逐行输出，在写入底层 Writer 的同时通过 WebSocket 实时广播给前端
type sysLogBroadcaster struct {
	inner io.Writer
	buf   []byte
}

func (w *sysLogBroadcaster) Write(p []byte) (n int, err error) {
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

// recoverInterruptedOperations 扫描启动时残留的 _old 升级备份容器，自愈回滚损坏服务
func recoverInterruptedOperations() {
	cli, err := dockerclient.NewLocalClient()
	if err != nil {
		log.Printf("[WARNING] 恢复器无法连接 Docker 引擎: %s\n", err.Error())
		return
	}
	defer cli.Close()

	ctx := context.Background()
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		return
	}

	for _, c := range containers {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}
		if name == "" || !strings.HasSuffix(name, "_old") {
			continue
		}

		baseName := strings.TrimSuffix(name, "_old")
		log.Printf("[INFO] 发现未完成备份记录: %s\n", name)

		primaryRunning := false
		for _, o := range containers {
			oName := strings.TrimPrefix(o.Names[0], "/")
			if oName == baseName && o.State == "running" {
				primaryRunning = true
				break
			}
		}

		if primaryRunning {
			// 新版本已成功处于运行中，核对数据库看是否需要保留该备份
			var meta db.RollbackMetadata
			if err := db.DB.First(&meta, "container_name = ?", baseName).Error; err != nil {
				// 无有效元数据记录且未开启延迟备份保留，清除该残留孤儿备份
				log.Printf("[INFO] 自动清除多余孤儿备份容器: %s\n", name)
				_ = cli.ContainerRemove(ctx, c.ID, types.ContainerRemoveOptions{Force: true})
			}
			continue
		}

		// 主容器缺失或未运行，说明前次升级中途崩溃或闪退，自动进行自愈回退
		log.Printf("[INFO] 主容器缺失或非运行中，启动自愈回退恢复: %s\n", baseName)
		_ = cli.ContainerRemove(ctx, baseName, types.ContainerRemoveOptions{Force: true}) // 移除闪退的损坏新版

		err = cli.ContainerRename(ctx, c.ID, baseName)
		if err == nil {
			// 恢复重启策略并启动
			var meta db.RollbackMetadata
			var policy container.RestartPolicy
			if err := db.DB.First(&meta, "container_name = ?", baseName).Error; err == nil {
				_ = json.Unmarshal([]byte(meta.RestartPolicy), &policy)
				db.DB.Delete(&meta)
			} else {
				policy = container.RestartPolicy{Name: "unless-stopped"}
			}
			_, _ = cli.ContainerUpdate(ctx, c.ID, container.UpdateConfig{RestartPolicy: policy})
			_ = cli.ContainerStart(ctx, baseName, types.ContainerStartOptions{})
			log.Printf("[SUCCESS] 成功将容器 %s 从前次崩溃更新中复原\n", baseName)
		}
	}
}

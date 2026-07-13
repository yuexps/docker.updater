# AGENTS.md

## 项目概述
**Docker Updater** 是飞牛系统 (FNOS) 的 Docker 容器升级管理器（`.fpk` 格式第三方应用）。提供 Web GUI 实现宿主机容器版本检测、一键升级、回滚、备份和镜像管理。

## 技术栈
* **后端**: Go 1.26 + [Gin](https://github.com/gin-gonic/gin) + [GORM](https://gorm.io)
* **数据库**: SQLite (纯 Go 驱动 [glebarez/sqlite](https://github.com/glebarez/sqlite)，CGO-free)
* **Docker SDK**: [docker/docker/client](https://github.com/moby/moby/tree/master/client) (不使用 CLI)
* **通信**: WebSocket + SSE 实时流
* **定时任务**: [robfig/cron/v3](https://github.com/robfig/cron)
* **前端**: Vue 3 (Composition API, `<script setup>`) + [Naive UI](https://www.naiveui.com/) + Tailwind CSS v4
* **构建/打包**: Vite 8 + TS 5.7 + Vue Router 5 | fnpack (`.fpk`)

## 项目结构
* **[api/](api)**: Gin 路由与 WebSocket Hub。
* **[db/](db)**: 数据库连接初始化与 6 个模型定义 ([models.go](db/models.go))。
* **[dockerclient/](dockerclient)**: Docker API 交互（拉取、启动、检测、Tag 提取）。
* **[scheduler/](scheduler)**: 定时任务调度器。
* **[service/](service)**: 队列调度 ([queue.go](service/queue.go))、断电自愈 ([selfheal.go](service/selfheal.go))、仓库凭证 ([credentials.go](service/credentials.go)) 等业务逻辑。
* **[utils/](utils)**: 统一日志与网络辅助方法。
* **[frontend/](frontend)**: Vue 前端 SPA 源码。
* **[docs/](docs)**: 包含项目设计与规范文档。
  * **[specification.md](docs/specification.md)**: 飞牛 FNOS Docker 容器升级管理器后端技术规格书。
  * **[frontend_features.md](docs/frontend_features.md)**: 前端所有视图、组件、底层交互机制及业务逻辑功能总结文档。
* **[fnpack/](fnpack)**: `.fpk` 打包配置及应用静态资产。
* **[main.go](main.go)**: 入口程序，嵌有前端静态资产，支持 `--fnos` 参数启动模式。
* **[build.cmd](build.cmd)**: Windows 构建与 `.fpk` 打包脚本。
* **[build.sh](build.sh)**: Linux 构建与 `.fpk` 打包脚本。
* **[.github/workflows/release.yml](.github/workflows/release.yml)**: GitHub Actions 自动构建与 Pre-release 发布工作流。

## 关键架构与设计决策
1. **单二进制嵌入**: 后端通过 `go:embed` 挂载 `frontend/dist/*` 资产。
2. **启动与监听模式**: 支持飞牛模式与原生模式。飞牛模式（携带启动参数 `--fnos`）下强依赖 `TRIM_APPDEST` 环境变量，监听 Unix Domain Socket 进行网关代理；原生模式（无启动参数，默认）下监听 TCP 端口 `2293`，并将日志和配置文件路径强制设为可执行文件同目录下。
3. **路由统一**: 所有 API 及静态服务统归 `/app/docker-updater/` 组。
4. **单工排队**: 容器升级、回滚加入单工任务队列，规避并发争抢。
5. **探活与自愈**: 容器更新后休眠探活，失败则回退至 `{name}_backup_docker_updater` 旧容器并重启。
6. **无 Emoji与日志落盘**: 统一采用文本前缀禁用 emoji。全局日志由启动脚本重定向落盘，Go 程序仅向标准输出与 WebSocket 广播以防写重；启动时若 `info.log` 超过 10MB 则自动对半截断。
7. **CGO-free**: 禁止引入 CGO，采用纯 Go SQLite 驱动，支持交叉编译。
8. **无 CLI 依赖**: 所有 Docker 操作均通过 Docker SDK 交互，禁止调用命令行 `docker` 进程。
9. **文档优先**: 变更代码前，优先同步更新 [AGENTS.md](AGENTS.md) 及 [specification.md](docs/specification.md)。
10. **禁止自动构建**: AI 代理禁止自行运行构建或打包脚本（如 `build.cmd` 或 `build.sh`），如有构建需求应引导并交由用户手动执行。

## 构建命令
* **Windows 平台**:
  ```cmd
  build.cmd
  ```
* **Linux 平台**:
  ```bash
  chmod +x build.sh && ./build.sh
  ```

## 自动化构建与发布
项目配置了 GitHub Actions 自动化工作流：
* 当有代码推送（push）至 `main` 分支时，将自动执行构建与打包。
* 自动发布并覆盖名为 `pre-release` 的预发布版本，始终只保留最新的那一版构建产物（`docker.updater-x86.fpk` 与 `docker.updater-arm.fpk` 及其对应的原生 Linux 二进制 `docker-updater-linux-amd64` 与 `docker-updater-linux-arm64`）。

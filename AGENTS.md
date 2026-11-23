# AGENTS.md

## 项目概述
**Docker Updater** 是飞牛系统 (FNOS) 的 Docker 容器升级管理器（`.fpk` 格式第三方应用）。提供 Web GUI 实现宿主机容器版本检测、一键升级、回滚、备份和镜像管理。

## 技术栈
* **后端**: Go 1.27 + [Gin](https://github.com/gin-gonic/gin) + [GORM](https://gorm.io) + `gopkg.in/yaml.v3` (Compose YAML 解析)
* **数据库**: SQLite (纯 Go 驱动 [glebarez/sqlite](https://github.com/glebarez/sqlite)，CGO-free)
* **Docker SDK**: [docker/docker/client](https://github.com/moby/moby/tree/master/client) (不使用 CLI)
* **通信**: WebSocket + SSE 实时流
* **定时任务**: [robfig/cron/v3](https://github.com/robfig/cron)
* **前端**: Vue 3 (Composition API, `<script setup>`) + [Naive UI](https://www.naiveui.com/) + Tailwind CSS v4 + CodeMirror 6 (`@codemirror/lang-yaml`)
* **构建/打包**: Vite 8 + TS 5.7 + Vue Router 5 | fnpack (`.fpk`)

## 项目结构
* **[api/](api)**: Gin 路由与 WebSocket Hub。
* **[db/](db)**: 数据库连接初始化与模型定义 ([models.go](db/models.go))。
* **[dockerclient/](dockerclient)**: Docker API 交互（拉取、启动、检测、Tag 提取）。
* **[scheduler/](scheduler)**: 定时任务调度器。
* **[service/](service)**: 队列调度 ([queue.go](service/queue.go))、仓库凭证 ([credentials.go](service/credentials.go)) 等业务逻辑。
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
4. **单工排队**: 容器升级加入单工任务队列，规避并发争抢。
5. **探活与自愈**: 容器更新后执行 6 秒探活，若失败自动退标故障镜像并基于旧 Spec 秒级重建还原。
6. **无 Emoji与统一日志落盘**: 统一采用文本前缀禁用 emoji。后端禁止直接调用原生 log/fmt 打印日志，必须统一使用 `utils` 封装器 (`utils.LogInfo` / `LogWarning` / `LogError` / `LogSuccess` / `LogFatal`)。全局日志文件限制单文件最大 3MB，达到上限自动重命名为 `info_YYYYMMDD_HHMMSS.log` 并重新创建干净的 `info.log`，历史备份文件最多自动保留 3 份。
7. **CGO-free**: 禁止引入 CGO，采用纯 Go SQLite 驱动，支持交叉编译。
8. **无 CLI 依赖**: 所有 Docker 操作均通过 Docker SDK 交互，禁止调用命令行 `docker` 进程。
9. **文档优先**: 变更代码前，优先同步更新 [AGENTS.md](AGENTS.md) 及 [specification.md](docs/specification.md)。
10. **禁止自动构建**: AI 代理禁止自行运行构建或打包脚本（如 `build.cmd` 或 `build.sh`），如有构建需求应引导并交由用户手动执行。
11. **代码注释规范**: 代码注释必须严肃，禁止虚空名词；简洁高效、直击核心，无废话无多余赘述（例如：`// InitDB 初始化 SQLite 数据库`）。

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
* 手动通过 `workflow_dispatch` 触发。
* 触发后自动构建前端与交叉编译前后端单二进制。
* 自动打包 `docker.updater-x86.fpk` 与 `docker.updater-arm.fpk` 以及对应的原生 Linux 独立二进制（`docker-updater-linux-amd64` 与 `docker-updater-linux-arm64`）。
* 根据 `fnpack/manifest` 中的语义化版本自动创建 GitHub Release、自动生成 Commit 更新日志。
* 自动计算各平台安装包的 SHA256 与文件大小，并同步更新推送至 `yuexps/FnDepot` 第三方应用商店源（需配置 `GH_PAT` Secret）。

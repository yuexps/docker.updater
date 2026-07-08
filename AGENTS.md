# AGENTS.md

## 项目概述

**Docker Updater** 是飞牛系统 (FNOS) 的 Docker 容器升级管理器，以 `.fpk` 格式打包为第三方应用。提供 Web GUI 管理宿主机上所有 Docker 容器的版本检测、一键升级、回滚、备份和镜像管理。

## 技术栈

| 层级 | 技术 |
|------|------|
| 后端 | Go 1.26 + Gin + GORM |
| 数据库 | SQLite (glebarez/sqlite, CGO-free) |
| Docker SDK | docker/docker client |
| 实时通信 | WebSocket (gorilla/websocket) + SSE |
| 定时任务 | robfig/cron |
| 前端 | Vue 3 (Composition API, `<script setup>`) + Naive UI + Tailwind CSS v4 |
| 构建 | Vite 8 + TypeScript 5.7 + Vue Router 5 |
| 打包 | fnpack (.fpk) |

## 架构

```
backend/          # Go 后端
  main.go         # 入口，Unix Socket / TCP 自动切换
  api/            # Gin 路由 + WebSocket Hub
  db/             # SQLite 数据模型 (6 表)
  dockerclient/   # Docker 引擎交互、版本检测、升级/回滚、任务队列
  scheduler/      # 定时检查 + 过期备份清理
frontend/         # Vue 3 前端
  src/
    router/       # 7 个 SPA 路由
    views/        # Dashboard, Containers, Images, History, Tasks, Logs, Settings
    utils/        # WebSocket 客户端
fnpack/           # fnpack 打包配置 (manifest, cmd, config)
docs/             # 技术规格文档
build.cmd         # 一键构建脚本
```

### 关键架构决策

- **单二进制部署**: Go 后端通过 `//go:embed` 嵌入前端 dist 产物
- **生产环境**: Unix Domain Socket (`${TRIM_APPDEST}/web.sock`)，由飞牛统一网关代理
- **开发环境**: 自动回退 TCP `:9090`
- **路径前缀**: 所有请求统一在 `/app/docker-updater/` 下
- **全站无 emoji**: 日志标签使用 `[INFO]`, `[WARNING]`, `[ERROR]`, `[SUCCESS]`

## 构建

```cmd
build.cmd
```

流程：
1. `cd frontend && npm run build`
2. 同步 `frontend/dist/` → `backend/dist/`
3. 交叉编译 Go → Linux x86_64 (`GOOS=linux GOARCH=amd64`)
4. 输出到 `fnpack/app/bin/docker-updater`
5. `fnpack build` 打包 `.fpk`

## 设计规范

- UI 遵循 Apple 极简风格：主题色 `#0066cc`，大圆角 (18px)，胶囊按钮 (9999px)，白色卡片 + 浅米色背景
- 侧边栏导航布局
- 不使用 emoji
- Naive UI + Tailwind CSS Skill技能(.agents\skills)

## 开发约定

- 代码风格遵循各自语言的社区惯例 (Go: gofmt, Vue: ESLint + Prettier)
- 所有 Docker 操作通过 docker/docker client SDK，不调用 CLI
- API 响应统一 JSON 格式，升级/回滚操作通过 SSE 流式输出日志
- WebSocket 用于全局状态广播和实时日志推送

# 飞牛系统本地 Docker 容器升级管理器技术规格文档

本文档定义了重构后的本机 Docker 容器手动升级管理器的技术规范。重构采用 Go 作为后端语言，Vue 3 + Naive UI + Tailwind CSS v4 作为前端技术栈，专门适配飞牛系统 (FNOS) 环境与 Apple 极简设计美学，且在全站、后台、日志中不使用任何 emoji 表情。

---

## 1. 飞牛系统 (FNOS) 环境适配规范

程序在启动时从飞牛系统容器或宿主机环境中读取以下环境变量，并以此配置服务行为：

### 1.1 Unix Domain Socket 监听
- **环境变量**：`TRIM_APPDEST`
- **监听文件**：`${TRIM_APPDEST}/web.sock`
- **服务机制**：程序启动时，必须检查并删除可能残留的套接字文件，然后在此路径上创建 Unix Domain Socket 监听。监听到连接后，赋予 `0666` 读写权限，允许飞牛系统的统一反向代理网关访问。不开放任何 TCP 监听端口。

### 1.2 路径前缀过滤 (BaseURL)
- **前缀路径**：`/app/docker-updater`
- **前端配置**：Vite 编译时的 `base` 参数设置为 `/app/docker-updater/`，Vue Router 的基础 history 路径设置为 `/app/docker-updater/`。
- **后端配置**：所有的 Web 资源请求和 RESTful API 路由统一通过 Gin 路由组 `/app/docker-updater` 进行拦截和路由转发。其中对于 `/containers`、`/history`、`/images` 和 `/settings` 等 SPA 路由，后端负责拦截并直接返回前端静态包的 `index.html`，以防用户刷新页面时发生 404 错误。对于未匹配到该前缀的请求，一律返回 404。

### 1.3 日志管理
- **环境变量**：`TRIM_PKGVAR`
- **日志文件**：`${TRIM_PKGVAR}/info.log`
- **服务机制**：初始化时重定向 Go 的标准日志流、Gin 的访问日志和错误输出流至该文件中。系统在输出更新状态或拉取状态时，以文本格式（如 `[INFO]`、`[WARNING]`、`[ERROR]`、`[SUCCESS]`）作为行前缀，坚决不用任何 emoji 图标。

### 1.4 数据持久化 (SQLite 数据库)
- **数据库路径**：`${TRIM_PKGVAR}/data.db`
- **驱动选择**：使用纯 Go (CGO-free) 的 SQLite 驱动（如 `github.com/glebarez/sqlite`），使得程序可以交叉编译并在没有动态依赖的 Debian 12 系统上稳定运行。

---

## 2. 数据库设计 (SQLite Schemas)

数据库包含 5 张表，分别负责配置、待更新状态、延期计划、历史日志和回滚数据记录：

### 2.1 settings (系统配置表)
用于存储备份保留时间等全局选项：
- `key` (TEXT, PRIMARY KEY)：配置键。
- `value` (TEXT)：配置值。
- **常用键**：`backup_enabled` (是否开启更新前备份), `backup_hours` (备份保留时长，整数默认 24), `restart_stack` (升级后是否重启 Compose 栈内其他容器), `temp_mirrors` (临时镜像源加速列表，序列化 JSON 字符串存储)。

### 2.2 available_updates (可用更新表)
记录检测到有新版本的本地容器：
- `container_name` (TEXT, PRIMARY KEY)：容器名称。
- `image` (TEXT)：镜像名称。
- `local_digest` (TEXT)：本地镜像的 Digest。
- `remote_digest` (TEXT)：仓库的最新 Digest。
- `checked_at` (TEXT)：比对检查时间 (UTC ISO8601)。
- `compose_project` (TEXT, 可空)：Docker Compose 项目标签。

### 2.3 deferred_updates (延期升级表)
记录被用户手动设置为暂不更新的容器：
- `container_name` (TEXT, PRIMARY KEY)：容器名称。
- `until` (TEXT)：延迟截止日期 (格式为 YYYY-MM-DD)。

### 2.4 update_history (升级与回滚历史表)
记录最近的升级与回退历史：
- `id` (INTEGER, PRIMARY KEY AUTOINCREMENT)：主键。
- `container_name` (TEXT)：容器名称。
- `image` (TEXT)：升级/回滚时对应的镜像。
- `updated_at` (TEXT)：发生时间 (UTC ISO8601)。
- `status` (TEXT)：状态，例如 `success` (升级成功), `error: [错误描述]` (升级报错)。

### 2.5 rollbacks (备份回滚表)
记录已被重命名为 `{name}_old` 且处于保留期内的旧容器信息：
- `container_name` (TEXT, PRIMARY KEY)：容器原名称.
- `backed_up_at` (TEXT)：备份建立时间。
- `expires_at` (TEXT)：备份过期截止时间。
- `restart_policy` (TEXT)：原容器的重启策略 (序列化为 JSON 字符串存储)。

### 2.6 registry_credentials (私有仓库凭据配置表)
记录用于登录并拉取私有镜像仓库的域名与身份认证信息：
- `id` (INTEGER, PRIMARY KEY AUTOINCREMENT)：主键。
- `registry` (TEXT)：私有仓库域名或宿主机 IP 端口。
- `username` (TEXT)：用户名。
- `password` (TEXT)：明文存储的密码（依赖本地 SQLite 物理文件权限安全防护）。
- `created_at` (DATETIME)：创建时间。
- `updated_at` (DATETIME)：更新时间。

---

## 3. 后端 API 接口设计

所有请求均在前缀 `/app/docker-updater` 下进行：

### 3.1 获取整体状态
- **端点**：`GET /app/docker-updater/api/status`
- **返回**：
  ```json
  {
    "containers": [
      {
        "name": "nginx-web",
        "image": "nginx:latest",
        "status": "update", // 可选: update, deferred, ok
        "defer_until": null,
        "checked_at": "2026-07-08T10:00:00Z",
        "has_rollback": true,
        "rollback_expires": "2026-07-09T10:00:00Z",
        "compose_project": "my-stack",
        "running": true
      }
    ],
    "last_check": "2026-07-08T10:00:00Z",
    "history": []
  }
  ```

### 3.2 触发本地更新比对
- **端点**：`POST /app/docker-updater/api/check`
- **说明**：后台异步启动镜像 Digest 校验，完成后写入 `available_updates`。

### 3.3 升级容器
- **端点**：`GET /app/docker-updater/api/update/:name`
- **交互机制**：API 用于向后台任务队列中异步发起升级注册。一旦任务进入排队与调度流程，其 Pull 进度、物理重建进度、防抖重启和结果日志将一律通过全局 **WebSocket 信道 (`logs:<name>`)** 实时广播至订阅的各前端客户端，同时向本地 `${TRIM_PKGVAR}/logs/${name}.log` 写入完整运行日志且不用任何 emoji。
- **重启回滚**：启动新容器后休眠 2 秒，检查状态。若状态不为 `running`，则自动执行回退：克隆恢复原备份容器 `{name}_old` 并重启，通过 WS 向日志订阅者分发 `[ROLLBACK SUCCESS] Restore original container`。

### 3.4 回滚容器
- **端点**：`GET /app/docker-updater/api/rollback/:name`
- **交互机制**：API 用于异步注册容器回滚任务。后台将备份的 `{name}_old` 物理恢复为原名并重新拉起。相关的回退流式执行日志同样通过全局 **WebSocket 信道 (`logs:<name>`)** 实时向前端订阅客户端进行流式推送。

### 3.5 清除备份
- **端点**：`DELETE /app/docker-updater/api/backup/:name`
- **说明**：直接删除宿主机上的 `{name}_old` 容器，释放存储。

### 3.6 设置延迟更新
- **端点**：`POST /app/docker-updater/api/defer/:name`
- **参数**：`{"days": 7}`
- **说明**：向 `deferred_updates` 表中写入对应的截止日期。

### 3.7 恢复延迟更新对比
- **端点**：`POST /app/docker-updater/api/undefer/:name`
- **说明**：删除 `deferred_updates` 表中该容器的记录，恢复正常的版本比对。

### 3.8 获取容器最新日志 (调试诊断)
- **端点**：`GET /app/docker-updater/api/container/:name/logs`
- **说明**：从 Docker 引擎中拉取指定容器最近的 stdout/stderr 最新控制台日志。

### 3.9 获取历史持久化日志
- **端点**：`GET /app/docker-updater/api/update-log/:name`
- **说明**：从本地磁盘中直接读取上一次升级或回退产生的 `${name}.log` 文本文件内容。

### 3.10 获取本地镜像列表
- **端点**：`GET /app/docker-updater/api/images`
- **说明**：获取宿主机上已下载的全部 Docker 镜像，包含体积大小、RepoTags（无 Tag 时标为 `<none>:<none>`）及创建时间。

### 3.11 删除本地镜像
- **端点**：`DELETE /app/docker-updater/api/image`
- **参数**：`?id=sha256:...` (query 参数)
- **说明**：通过 Docker 镜像 ID 执行物理强制删除。

### 3.12 一键清理虚悬镜像 (Prune)
- **端点**：`POST /app/docker-updater/api/images/prune`
- **说明**：修剪宿主机上所有未被运行或停止容器关联的悬空 (dangling=true) 镜像垃圾，返回释放的磁盘体积及删除数量。

### 3.13 获取系统全局配置
- **端点**：`GET /app/docker-updater/api/settings`
- **返回**：`{"backup_enabled": bool, "backup_hours": int, "restart_stack": bool}`

### 3.14 保存系统全局配置
- **端点**：`POST /app/docker-updater/api/settings`
- **参数**：`{"backup_enabled": bool, "backup_hours": int, "restart_stack": bool}`
- **说明**：更新并存入 SQLite 数据库，彻底精简移去了 notify_url 推送配置。

### 3.15 获取后台任务队列状态
- **端点**：`GET /app/docker-updater/api/tasks`
- **说明**：获取全局排队系统的当前状态。返回正在执行的活跃任务 (`active`) 以及在等待队列中的任务列表 (`queued`)。

### 3.16 取消排队中的任务
- **端点**：`POST /app/docker-updater/api/tasks/cancel/:name`
- **说明**：将指定容器的、尚处于等待排队（`waiting`）状态的任务从队列中安全移出，取消其升级。如果任务已经开始运行（`running`），则返回失败。

### 3.17 获取系统全局运行日志 (程序主日志)
- **端点**：`GET /app/docker-updater/api/system/logs`
- **说明**：从本地读取本程序守护进程自身的 `info.log` 全局运行日志文件，截取返回最新的最末尾 400 行用于前端的高效渲染。

### 3.18 清空系统全局运行日志
- **端点**：`DELETE /app/docker-updater/api/system/logs`
- **说明**：以截断写空（O_TRUNC）模式清除 `info.log` 日志文件的物理大小，防止由于后台写占用导致锁死或句柄失效。

### 3.19 清除容器操作日志
- **端点**：`DELETE /app/docker-updater/api/update-log/:name`
- **说明**：物理删除存储上针对指定容器升级/回滚产生的那一份 `${name}.log` 持久化操作日志文件，释放空间。

### 3.20 获取私有仓库凭证列表
- **端点**：`GET /app/docker-updater/api/registries`
- **说明**：获取当前数据库中已注册的私有镜像仓库凭据（密码已被脱敏遮蔽为 `******`）。

### 3.21 保存/修改私有仓库凭证
- **端点**：`POST /app/docker-updater/api/registries`
- **参数**：`{"id": 0, "registry": "registry.com", "username": "user", "password": "pwd"}`
- **说明**：新增或覆写保存私有仓的账户密码凭证信息。

### 3.22 删除私有仓库凭证
- **端点**：`DELETE /app/docker-updater/api/registries/:id`
- **说明**：根据物理主键 ID 删除对应的镜像仓库凭证。

### 3.23 WebSocket 实时长连接通信管道
- **端点**：`GET /app/docker-updater/api/ws`
- **说明**：提供持久化的双向长连接以替换轮询，客户端通过特定事件主题保持前端组件状态高同步：
  - **上行心跳消息**：`{"type": "ping"}`
  - **上行订阅日志主题**：`{"type": "subscribe", "target": "logs:<container_name>"}`
  - **上行注销日志主题**：`{"type": "unsubscribe", "target": "logs:<container_name>"}`
  - **下行全局状态广播**：
    ```json
    {
      "type": "status",
      "payload": {
        "containers": [...],
        "last_check": "...",
        "history": [...],
        "active": {},
        "queued": []
      }
    }
    ```
  - **下行实时流式日志单播**：
    ```json
    {
      "type": "log",
      "payload": {
        "container": "nginx",
        "task": "update",
        "message": "[PULL] Extracting fs layer"
      }
    }
    ```

### 3.24 只读获取宿主机全局镜像加速源
- **端点**：`GET /app/docker-updater/api/settings/system-mirrors`
- **说明**：只读尝试解析宿主机 `/etc/docker/daemon.json` 中的 `"registry-mirrors"` 数组。如果宿主机未配置、文件不存在或读取无权限，则静默返回空数组 `[]`。不提供改写接口，对宿主机配置无破坏。

---

# hxy-blog

基于 Go、React 和 MySQL 的个人博客，面向 2C2G 单机部署。

## 开发环境

- Go 1.25.0
- Node.js 20.19+ 或 22.12+
- npm 10+
- Docker Compose
- [Task 3.53.1](https://taskfile.dev/docs/installation)（MIT；仅用于本地与 CI，不进入生产镜像）

Linux 本地可按固定版本安装 Task；安装目录需加入 `PATH`：

```bash
sh -c "$(curl --location https://taskfile.dev/install.sh)" -- -d -b ~/.local/bin v3.53.1
task --version
```

## 快速开始

```bash
task install
cp .env.example .env
task dev-db
task dev-migrate
task dev-backend
task dev-web
```

Go API 默认监听 `http://localhost:8080`，Vite 默认监听 `http://localhost:5173`。前端开发服务器会将 `/api` 代理到 Go API。

MySQL 使用腾讯云服务器已有的 `mysql:8.0` 镜像 Digest。开发环境的数据库和 Web 端口只绑定 `127.0.0.1`，不会直接暴露到局域网或公网。

## 容器基线

```bash
# 校验 Compose 配置，不启动容器。
task container-config

# 构建 API 与 Web 镜像。
task container-build

# 使用 .env 构建并启动完整本地环境。
task container-up
```

Web 默认通过 `http://127.0.0.1:8080` 访问，并把 `/api` 请求转发到 Go API。`task container-up` 会在启动 API/Web 前先等待 MySQL 并执行 Goose up。生产环境由 CD 使用 GHCR 镜像 Digest 覆盖本地镜像名，不在服务器拉取源码或执行构建。

## 质量检查

```bash
task check
```

该命令依次运行 `lint`、`test`、`build`，覆盖 Go 格式、vet、测试与构建，以及前端 lint、测试和生产构建；检查不会改写源码或锁文件。运行 `task --list` 可查看其他开发和 CI 任务。

数据库 Schema 使用 Goose 迁移，不使用 GORM `AutoMigrate`。迁移编译在 API 镜像的 `/app/migrate` 中，CI 在 `task container-build` 后运行 `task ci:migration` 和 `task ci:smoke`，针对真实 MySQL 验证 `up`、重复 `up`、`down`、容器健康与路由；生产部署固定执行“备份成功后再 `up`”。

## 工程规范

从 [AGENTS.md](./AGENTS.md) 开始阅读。它会引导人和 AI 找到技术章程、代码规范、Git 规范、敏捷流程和架构决策。

# hxy-blog

基于 Go、React 和 MySQL 的个人博客，面向 2C2G 单机部署。

## 开发环境

- Go 1.25.0
- Node.js 20.19+ 或 22.12+
- npm 10+
- Docker Compose

## 快速开始

```bash
make setup
cp .env.example .env
make dev-db
make dev-backend
make dev-web
```

Go API 默认监听 `http://localhost:8080`，Vite 默认监听 `http://localhost:5173`。前端开发服务器会将 `/api` 代理到 Go API。

MySQL 使用腾讯云服务器已有的 `mysql:8.0` 镜像 Digest。开发环境的数据库和 Web 端口只绑定 `127.0.0.1`，不会直接暴露到局域网或公网。

## 容器基线

```bash
# 校验 Compose 配置，不启动容器。
make container-config

# 构建 API 与 Web 镜像。
make container-build

# 使用 .env 构建并启动完整本地环境。
make container-up
```

Web 默认通过 `http://127.0.0.1:8080` 访问，并把 `/api` 请求转发到 Go API。生产环境由 CD 使用 GHCR 镜像 Digest 覆盖本地镜像名，不在服务器拉取源码或执行构建。

## 质量检查

```bash
make check
```

该命令会检查 Go 格式、运行 `go vet` 和测试，并完成前端 lint 与生产构建。

## 工程规范

从 [AGENTS.md](./AGENTS.md) 开始阅读。它会引导人和 AI 找到技术章程、代码规范、Git 规范、敏捷流程和架构决策。

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
make dev-backend
make dev-web
```

Go API 默认监听 `http://localhost:8080`，Vite 默认监听 `http://localhost:5173`。前端开发服务器会将 `/api` 代理到 Go API。

MySQL 环境将在确认现有镜像名称和标签后加入，不会自动拉取其他镜像。

## 质量检查

```bash
make check
```

该命令会检查 Go 格式、运行 `go vet` 和测试，并完成前端 lint 与生产构建。

## 工程规范

从 [AGENTS.md](./AGENTS.md) 开始阅读。它会引导人和 AI 找到技术章程、代码规范、Git 规范、敏捷流程和架构决策。

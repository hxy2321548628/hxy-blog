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

数据库 Schema 使用 Goose 迁移，不使用 GORM `AutoMigrate`。迁移编译在 API 镜像的 `/app/migrate` 中，CI 会针对真实 MySQL 验证 `up`、重复 `up` 和 `down`；生产部署固定执行“备份成功后再 `up`”。

## 工程规范

从 [AGENTS.md](./AGENTS.md) 开始阅读。它会引导人和 AI 找到技术章程、代码规范、Git 规范、敏捷流程和架构决策。

## 文档导航

| 想了解什么 | 读哪一份 |
| --- | --- |
| 项目约束与 AI 协作入口 | [AGENTS.md](./AGENTS.md) |
| 架构决策（为什么这样选） | [doc/architecture/](./doc/architecture/) |
| 生产部署与运维操作 | [生产部署运行手册](./doc/engineering/production-deployment.md) |
| Sprint 0 做了什么、为什么 | [Sprint 0 复盘](./doc/engineering/sprint-0-retrospective.md) |
| AI 项目工程化原理 | [AI 项目工程化深度解析](./doc/engineering/ai-engineering-deep-dive.md) |
| CI/CD 全链路原理 | [CI/CD 深度解析](./doc/engineering/ci-cd-deep-dive.md) |
| 云服务器与备份运维 | [云服务器与生产运维](./doc/engineering/cloud-server-operations.md) |
| 踩过的坑与常见问题 | [排错记录与常见问题](./doc/engineering/troubleshooting-and-faq.md) |
| 需求状态与 Sprint 计划 | [产品 Backlog](./doc/product/backlog.md) |

# Sprint 7 开发计划

- 日期：2026-10-10
- 状态：本地实施和容器验证完成，待 PR 与发布流水线验证
- Sprint 目标：以 Task 统一本地和 CI 的命令入口，按 `.claude/cicd.md` 收敛验证与发布流程。

## 目标与范围

- 用根目录 `Taskfile.yml` 取代 Makefile；保留现有开发、检查、Compose 和隔离 COS 测试能力，并提供规范要求的 `install`、`lint`、`test`、`build`、`check`。
- 本地与 GitHub Actions 固定相同 Task 版本；安装与 `check` 不改写源码或锁文件。
- PR 的 `check` 在无生产密钥的 GitHub 托管 Runner 上运行 `task install`、`task check`，并验证镜像、真实 MySQL 迁移和容器冒烟路径。
- 合并后的发布仅使用通过 `main` CI 的提交构建，记录 Commit、CI/Release 运行链接、镜像版本和 Digest；保留生产 Environment 审批、串行部署、健康检查和既有服务器回退入口。
- 同步更新当前使用说明、提交门禁和代码导读中的命令。历史验收记录保留当时使用的 `make` 命令。

## 非目标

- 不修改业务代码、数据库 Schema、API、生产服务器配置或运行时镜像基线。
- 不增加新的部署环境、发布触发方式、缓存层或并行构建优化。
- 不改写 Sprint 0–6 的历史验证证据。

## 故事与验收标准

### 故事 1：Task 替换本地命令

- Given 开发者安装项目固定版本的 Task，When 运行 `task --list`，Then 能找到原有 Make 目标的对应任务及其用途。
- Given 已按锁文件安装依赖，When 运行 `task check`，Then 依次完成静态检查、自动化测试、前端生产构建和部署配置检查，且工作树没有因检查而改变。
- Given 本地 `.env`，When 执行开发或容器任务，Then 环境变量仅传给所需命令；不在命令输出中泄露密钥。

### 故事 2：CI 门禁与发布

- Given Pull Request 指向 `main`，When `check` 运行，Then 固定 Task 和 Action 版本、执行 `task install` 与 `task check`，再验证镜像、迁移和容器；失败返回非零且不接触生产凭据。
- Given 已合并的 `main` 提交通过 CI，When Release 运行，Then 使用该提交生成镜像并记录完整 Commit、CI/Release 链接、不可变标签和 Digest；发布只使用这些 Digest。
- Given 生产批准或部署失败，When 部署作业继续或退出，Then 保持现有 Environment 门禁、串行执行、健康检查和服务器回退行为，不自动执行数据库 down。

## 假设、风险与依赖

- “替换”指删除 Makefile 并更新当前入口文档；历史验收记录保留原命令以维护证据真实性。
- Task 为开发与 CI 工具，不进入生产镜像或 2C2G 服务器；需在 README 中说明安装方式、版本与额外资源成本。
- Task 3.53.1 为 MIT 许可证，Runner 上的官方 `setup-task` Action 为 GPL-3.0；二者都不进入应用制品或生产服务器。
- 迁移和容器冒烟依赖 Docker；本地若无可用 Docker daemon，只能完成 Task 静态检查和非容器门禁，CI 仍需验证完整路径。
- 发布流程变动可能影响分支保护和生产发布；保持 `check` Job 名称、最小权限与现有生产审批。上线前由维护者复核权限、制品身份和回退步骤。

## 验证命令与检查

- `task --version`、`task --list`、`task --dry check`
- `task install`、`task check`
- `task container-build`、`task ci:migration`、`task ci:smoke`（需 Docker）
- 对 GitHub workflow 做 YAML/Action 静态检查，并在 Pull Request 与后续受保护 `main` 发布中核对实际权限和部署行为。
- 按仓库现行执行契约，在本次更改完成前运行 `make check`；移除 Makefile 后等价执行 `task check` 并记录结果。

## 本地验证记录

- 移除 Makefile 前运行 `make check` 通过；替换后 `task --list`、`task --dry check`、`task install` 和 `task check` 通过。Go 测试与 11 个前端测试文件中的 24 个测试均通过，前端生产构建完成。
- 本地 Task 版本为 3.54.0；Taskfile 最低版本与 CI 安装版本固定为稳定版 3.53.1。两个版本相邻，但固定版仍需由 GitHub Runner 实测。
- workflow YAML、第三方 Action 的完整 SHA、`check` 任务调用和 production Environment 门禁通过静态检查。
- 使用隔离的假 Docker/SSH 命令验证发布脚本拒绝无效 Commit SHA、传递两个 Digest 与运行链接，部署脚本拒绝错误环境、校验制品并清理临时凭据；迁移和冒烟脚本在失败路径均执行容器卷清理。
- 在授权访问本机 Docker daemon 后，以可达的 Go 模块镜像源构建 API/Web 镜像；`task ci:migration` 完成真实 MySQL 的 `up → 重复 up → 集成测试 → down → up`，`task ci:smoke` 验证 API 健康响应和 SPA 路由回退，两个独立 Compose 项目的容器与卷均在退出时清理。
- GitHub Runner 上的固定 Task 3.53.1、PR `check` 状态仍需 Pull Request 实测。
- Release 的 GHCR 推送、生产批准、健康检查和服务器回退未在本地触发，需在受保护 `main` 的后续发布中核对。

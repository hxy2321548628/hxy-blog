# hxy 的个人博客系统

## 项目目标

为个人内容发布提供一个简单、可维护、可在 2C2G 单机上运行的博客系统。

## 技术基线

- 后端：Go 1.25.0，模块化单体
- 前端：React + TypeScript + Vite
- 数据库：MySQL（版本以用户现有镜像为准）
- 交付：Docker Compose，单机部署
- 镜像：使用用户提供的现有镜像，未确认前不写死镜像名称

## 必读文件

- 技术章程：`.claude/constitution.md`
- CICD规范：`.claude/cicd.md`
- 代码规范：`.claude/code-style.md`
- Git 规范：`.claude/git-command.md`
- AI 开发流程：`doc/engineering/ai-coding-workflow.md`
- 架构决策：`doc/architecture/0001-initial-architecture.md`
- 媒体存储决策：`doc/architecture/0002-media-storage.md`
- 应用技术栈：`doc/architecture/0003-application-stack.md`
- 管理员鉴权：`doc/architecture/0004-authentication.md`
- 异地备份：`doc/architecture/0006-offsite-database-backup.md`
- 产品待办：`doc/product/backlog.md`

## 目录边界

```text
src/backend/   Go API
src/web/       React Web
doc/           架构、工程和产品文档
deploy/        部署配置
test/          跨边界测试（必要时再创建）
```

## AI 执行契约

1. 进入新 Sprint 前，必须先更新产品 Backlog 和 Sprint 计划/验收文档，写明目标、非目标、故事、验收标准、风险与验证命令；范围经用户确认后才能创建开发分支或修改代码。
2. 实施前先复述目标、非目标、假设和可验收标准。
3. 一次只处理一个可验收的小故事；不提前搭建未被需求驱动的抽象。
4. Bug 修复先补失败测试；新功能覆盖核心成功路径和关键失败路径。
5. 只修改与当前需求直接相关的文件；发现无关问题时记录，不顺手重构。
6. 添加依赖前说明必要性、许可证和资源成本；优先标准库。
7. 任务完成前运行 `task check`；若无法运行，明确说明未验证项和原因。
8. 涉及数据库 schema 的变更必须通过可回滚迁移完成，不得依赖手工改库。
9. 日志和测试数据中不得出现密码、Token、Cookie 或完整连接串。
10. 新增或修改代码时必须遵守 `.claude/code-style.md` 的学习友好注释规范；在关键逻辑和工程配置中用中文解释设计原因，并确保注释随行为同步更新。

## 常用命令

```bash
task dev-db       # 使用已确认的 MySQL 8.0 镜像启动本地数据库
task dev-backend  # 启动 Go API
task dev-web      # 启动 React
task test         # 运行测试
task check        # 提交前完整检查
```

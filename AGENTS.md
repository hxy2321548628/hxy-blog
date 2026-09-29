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
- 代码规范：`.claude/code-style.md`
- Git 规范：`.claude/git-command.md`
- AI 开发流程：`doc/engineering/ai-coding-workflow.md`
- 架构决策：`doc/architecture/0001-initial-architecture.md`
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

1. 实施前先复述目标、非目标、假设和可验收标准。
2. 一次只处理一个可验收的小故事；不提前搭建未被需求驱动的抽象。
3. Bug 修复先补失败测试；新功能覆盖核心成功路径和关键失败路径。
4. 只修改与当前需求直接相关的文件；发现无关问题时记录，不顺手重构。
5. 添加依赖前说明必要性、许可证和资源成本；优先标准库。
6. 任务完成前运行 `make check`；若无法运行，明确说明未验证项和原因。
7. 涉及数据库 schema 的变更必须通过可回滚迁移完成，不得依赖手工改库。
8. 日志和测试数据中不得出现密码、Token、Cookie 或完整连接串。

## 常用命令

```bash
make dev-db       # 启动本地 MySQL（镜像确认后启用）
make dev-backend  # 启动 Go API
make dev-web      # 启动 React
make test         # 运行测试
make check        # 提交前完整检查
```

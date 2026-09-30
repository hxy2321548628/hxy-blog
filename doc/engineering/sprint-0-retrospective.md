# Sprint 0 复盘：从零到可回滚的生产闭环

本文是 Sprint 0 的完整复盘：时间线、关键转折、决策全表、教训和 Sprint 1 的进入条件。如果你想先看原理，请读同目录下的三篇深度文档；本文负责回答“**我们到底做了什么，为什么这样做**”。

- 深度文档：[`AI 项目工程化深度解析`](./ai-engineering-deep-dive.md)、[`CI/CD 深度解析`](./ci-cd-deep-dive.md)、[`云服务器与生产运维`](./cloud-server-operations.md)
- 原始证据：[`Sprint 0 验收记录`](./sprint-0-acceptance.md)、[`产品 Backlog`](../product/backlog.md)
- 架构决策：[`doc/architecture/`](../architecture/)

## 1. Sprint 0 的目标

> 建立一条**人与 AI 都能重复执行**的开发闭环。

这句话拆开是三件事：

1. **可重复**：任何人（或 AI）在任何时间，用同样命令得到同样结果。
2. **有边界**：什么能做、什么不能做，写在文件里而不是记在脑子里。
3. **可回滚**：发布失败能自动回到上一个可用版本，数据能恢复。

Sprint 0 **不开发任何业务功能**。文章、鉴权、图片上传全部留给 Sprint 1。理由很实际：如果数据库迁移和集成测试没有稳定运行环境，业务开发只会放大返工。

## 2. 交付时间线（对应真实 Git 提交）

| 阶段 | 提交 | 交付内容 | 验证方式 |
| --- | --- | --- | --- |
| 1. 工程基线 | `9e9b65e` | `AGENTS.md`、章程、代码/Git 规范、AI 工作流、初始 ADR、Backlog、行走骨架、Makefile、初始 CI | `make check` |
| 2. CI 门禁 | `7ea3119` | 强化 `main` 分支集成门禁（PR #1） | 远程保护规则实测生效 |
| 3. 视觉设计 | `679596b` | 交互式视觉设计手册 `visual-design-manual.html`（PR #2） | 桌面/移动/深色模式渲染检查 |
| 4. 媒体存储 ADR | `1591145` | ADR-0002：腾讯云 COS、Go API 中转上传（PR #3） | 文档评审 |
| 5. 应用栈与鉴权 | `677f38f` | ADR-0003 应用技术栈、ADR-0004 管理员鉴权（PR #4） | 文档评审 |
| 6. CD 架构 | `afa4ea9` | ADR-0005：GHCR + 受限 SSH + 自动回滚（PR #5） | 文档评审 |
| 7. 容器基线 | `a6eb7f3` | API/Web Dockerfile、`.dockerignore`、本地 Compose、Nginx（PR #6） | 本地构建 + 容器冒烟 |
| 8. 镜像发布 | `ad31df1` | GHCR 版本化镜像、Commit SHA 标签与 Digest（PR #7） | 首次 Release 成功 |
| 9. 受限部署入口 | `fc700a5` | `hxy-deploy` 用户、`/opt/hxy-blog`、固定部署脚本、人工审批（PR #8） | 首次受保护生产部署 |
| — | 演练 | 受控自动回滚演练（无代码变更） | 注入失败版本 → 自动恢复上一版本 |
| 10. 备份与迁移门禁 | `8aa51a6` | Goose 迁移验证、部署前备份、隔离恢复演练（PR #9） | `tables=1`、`goose_version=1` |
| 11. 异地备份 | `c0e704c` | COS 上传/回读/取回、systemd 定时器、CAM 最小权限（PR #10） | 逐字节一致 + 隔离恢复 |
| 12. 验收收口 | `d536c6c` | 验收记录、Backlog 同步、ADR 参数关闭（PR #11） | 全链路复验 |

功能基线：`c0e704c6bdba41d5a1e1e63e6c97089e8847e0bf`（验收日期 2026-09-30）。

## 3. 三个关键转折

### 转折一：AI 越权决策被叫停

**发生了什么**：AI 自行选定了 Go 标准库、`sqlc`、GitHub Actions，并直接提交到 `main`。

**为什么是问题**：技术选型是**不可逆成本最高**的决策之一，却由 AI 单方面做出，且没有留下“这是待确认”的标记。同时，提交直接落在受保护本应保护、但当时尚未保护的分支上。

**怎么纠正**：

1. 该提交被追认为**待评审的工程草案**，不继续业务开发。
2. 重排顺序：产品范围 → 技术选型（ADR）→ 规范 → 分支保护 → CI → CD → 才能开始 Sprint 1。
3. 在 AI 工作流中加入“开工前必须复述目标/非目标/假设/验证方法”的契约。

**留下的制度**：技术分叉必须给选项、列代价、请求确认、写 ADR。

### 转折二：私有仓库无法强制保护 `main`

**发生了什么**：配置分支保护时 GitHub 返回 403——免费套餐的私有仓库不支持。

**为什么重要**：这直接决定“禁止在 main 上开发”是**规则**还是**愿望**。

**怎么决策**：把仓库改为公开，换取真正的服务端强制（必须 PR、必须过 `check`、禁止强推和删除、管理员同样受约束）。

**留下的制度**：约束要有服务端强制；本地 Hook 只作辅助。

### 转折三：只用“发布成功”不能验收 CD

**发生了什么**：所有发布都是绿的，但回滚代码路径从未执行过。

**为什么是问题**：ADR-0005 明确要求“故意部署健康检查失败的版本，确认自动恢复上一版本”。绿的成功路径**不能代替**失败路径验证。

**怎么解决**：执行受控回滚演练，确认部署返回失败、`previous.env` 被恢复、上一版本恢复健康、MySQL 卷未受影响。

**留下的制度**：CD 完成的定义包含“构建、推送、部署、健康检查、回滚、备份恢复**均有可重复证据**”。

## 4. 技术决策全表

| 决策点 | 最终选择 | 核心理由 | ADR |
| --- | --- | --- | --- |
| 架构形态 | 2C2G 单机模块化单体 | 个人博客体量，微服务是负债 | 0001 |
| 前端渲染 | React SPA（放弃 Next.js SSR） | 生产不需要额外跑 Node.js；SEO 弱点作为已接受取舍 | 0003 |
| 前端框架 | React + TypeScript + Vite | 构建快、开发体验好 | 0003 |
| 路由/状态 | React Router + Redux Toolkit | Redux 官方推荐 RTK，避免两套缓存（不引 TanStack Query） | 0003 |
| HTTP 客户端 | Axios + RTK Query 自定义 baseQuery | 统一请求、鉴权和缓存 | 0003 |
| 后端 Web | Gin | 路由分组、绑定、中间件开箱即用；`gin.Context` 只留在 Handler 层 | 0003 |
| 数据访问 | GORM | CRUD/事务效率高；显式定义查询，不在 Handler 里链式调用 | 0003 |
| Schema 迁移 | Goose（禁用生产 AutoMigrate） | 可审计、有版本历史、支持 up/down | 0003 |
| 鉴权 | 短期 Access JWT（内存）+ 可轮换 Refresh Token（HttpOnly Cookie） | JWT 做 API 鉴权，同时具备可撤销能力；令牌不进 localStorage | 0004 |
| 密码哈希 | Argon2id | OWASP 当前优先推荐 | 0004 |
| 媒体存储 | 腾讯云 COS + Go API 中转上传 | 服务器重装不丢图；应用可无状态替换；不做“本地卷再迁移” | 0002 |
| 镜像仓库 | 公开 GHCR + Commit SHA 标签 + Digest | 可追溯、可增量拉取、服务器无需长期 Token | 0005 |
| 部署方式 | 受限 SSH 调用固定脚本 | 不给 Actions 任意 root Shell 的能力 | 0005 |
| 审批 | GitHub `production` Environment 人工批准 | 高风险变更必须有人在场 | 0005 |
| 回滚策略 | 自动回滚应用镜像，不自动 `goose down` | 自动回退 Schema 可能毁数据 | 0005 |
| 异地备份 | COS + SSE-COS AES-256 + 90 天 | 脱离服务器、加密、免删 | 0006 |
| 备份凭据 | 专用 CAM 子用户（仅编程访问、最小权限、无删除权） | Lighthouse 不支持实例角色，无法使用免密钥的临时凭证 | 0005/0006 |

## 5. 数字与实测事实

这些数字是**实测**，不是估算，对同类 2C2G 项目有直接参考价值：

| 指标 | 实测值 |
| --- | --- |
| 生产内存占用 | MySQL 约 370–420 MiB、API 约 2–4 MiB、Web 约 3–7 MiB |
| 服务器公网监听 | 仅 SSH 22；Web 只绑 `127.0.0.1:8080` |
| 磁盘剩余 | 约 40 GB |
| Swap | 约 1.9 GiB |
| 服务器访问 `ghcr.io` | 正常（匿名拉取成功） |
| 服务器访问 `github.com` | 超时 |
| 备份保留 | 服务器本地 7 份；COS 90 天 |
| 恢复演练输出 | `tables=1 goose_version=1` |
| 目标机冒烟观察时长 | 约 9 小时，重启次数 0 |

## 6. 做得好的地方

1. **把“不能做什么”写清楚**：ADR 的“范围外”和“暂不进入 MVP”清单，有效阻止了提前抽象。
2. **本地与 CI 同源**：`make check` 一键复用，不存在两套逻辑。
3. **先备份后迁移、先迁移后切换**：顺序设计让失败都是“无副作用”的失败。
4. **上传后立即回读校验**：把“能上传”和“能恢复”在同一次任务里一起证明。
5. **验收记录附证据 ID**：CI 运行号 + 提交 SHA，可复现。
6. **密钥边界清晰**：GitHub 只存“怎么连服务器”，不存“应用怎么运行”。

## 7. 需要改进的地方

| 问题 | 表现 | 改进措施 |
| --- | --- | --- |
| AI 越权决策 | 单方面选型并提交 `main` | 强制“给选项 → 请求确认 → 写 ADR” |
| 验证不完整就宣布完成 | 容器基线只验了 Runner，没验目标机 | 增加目标机冒烟测试环节 |
| 文档状态滞后 | Backlog 与实际进度脱节 | Sprint 收口必须同步 Backlog 与验收记录 |
| 告警长期挂着 | Node 20 弃用、`go.sum` 缓存警告 | 不为消警告乱改版本，但要登记并排期 |
| 高风险操作缺乏统一清单 | 服务器初始化靠对话记忆 | 高风险操作保留执行清单与验证输出 |

## 8. Sprint 0 交付物地图

```text
约束层
├── AGENTS.md                         AI 与人的统一入口
├── .claude/constitution.md           技术章程
├── .claude/code-style.md             代码规范
├── .claude/git-command.md            Git 与提交规范
├── .github/ISSUE_TEMPLATE/*.yml      需求与缺陷模板
└── .github/pull_request_template.md  PR 模板

决策层（doc/architecture/）
├── 0001 初始架构        0002 媒体存储       0003 应用技术栈
└── 0004 管理员鉴权      0005 持续交付       0006 异地备份

流程层（doc/engineering/）
├── ai-coding-workflow.md             AI 协作流程与 DoR/DoD
├── production-deployment.md          生产运维手册
├── sprint-0-acceptance.md            Sprint 0 验收记录
└── 本目录的深度解析与复盘文档

实现层
├── src/backend/            Go API + migrate 二进制 + 迁移
├── src/web/                React SPA + Nginx 配置
├── deploy/compose.yaml            本地编排
├── deploy/compose.production.yaml 生产编排
├── deploy/scripts/*.sh            部署、备份、恢复、COS 同步
├── deploy/systemd/*               每日备份定时器
└── deploy/cam/cos-backup-policy.json  CAM 最小权限策略

流水线（.github/workflows/）
├── ci.yml        PR 与 main 门禁
└── release.yml   构建 → GHCR → 人工审批 → 受限 SSH 部署 → 健康检查 → 回滚
```

## 9. 进入 Sprint 1 的条件（已满足）

- [x] 所有 Sprint 0 Backlog 项完成。
- [x] 功能基线的 CI、Release、部署、回滚、备份、恢复都有可重复证据。
- [x] ADR 中影响 Sprint 1 的开放参数已关闭（JWT 算法与时效、Cookie/CORS、Argon2id 参数、媒体桶与图片限制）。
- [x] 生产运行修订与 `main` 一致，容器全部健康。
- [x] 收口文档通过 `make check`，且不含密码、Token、Cookie 或完整连接串。

未完成但**不阻塞**的事项：ICP 备案、正式 DNS、TLS、公网开放、公安备案。这些属于上线门禁，不影响 Sprint 1 的开发与验证（备案期通过 SSH 隧道访问）。

## 10. Sprint 1 计划（供对照）

Sprint 1 目标：访客可查看已发布文章，博主可在受保护入口创建并发布文章。

1. 初始数据库模型与可回滚 Goose 迁移。
2. 访客查看已发布文章列表。
3. 访客通过稳定 slug 查看文章详情。
4. 单管理员安全登录。
5. 管理员创建草稿并发布文章。
6. 管理员上传图片并插入文章。

每个故事都沿用 Sprint 0 建立的节奏：澄清 → 写失败测试 → 最小实现 → `make check` → PR → 演示 → 合并。

## 11. 五条可以带走的经验

1. **约束要写在文件里，并尽量变成服务端强制。** 靠记忆和自觉的规范会在第二次会话就失效。
2. **让 AI 区分“我推荐”和“已确认”。** 每个决策点明确请求确认，并留下 ADR。
3. **验证失败路径，而不只是成功路径。** 回滚、恢复、降级都必须演练，否则等于没有。
4. **顺序即安全。** 备份先于迁移、迁移先于切换、拉取失败不改状态——顺序设计本身就是防护。
5. **证据可复现才算完成。** “测试通过”不如“CI 运行 36664942430 通过”有用。

## 相关文档

- [`AI 项目工程化深度解析`](./ai-engineering-deep-dive.md)
- [`CI/CD 深度解析`](./ci-cd-deep-dive.md)
- [`云服务器与生产运维`](./cloud-server-operations.md)
- [`排错记录与常见问题`](./troubleshooting-and-faq.md)
- [`Sprint 0 验收记录`](./sprint-0-acceptance.md)

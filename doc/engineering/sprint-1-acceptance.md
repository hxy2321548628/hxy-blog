# Sprint 1 验收记录

- 验收日期：2026-10-01
- 功能基线：`1a0e51c0e26a1b8fa3adff5bb3fac87b6f095bfa`
- 验收结论：通过，最小可发布内容链路已闭合

## 验收范围

Sprint 1 的目标是让访客查看已发布文章，并让博主通过受保护入口创建、插入图片和发布文章。本次只验收单管理员最小链路，不引入多用户、评论、搜索、媒体库或图片处理。

| 能力 | 验收结果 | 证据 |
| --- | --- | --- |
| 数据模型与迁移 | 通过 | `posts`、`admins`、`refresh_tokens` 和 `media_assets` 均由带 up/down 的 Goose 迁移管理；MySQL 集成测试覆盖约束与持久化行为，开发库迁移版本为 4 |
| 访客文章列表 | 通过 | 公开分页接口只返回已发布文章，按发布时间稳定排序；React 列表页覆盖成功、空状态与失败状态 |
| 稳定 slug 详情 | 通过 | 公开详情接口按 slug 读取已发布正文，草稿与不存在的 slug 均返回 404；Markdown 渲染会过滤危险链接 |
| 单管理员登录 | 通过 | Argon2id 密码、短期访问令牌、刷新令牌轮换与重放撤销已实现；真实容器登录返回 200，退出返回 204 |
| 草稿创建与发布 | 通过 | 真实容器链路依次得到创建 201、草稿公开访问 404、发布 200、详情 200、列表 200，验收文章已精确清理 |
| 图片上传与插入 | 通过 | 隔离 COS 桶上传、HTTPS 回读、SHA-256 校验与清理测试通过；正式媒体链路得到上传 201、稳定域名读取 200、草稿创建 201、发布 200、访客详情 200 |

## 媒体验收细节

- 正式媒体桶、测试桶及各自 CAM 身份相互隔离，正式桶名称和地域符合 ADR-0002，未复用数据库备份桶。
- 管理端只通过受保护的 `POST /api/admin/media` 代理上传，浏览器不接触 COS 凭据。
- 正式上传返回 `https://media.hxy2333.site/media/...`，经 HTTPS 回读后的 SHA-256 与原始 PNG 完全一致。
- 数据库中的对象键、MIME、字节数、宽高、校验和及 `ready` 状态与上传结果一致。
- 验收正文使用带替代文本的 Markdown 图片语法；公开详情接口返回相同内容，前端组件测试覆盖安全图片渲染。
- 日志包含 `media object upload completed` 和 `media upload request completed`，记录对象键、字节数、耗时、结果和状态码，不包含密码、Token、Cookie 或 COS 密钥。
- 验收结束后，测试文章、媒体元数据和正式 COS 测试对象均按精确 ID 与对象键删除，数据库残留计数均为 0；隔离桶用例也自动删除测试对象。

## 安全与资源边界

- 图片仅允许 JPG、PNG 和 WebP，单文件不超过 10 MiB；限制单边尺寸、总像素并拒绝 EXIF。
- Nginx 与 Go API 同时限制请求体和超时；单实例串行上传，繁忙时返回 429，不把大文件完整保存在 Redux。
- 对象键由服务端随机生成并禁止覆盖；COS 成功而数据库失败时执行补偿删除。
- 无完整 COS 配置时应用仍可启动，但上传明确返回 503，不回退到服务器磁盘。
- 媒体 CAM 策略只允许正式桶 `media/*` 所需的对象级操作，不允许列举桶或账户级管理。

## 验证记录

```text
make test-media-cos
ok hxy-blog/backend/internal/media

真实内容链路：
login=200 create=201 draft_public=404 publish=200 detail=200 list=200 logout=204

真实媒体链路：
login=200 upload=201 https_read=200 create=201 publish=200 detail=200 logout=204

清理结果：
post_rows=0
media_rows=0
```

`make check` 覆盖 Go vet/单元测试、React 组件测试、ESLint、TypeScript、Vite 生产构建、开发与生产 Compose 配置、部署脚本语法和 CAM JSON 校验。收口提交前再次运行该命令。

## 回滚与后续边界

- 应用回滚只切换 API/Web 镜像，不删除已发布图片；既有稳定媒体 URL 不随应用版本变化。
- 媒体迁移仅在确认无不兼容数据时执行 Goose down；生产环境不得批量清空媒体桶。
- ICP 备案完成前继续遵守回环地址或 SSH 隧道限制，不因 Sprint 1 完成而开放公网服务。
- 搜索、评论、分类标签、独立媒体库、图片编辑与转换继续留在后续 Backlog。

## Definition of Done

- Sprint 1 六项 Backlog 均已完成并有自动化测试或可重复的真实链路证据。
- `make check` 与隔离 COS 往返测试通过，正式媒体稳定域名可通过 HTTPS 读取。
- 代码、API、迁移、部署文档和验收记录保持同步。
- 仓库与日志不包含密码、Token、Cookie、私钥或完整数据库连接串。

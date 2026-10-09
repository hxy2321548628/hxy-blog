# 代码阅读指南

本指南解释项目各入口如何协作。源码中的中文注释重点说明局部设计原因；这里提供跨文件的整体路径。建议先在仓库根目录运行 `make` 或 `make help` 查看可执行命令。

## 推荐阅读顺序

1. `Makefile`：理解本地开发与提交前门禁。
2. `src/backend/cmd/api` 与 `src/web/src`：理解最小应用请求链。
3. `src/backend/internal/migrations`：理解文章、管理员和刷新会话的初始数据约束。
4. `deploy/compose.yaml`、两个 Dockerfile 和 `src/web/nginx.conf`：理解容器如何组成运行环境。
5. `.github/workflows/ci.yml`：理解 PR 为什么能阻止有问题的代码合并。
6. `.github/workflows/release.yml` 与 `deploy/scripts/deploy.sh`：理解合并后如何发布和回滚。
7. `deploy/scripts/backup-db.sh`、`sync-backup-cos.sh` 和 `restore-drill.sh`：理解数据保护闭环。
8. `src/backend/internal/media` 与 `src/web/src/features/media`：理解图片校验、上传状态和 COS 一致性边界。

## 请求链

```text
浏览器
  ├─ /assets、页面路由 ──> Nginx ──> React 静态文件
  └─ /api/* ───────────> Nginx ──> Go API
                                      └─ 后续业务请求 ──> MySQL / COS
```

开发时 Vite 代替 Nginx 代理 `/api`；生产时 Nginx 使用 Compose 服务名 `api` 转发。前端始终请求相对路径，因此不需要在业务代码中判断环境。

访客文章列表与详情遵循 `React 页面 → RTK Query → Axios → Gin Handler → Post Service → GORM Repository → MySQL` 的单一路径。Handler 只处理协议参数和响应转换；Repository 负责“仅已发布”的公开边界，并分别提供按发布时间倒序的分页列表和按稳定 slug 查询详情。详情页的 Markdown 在浏览器端按需加载，经过 GFM 扩展与 HTML 白名单清洗后渲染。

公开页由 `App.tsx` 的 `public-app` 容器承载，`public.css` 只覆盖该容器内的瑞士风格令牌与布局；管理端继续使用 `styles.css` 的原有样式。`PostListPage` 保留 URL 分类和分页参数，`PostDetail` 让标题横跨阅读网格、把大纲置于正文前以适应手机阅读顺序。背景音乐仍由 `App` 在公开路由挂载，列表和详情切换不会重建播放器。

管理员登录从 `cmd/admin` 的一次性建号开始，密码只保存 Argon2id 哈希。浏览器登录后将 10 分钟 Access Token 仅保存在 Redux 内存中，7 天 Refresh Token 则由服务端写入 HttpOnly、SameSite=Strict Cookie。页面重载和 Access Token 过期时共用同一个刷新协调器；并发请求只轮换一次 Refresh Token，重放检测会在服务端撤销整个令牌家族。`/admin` 在前端提供登录状态引导，真正的数据写入权限仍必须由后端 Bearer Token 校验。

文章后台通过 `/api/admin/posts` 读写草稿，所有路由先验证 Access JWT。草稿允许正文为空，方便分步保存；发布动作则使用带状态和非空正文条件的原子更新，只有 `draft` 能转为 `published`。发布后文章不可再编辑，公开列表和详情缓存会同时失效。后台编辑器与访客详情共用经过清洗的 Markdown 渲染组件，避免预览和正式页面采用不同安全规则。

图片上传遵循 `React 编辑器 → Axios multipart → Nginx 流式代理 → 管理员鉴权 → Media Service → 腾讯云 COS + MySQL`。浏览器只把文件对象保存在组件内存，Redux 保存文件名、进度、错误和媒体结果等可序列化状态；上传成功后把带替代文本的 Markdown 插入当前光标位置。Nginx 接受包含 multipart 开销的 11 MiB 请求并关闭请求体缓冲，Go API 再以 10 MiB 文件上限、格式签名、完整解码、尺寸/像素和 EXIF 规则执行最终校验。

Media Service 使用不可预测对象键和 `Content-MD5` 写入 COS，并通过禁止覆盖请求头处理极小概率键冲突。COS 写入成功后才保存 `media_assets`；数据库失败时使用独立超时上下文补偿删除对象。正式正文只接收 `MEDIA_PUBLIC_BASE_URL` 生成的稳定 HTTPS URL，不保存 COS 默认域名。没有完整配置四个 `MEDIA_*` 变量时应用仍可启动，但上传明确返回可重试的 503，不回退到服务器磁盘。

## CI/CD 链

```text
Pull Request
  └─ CI check：格式、测试、构建、迁移、容器冒烟
       └─ 合并 main
            └─ main CI 再验证
                 └─ Release 发布 GHCR Digest
                      └─ 人工批准 production
                           └─ 受限 SSH → 备份 → Goose up → 切换 → 健康检查/回滚
```

Tag 方便人阅读，但可能移动；Digest 由镜像内容计算，不会移动，因此生产部署只接受 Digest。部署服务器不能通过受限 SSH 执行任意命令，也不会现场构建源码。

## 数据保护链

部署和每日 Timer 都复用同一备份脚本：`mysqldump` 一致性快照 → gzip → SHA-256 → COS 上传 → 立即回读复验。本地恢复演练只导入 `hxy_blog_restore_drill` 隔离库，退出时删除隔离库，不覆盖生产库。

## Shell 脚本常见写法

- `set -Eeuo pipefail`：命令失败、未定义变量或管道中任一命令失败时停止；`-E` 让错误陷阱在函数中继续生效。
- `umask 077`：新文件默认只有当前用户可访问，适合备份和密钥配置。
- `readonly`：启动后不允许意外改写路径、安全常量或参数。
- `trap ... EXIT`：无论成功还是失败都清理临时文件或隔离数据库。
- `flock -n`：用非阻塞文件锁拒绝并发部署、备份或恢复，而不是让任务无限等待。
- `exec`：用目标进程替换当前 Shell，使 systemd、SSH 或 Actions 得到真实退出码和信号行为。
- `realpath` 加目录前缀检查：先消除 `..` 和符号路径，再确认输入仍位于允许目录中。

## Make 常见写法

- `target: dependency`：执行目标前先执行依赖；`check` 因此可以组合多道独立门禁。
- `:=`：解析 Makefile 时立即计算；`?=`：只有外部没有提供值时才使用默认值。
- `$(VAR)`：Make 变量；Shell 命令中的 `$$` 会转换为单个 `$` 后再交给 Shell。
- 命令前的 `@`：执行但不回显命令本身，适合 `help` 这类只关心输出的目标。
- `.PHONY`：声明动作目标，避免同名文件让 Make 误判为“已经完成”。

## 不能直接添加注释的文件

- `package.json`：标准 JSON。`scripts` 分别提供开发、构建、Lint 和组合检查命令；依赖版本由 `package-lock.json` 精确锁定。
- `package-lock.json`、`go.sum`：包管理器生成的完整性清单，不手工修改。
- `deploy/cam/cos-backup-policy.json`、`deploy/cam/cos-media-policy.json`：腾讯云 CAM 标准 JSON，字段解释位于 [`deploy/cam/README.md`](../../deploy/cam/README.md)。
- `go.mod`：Go 模块清单。直接依赖位于第一组，工具解析出的间接依赖位于第二组。

## 阅读注释时的原则

注释解释“为什么”和边界条件，代码表达“怎么做”。如果注释与可执行代码冲突，以测试和代码为准，并在同一个 PR 中修正失真的注释。

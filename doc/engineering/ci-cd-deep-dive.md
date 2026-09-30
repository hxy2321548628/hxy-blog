# CI/CD 深度解析

本文逐层拆解本项目的持续集成（CI）与持续交付（CD）链路：从一次 PR 提交开始，到镜像构建、GHCR 发布、人工审批、受限 SSH 部署、数据库备份迁移、健康检查与自动回滚。目标是让你能独立复述并修改这条链路。

相关文件：

- 工作流：[`ci.yml`](../../.github/workflows/ci.yml)、[`release.yml`](../../.github/workflows/release.yml)
- 部署入口：[`deploy/scripts/deploy.sh`](../../deploy/scripts/deploy.sh)、[`deploy/scripts/ssh-entry.sh`](../../deploy/scripts/ssh-entry.sh)
- 生产编排：[`deploy/compose.production.yaml`](../../deploy/compose.production.yaml)
- 架构决策：[`ADR-0005`](../architecture/0005-continuous-delivery.md)

## 1. 先分清 CI 与 CD

| 概念 | 回答的问题 | 本项目对应 |
| --- | --- | --- |
| CI（持续集成） | 这次改动是否合格？ | PR 与 `main` 上运行 `check` |
| CD（持续交付/部署） | 合格的改动如何到达生产？ | Release 工作流构建、推送、部署 |

CI 的产出是**判断**，CD 的产出是**运行中的版本**。两者必须解耦：CI 不碰生产，CD 只接受已通过 CI 的提交。

本项目的核心原则：

> 生产镜像必须来自已通过 CI 的 `main` 提交，并且能被追溯到一个唯一的 Git Commit。

## 2. GitHub Actions 基本模型

| 术语 | 含义 | 本项目示例 |
| --- | --- | --- |
| Workflow | 一个 YAML 文件定义的一条流水线 | `CI`、`Release` |
| Event | 触发条件 | `pull_request`、`push`、`workflow_run` |
| Job | 一次运行中的独立任务，默认并行 | `check`、`publish`、`deploy` |
| Step | Job 内的顺序步骤 | `make check`、`docker build` |
| Runner | 执行 Job 的机器 | `ubuntu-latest`（GitHub 托管） |
| Secrets | 加密注入的敏感变量 | `DEPLOY_SSH_KEY` |
| Environment | 带保护规则的部署目标 | `production` |

两个容易被忽略但很重要的字段：

- `permissions`：默认应设为只读。写权限按 Job 最小化授予。
- `concurrency`：控制同一时间能否并发运行。CI 用 `cancel-in-progress: true` 省时长；发布用 `false`，避免留下半套镜像。

## 3. CI 工作流逐段讲解

触发条件：

```yaml
on:
  pull_request:
    branches: [main]
  push:
    branches: [main]
```

- `pull_request`：合并前的门禁。分支保护要求这个检查必须通过。
- `push`：合并后再检查一次 `main`，捕捉“合并结果本身有问题”的情况（例如两个 PR 各自通过、合并后冲突）。

并发与权限：

```yaml
concurrency:
  group: ci-${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

permissions:
  contents: read
```

`check` Job 的主要步骤：

| 步骤 | 作用 | 为什么需要 |
| --- | --- | --- |
| `actions/checkout@v4` | 拉取代码 | — |
| `setup-go`（`go-version-file: src/backend/go.mod`） | 安装 Go 并缓存 | 版本来源单一，避免与本地不一致 |
| `setup-node`（`cache: npm`） | 安装 Node 并缓存 | 加速 `npm ci` |
| `make check` | 格式、vet、单测、前端 lint/构建、Compose 配置校验 | 与本地完全同源的门禁 |
| `make container-build` | 构建两个生产镜像 | Dockerfile 也是交付代码，必须在 PR 阶段验证 |
| Goose 迁移验证 | 真实 MySQL 上跑 `up`、重复 `up`、`down` | 只验证 SQL 语法不够，要验证真实执行与幂等 |
| 容器冒烟测试 | 验证 `/healthz`、`/api/health`、SPA 回退 | 证明镜像能启动且路由正确 |
| 清理步骤（`if: always()`） | 删除容器与卷 | 保证失败时也不残留资源 |

几个值得学习的细节：

1. **迁移验证用真实 MySQL 容器**，而不是 SQLite 或纯文本比对：

   ```bash
   docker compose --env-file .env.example -f deploy/compose.yaml up -d mysql --wait
   docker compose ... run --rm --no-deps --entrypoint /app/migrate api up
   docker compose ... run --rm --no-deps --entrypoint /app/migrate api up   # 幂等
   ...
   docker compose ... run --rm --no-deps --entrypoint /app/migrate api down
   docker compose ... run --rm --no-deps --entrypoint /app/migrate api up
   ```

   它同时验证了：迁移能执行、可重复执行、可回滚、回滚后能再前进。

2. **迁移版本用 SQL 断言**，而不是相信日志：

   ```bash
   SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1;
   test "$version" = 1
   ```

3. **冒烟测试用固定网络与网络别名**，让 Nginx 的 `proxy_pass http://api:8080` 在 CI 中也能解析：

   ```bash
   docker network create hxy-blog-ci
   docker run --detach --name hxy-blog-api-ci --network hxy-blog-ci --network-alias api hxy-blog-api:local
   ```

4. **轮询等待而不是 `sleep` 固定秒数**，失败时打印容器日志便于定位。

## 4. 分支保护：CI 如何变成“不可绕过”

CI 通过 PR 运行，但如果没人强制看结果，CI 就只是装饰。真正让它生效的是 GitHub 的 Ruleset / Branch Protection：

| 规则 | 效果 |
| --- | --- |
| Require a pull request before merging | 禁止直接推 `main` |
| Require status checks to pass（`check`） | CI 不通过无法合并 |
| Require branches to be up to date | 检查必须基于最新 `main` |
| Dismiss stale approvals | 新提交使旧批准失效 |
| Require linear history | 禁止合并提交 |
| Block force pushes / deletions | 禁止强推和删除 `main` |
| Enforce for administrators | 管理员同样受约束 |

重要事实（Sprint 0 实际遇到）：

> GitHub 免费套餐的**私有**仓库不支持分支保护和 Rulesets，API 返回 403。要真正禁止直接推送 `main`，只能升级套餐或把仓库设为公开。仓库最终选择公开。

同时要理解：**本地 Git Hook 不能替代服务端保护**。Hook 可以在本地被绕过（`--no-verify`、直接换机器推送），服务端规则才是强约束。

## 5. Release 工作流的触发方式

```yaml
on:
  workflow_run:
    workflows: [CI]
    types: [completed]
    branches: [main]
```

为什么不用 `push`？

- `push` 触发会与 CI 并发竞速：CI 还没出结果，镜像已经在构建。
- `workflow_run` 能拿到上游 CI 的结论与**精确提交**：

```yaml
if: >-
  github.event.workflow_run.conclusion == 'success' &&
  github.event.workflow_run.event == 'push' &&
  github.event.workflow_run.head_branch == 'main'
```

三个条件缺一不可：上游必须成功、必须是 `push` 触发（排除 PR 与手工运行）、必须是 `main`。

另一个关键点：**检出上游的精确 SHA，而不是当前 `main` 的 HEAD**。

```yaml
- uses: actions/checkout@d23441a48e516b6c34aea4fa41551a30e30af803 # v6
  with:
    ref: ${{ github.event.workflow_run.head_sha }}
```

否则 CI 通过后又有新提交合入，就会构建出一份**没有经过 CI 的代码**，整个门禁形同虚设。注意这里还固定了 Action 的完整 Commit SHA，防止 `v6` 标签被上游移动（供应链漂移）。

Job 依赖与最小权限：

```yaml
jobs:
  publish:
    permissions:
      contents: read      # 读仓库
      packages: write     # 写镜像包
  deploy:
    needs: publish
    permissions:
      contents: read      # 部署 Job 不需要写权限
```

## 6. 镜像与 Docker 基础

### 6.1 多阶段构建

后端 [`Dockerfile`](../../src/backend/Dockerfile)：

```text
builder 阶段：golang:1.25-alpine
  复制 go.mod/go.sum → go mod download（利用层缓存）
  复制源码 → CGO_ENABLED=0 静态编译 api 与 migrate 两个二进制

runtime 阶段：alpine:3.22
  只复制 CA 证书和两个二进制
  USER 10001:10001（非 root）
  HEALTHCHECK 调用 /api/health
```

前端 [`Dockerfile`](../../src/web/Dockerfile)：

```text
builder 阶段：node:24-alpine → npm ci → npm run build
runtime 阶段：nginx:stable-alpine → 只保留 nginx.conf 与 dist 静态文件
  USER 101:101（非 root）
```

好处：最终镜像**不含编译器、包管理器和源码**，体积小、攻击面小。2C2G 机器不会因为镜像肥大而吃紧。

两个必须说明的设计点：

- **`CGO_ENABLED=0`**：产出静态链接二进制，才能在 `alpine`（musl）上运行。
- **非 root 运行 + `read_only: true` + `cap_drop: ALL` + `no-new-privileges`**：即使应用被攻破，也难以写入文件系统或提权。因为根文件系统只读，Nginx 的 `pid`、`temp_path` 全部改到 `/tmp`（由 `tmpfs` 提供）。

### 6.2 为什么用 Digest 而不是 Tag

| 形式 | 可变性 | 用途 |
| --- | --- | --- |
| `mysql:8.0` | 标签可被覆盖 | 只适合人读 |
| `mysql:8.0@sha256:7dcddc…` | 内容寻址，不可变 | 生产必须用这个 |
| `ghcr.io/…/hxy-blog-api:sha-<commit>` | 约定上不可变 | 人可读的版本标识 |
| `ghcr.io/…/hxy-blog-api@sha256:…` | 不可变 | 部署时的最终身份 |

Tag 是**指针**，Digest 是**内容的指纹**。同一个 Tag 在不同时间可能指向不同内容，所以生产部署以 Digest 为准。这就是 `deploy.sh` 强制校验 Digest 格式（`^sha256:[0-9a-f]{64}$`）的原因。

### 6.3 构建参数与镜像元数据

```bash
docker build \
  --platform linux/amd64 \
  --build-arg VERSION="sha-${HEAD_SHA}" \
  --build-arg REVISION="${HEAD_SHA}" \
  ...
```

`--platform linux/amd64` 显式指定目标架构，避免在 ARM 机器上做出跑不起来的镜像。

`VERSION` 与 `REVISION` 通过 `LABEL org.opencontainers.image.*` 写入镜像元数据，可反查“这个镜像对应哪个提交”。

关键安全约束：

> `ARG`/`ENV` 会被记录在镜像层里。数据库密码、JWT 密钥、COS 凭证**绝不能**通过构建参数注入。它们只在服务器运行时通过环境文件注入。

## 7. GHCR：镜像仓库

GHCR（GitHub Container Registry）可以理解为“镜像版的 GitHub”，地址形如：

```text
ghcr.io/<owner>/hxy-blog-api:sha-<commit>
ghcr.io/<owner>/hxy-blog-web:sha-<commit>
```

本项目选择**公开镜像**的理由：源码本身已经公开，镜像不含任何密钥；公开后生产服务器可以匿名拉取，不需要长期保存 `read:packages` Token，减少凭据管理面。

推送与读取 Digest：

```bash
printf '%s' "$GHCR_TOKEN" | docker login ghcr.io --username "$ACTOR" --password-stdin
docker push "${API_IMAGE}:${immutable_tag}"
docker buildx imagetools inspect "${API_IMAGE}:${immutable_tag}"   # 查看 Digest
```

每个镜像同时打两个标签：

- `sha-<commit>`：不可变版本，部署用它。
- `main`：浮动指针，方便人查看“当前主干”，**不作为部署依据**。

## 8. 部署阶段：从 GitHub 到服务器

### 8.1 为什么不用 Self-hosted Runner 或服务器构建

| 方案 | 结论 | 原因 |
| --- | --- | --- |
| GitHub 托管 Runner 构建 + GHCR | 采用 | 生产机零构建负担、镜像可追溯 |
| 本地构建 + SCP 上传 | 仅应急 | 依赖个人电脑、工作区可能不干净、不可审计 |
| CI 构建 + SCP 上传 | 不采用 | 每次传完整镜像包，无增量、无版本管理收益 |
| 服务器拉源码构建 | 不采用 | 吃 2C2G 的 CPU/内存，还会污染生产环境 |
| 生产机跑 Self-hosted Runner | 不采用 | 公开仓库的 PR 代码可能接触生产网络与凭据 |

补充实测事实：这台服务器访问 `ghcr.io` 正常（未登录返回预期的 401），但访问 `github.com` 页面会超时。因此“服务器拉镜像”可行，“服务器拉源码”不可行。

### 8.2 人工审批门禁

```yaml
deploy:
  environment:
    name: production
```

`production` Environment 提供：

- 部署前**必须人工批准**（GitHub UI 点击 Approve and deploy）。
- Secret 只在 `main` 触发且批准后可读，**PR 工作流读不到**。
- 可限制只有受保护分支能部署。

这是把“高风险变更需人工确认”从文档规则变成了流水线强制。

### 8.3 受限 SSH 入口

工作流最后一步只有一个命令：

```bash
ssh -o BatchMode=yes \
    -o IdentitiesOnly=yes \
    -o StrictHostKeyChecking=yes \
    -o UserKnownHostsFile="$HOME/.ssh/known_hosts" \
    -i "$HOME/.ssh/deploy_key" \
    -p "$DEPLOY_PORT" \
    "${DEPLOY_USER}@${DEPLOY_HOST}" \
    "deploy ${HEAD_SHA} ${API_DIGEST} ${WEB_DIGEST}"
```

四层收紧：

| 措施 | 作用 |
| --- | --- |
| `BatchMode=yes` | 禁止交互式提问，避免流水线挂起 |
| `StrictHostKeyChecking=yes` + 固定 `known_hosts` | 防中间人，禁止关闭主机校验 |
| `restrict,command="/usr/local/sbin/hxy-blog-ssh-entry"` 写进 `authorized_keys` | 该密钥**只能**执行这一个入口，拿不到 Shell |
| `ssh-entry.sh` 不做 `eval`，只接受固定格式字段 | 杜绝命令注入 |

[`ssh-entry.sh`](../../deploy/scripts/ssh-entry.sh) 的核心逻辑：

```bash
read -r action revision api_digest web_digest extra <<< "${SSH_ORIGINAL_COMMAND:-}"
[[ ${action:-} != deploy || -n ${extra:-} ]] && exit 64      # 只允许 deploy 且无多余字段
[[ ${revision} =~ ^[0-9a-f]{40}$ ]] || exit 64               # Commit 必须是 40 位十六进制
[[ ${api_digest} =~ ^sha256:[0-9a-f]{64}$ ]] || exit 64       # Digest 格式校验
exec sudo -n /usr/local/sbin/hxy-blog-deploy "${revision}" "${api_digest}" "${web_digest}"
```

注意 `sudo -n`：部署用户只能免密执行**指定的 root 脚本**，不能执行任意命令。

### 8.4 部署脚本的执行顺序

[`deploy.sh`](../../deploy/scripts/deploy.sh) 的顺序是这条链路的核心，每一步的顺序都有理由：

```text
1. 前置校验     root 身份、参数格式、文件存在与权限（runtime.env 不高于 600）
2. 单实例锁     flock -n 9   → 同一时间只允许一个部署
3. 生成候选版本  mktemp 写 .release.env.XXXXXX（不覆盖现行 release.env）
4. 拉取镜像     pull api web          → 失败不影响当前运行版本
5. 确保 MySQL   up -d mysql --wait    → 数据库必须先健康
6. 备份数据库   hxy-blog-db-backup    → 备份失败则停止发布
7. 执行迁移     /app/migrate up       → 迁移失败则停止发布，旧版本继续跑
8. 记录上一版本 release.env → previous.env
9. 切换版本     install candidate → release.env
10. 启动并等待  up -d --wait --wait-timeout 120
11. 健康检查    /healthz 与 /api/health
12. 失败回滚    恢复 previous.env 并重新 up
```

关键设计点：

- **备份在迁移之前**。否则迁移破坏了数据却没备份。
- **迁移在应用切换之前**。Schema 必须先就绪，新应用才能启动。
- **拉取失败不改 `release.env`**。失败是“无副作用”的。
- **回滚只回应用镜像，不回数据库**。自动 `goose down` 可能破坏新版本已写入的数据，必须人工评估。
- **首次部署失败时 `down --remove-orphans`**，但命名卷 `mysql_data` 保留，数据不丢。
- **只有成功路径才写 `previous.env`**，保证回滚基线始终是“上一个可用版本”。

校验片段示例：

```bash
[[ ${revision} =~ ^[0-9a-f]{40}$ ]] || fail 'Commit 格式无效'
if [[ -n $(find "${RUNTIME_ENV}" -maxdepth 0 -perm /077 -print) ]]; then
  fail "${RUNTIME_ENV} 权限必须不高于 600"
fi
```

这些都是**先校验再动作**，避免带着错误状态往下走。

### 8.5 release.env 与 previous.env 的版本记录

```text
/opt/hxy-blog/release.env     当前部署版本（root:root 600）
/opt/hxy-blog/previous.env    上一个可用版本（回滚基线）
/etc/hxy-blog/runtime.env     运行时密钥（root:root 600，GitHub 永远读不到）
```

切换版本的实现就是“换一份环境文件”，再由 Compose 用新的镜像 Digest 重建容器。简单、可审计、易回滚。

`release.env` 内容示例：

```text
API_IMAGE=ghcr.io/<owner>/hxy-blog-api@sha256:<64位>
WEB_IMAGE=ghcr.io/<owner>/hxy-blog-web@sha256:<64位>
APP_VERSION=sha-<commit>
APP_REVISION=<commit>
```

## 9. 自动回滚演练：如何证明回滚真的可用

“回滚代码写了”不等于“回滚能用”。ADR-0005 明确规定：**必须故意注入一个健康检查失败的版本，确认系统自动恢复上一版本**。

演练思路：

1. 先用正常流程部署一次，让 `previous.env` 有值。
2. 让候选版本故意无法通过健康检查（例如构造一个启动即失败或不响应 `/healthz` 的镜像摘要）。
3. 触发部署，观察脚本返回失败。
4. 验证 `previous.env` 被恢复、旧的容器重新健康、`mysql_data` 卷未受影响。

验收结论（Sprint 0 记录）：回滚路径真实执行，上一版本恢复，MySQL 容器与持久化卷未变化。

## 10. 本地复现这些检查

```bash
# 与 CI 完全一致的门禁
make check

# 构建两个镜像
make container-build

# 用示例环境文件校验 Compose 配置
make container-config
make container-production-config

# 本地完整起栈（需要先 cp .env.example .env）
make container-up
```

## 11. 排错速查

| 现象 | 常见原因 | 处理方向 |
| --- | --- | --- |
| PR 卡在 “Expected — Waiting for status” | 检查名与分支保护里配置的名字不一致 | 确认 Job 名仍为 `check` |
| Release 没被触发 | 上游 CI 失败，或触发源不是 `main` 的 push | 看 `workflow_run` 的 `if` 条件 |
| 部署报 “已有部署正在执行” | `flock` 锁被占用 | 确认没有残留部署进程 |
| 部署报 runtime.env 权限错误 | 权限高于 600 | `chmod 600 /etc/hxy-blog/runtime.env` |
| 容器健康但不通 | 检查业务端口与绑定地址 | 生产只绑定 `127.0.0.1:8080` |
| 服务器拉不到镜像 | GHCR 网络或镜像可见性 | 验证匿名拉取，必要时用 SCP 应急路径 |

## 相关文档

- [`云服务器与生产运维`](./cloud-server-operations.md)
- [`AI 项目工程化深度解析`](./ai-engineering-deep-dive.md)
- [`排错与常见问题`](./troubleshooting-and-faq.md)
- [`ADR-0005 生产持续交付架构`](../architecture/0005-continuous-delivery.md)

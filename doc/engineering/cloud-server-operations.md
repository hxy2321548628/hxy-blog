# 云服务器与生产运维

本文记录这台腾讯云服务器上，生产环境是如何组织的：目录、权限、用户、密钥、容器、备份和日常运维命令。所有内容都可以在零基础下照着执行。

相关文件：

- 生产编排：[`deploy/compose.production.yaml`](../../deploy/compose.production.yaml)
- 运维手册：[`production-deployment.md`](./production-deployment.md)
- 备份脚本：[`deploy/scripts/`](../../deploy/scripts)
- 定时任务：[`deploy/systemd/`](../../deploy/systemd)

## 1. 服务器基线

| 项目 | 实际情况 |
| --- | --- |
| 云厂商与产品 | 腾讯云**轻量应用服务器（Lighthouse）**，不是标准 CVM |
| 地域 | 南京 `ap-nanjing` |
| 规格 | 2 vCPU / 2 GiB 内存 |
| 系统 | Ubuntu 24.04 |
| Docker | 29.6.1 |
| Docker Compose | v5 系列（`docker compose` 子命令） |
| 磁盘剩余 | 约 40 GB |
| Swap | 约 1.9 GiB（内存只有 2 GiB，Swap 是必要的安全垫） |
| SSH 登录别名 | `ssh tencent` |
| 生产域名 | `hxy2333.site`（备案中，暂不解析） |
| 公网监听 | 仅 SSH（22）；Web 只监听 `127.0.0.1:8080` |

### 为什么“Lighthouse ≠ CVM”很重要

Sprint 0 计划用**实例角色（CAM Role）**给服务器授权访问 COS，这样服务器上不需要保存任何长期密钥。但实际操作时发现：

> 轻量应用服务器的控制台没有“绑定/修改实例角色”入口，因为实例角色是 CVM 的功能。

Lighthouse 支持的“CAM 管理”指的是**哪些用户能管理这台机器**，不等于**这台机器自己能获得角色身份**。所以最终改为方案 B：创建一个只用于备份的 CAM 子用户，密钥以 `root:root 0600` 保存在服务器。

> 经验：选服务器产品前先确认“是否支持实例角色”。这决定了你是用自动轮换的临时凭证，还是要自己保管长期密钥。

## 2. 网络与端口策略

```text
公网
 │
 ├── 22/tcp  SSH（唯一对外开放的服务）
 ├── 80/tcp  未监听（备案完成后再开）
 └── 443/tcp 未监听（备案完成后再开）

服务器本机
 └── 127.0.0.1:8080 → web 容器（Nginx）→ api 容器 → mysql 容器
```

绑定回环地址的写法：

```yaml
ports:
  - 127.0.0.1:${WEB_BIND_PORT:-8080}:8080
```

对比两种写法：

| 写法 | 谁能访问 | 适用场景 |
| --- | --- | --- |
| `8080:8080` | 公网 + 本机 | 不建议，等于把服务暴露出去 |
| `127.0.0.1:8080:8080` | 仅服务器本机 | 备案期、内部服务、数据库 |

这条规则同时用于开发环境（MySQL 只绑 `127.0.0.1:3306`）和生产环境（Web 只绑 `127.0.0.1:8080`）。

**注意**：端口绑定只是应用层不监听，真正拦公网的是腾讯云**防火墙/安全组**。两层都要正确，但“应用不监听”更彻底，因为它让服务根本不存在于公网。

### 备案期为什么不能走公网 IP

ICP 备案审核期间，腾讯云要求公网域名与**公网 IP 都不可访问**网站内容，否则可能被驳回。所以“先用 IP 顶一段时间”不是安全选项。

备案期验证手段：

```bash
# 服务器本机
curl http://127.0.0.1:8080/healthz
curl http://127.0.0.1:8080/api/health

# 开发机建立 SSH 隧道后，用浏览器访问
ssh -N -L 8080:127.0.0.1:8080 tencent
# 然后打开 http://127.0.0.1:8080
```

SSH 隧道原理：把本地 `8080` 端口的流量通过已加密的 SSH 连接转发到服务器的 `127.0.0.1:8080`。对服务器来说请求来自本机，对公网来说什么都没有暴露。

## 3. 目录与权限布局

```text
/opt/hxy-blog/                     部署目录（root 拥有）
├── compose.yaml                   生产编排文件
├── release.env                    当前版本：镜像 Digest + Commit（600）
└── previous.env                   上一个可用版本（600，回滚基线）

/etc/hxy-blog/
├── runtime.env                    运行时密钥：数据库密码等（root:root 600）
└── cos-backup.yaml                COSCLI 配置：CAM 子用户密钥（root:root 600）

/var/backups/hxy-blog/mysql/       即时备份目录（root:root 0700）
├── <时间戳>-sha-<commit12>.sql.gz
└── <时间戳>-sha-<commit12>.sql.gz.sha256

/usr/local/sbin/                   固定运维入口（root:root 755）
├── hxy-blog-ssh-entry             受限 SSH 入口（只接受 deploy 命令）
├── hxy-blog-deploy                root 部署脚本
├── hxy-blog-db-backup             备份 + COS 同步
├── hxy-blog-db-sync               COS 上传与回读校验
├── hxy-blog-db-fetch              从 COS 取回备份
├── hxy-blog-db-backup-current     定时任务调用的包装脚本
├── hxy-blog-db-restore-drill      隔离库恢复演练
└── hxy-blog-cos-configure         交互式写入 COS 密钥

/etc/systemd/system/
├── hxy-blog-db-backup.service
└── hxy-blog-db-backup.timer
```

### 三条权限设计原则

1. **按“谁需要读”划目录**：部署目录可被部署用户读取，但密钥目录 `/etc/hxy-blog` 只有 root 能读。
2. **密钥文件必须 600**：`deploy.sh` 会主动检查，权限过宽直接拒绝部署。
3. **运维脚本由 root 拥有**：部署用户能“调用”但不能“修改”，否则它就能改写脚本实现提权。

检查权限的实用命令：

```bash
stat -c '%U:%G %a %n' /etc/hxy-blog/runtime.env /etc/hxy-blog/cos-backup.yaml
# 期望：root:root 600 ...
```

## 4. 部署用户与受限 SSH

### 4.1 为什么不用日常登录账号部署

- 日常账号权限大，一旦其私钥泄露，影响面是整个服务器。
- 流水线用独立密钥，可以随时单独轮换或撤销，不影响人工登录。
- `hxy-deploy` 密码锁定，仅接受受限公钥，无法交互登录。

### 4.2 authorized_keys 的强制命令

```text
restrict,command="/usr/local/sbin/hxy-blog-ssh-entry" ssh-ed25519 <public-key>
```

| 选项 | 作用 |
| --- | --- |
| `restrict` | 关闭端口转发、Agent 转发、PTY 等所有额外能力 |
| `command=` | 无论客户端请求什么命令，都只执行这个脚本 |

因此 GitHub Actions 的那把私钥**拿到也开不了 Shell**，只能触发部署入口。

### 4.3 部署用户的 sudo 边界

`hxy-deploy` 只能免密执行 root 拥有的固定脚本：

```bash
sudo -n /usr/local/sbin/hxy-blog-deploy <commit> <api-digest> <web-digest>
```

`-n`（non-interactive）表示绝不允许输入密码，环境不满足就直接失败，避免流水线卡在密码提示上。

## 5. 生产容器编排要点

[`compose.production.yaml`](../../deploy/compose.production.yaml) 针对 2C2G 做了这些取舍：

### 5.1 内存上限（防止互相挤死）

| 服务 | `mem_limit` | 说明 |
| --- | --- | --- |
| MySQL | 768 MB | 加上 `--innodb-buffer-pool-size=256M` |
| API | 256 MB | Go 二进制实际占用极小（约 2–4 MB） |
| Web | 128 MB | Nginx 提供静态文件 |

2 GiB 内存要放下三个服务加系统，**必须设上限**。不设的话，某个服务内存暴涨会让 Linux OOM Killer 随机杀进程——通常是杀 MySQL，损失最大。

### 5.2 依赖与启动顺序

```yaml
api:
  depends_on:
    mysql:
      condition: service_healthy
web:
  depends_on:
    api:
      condition: service_healthy
```

`depends_on` + `condition: service_healthy` 表示“等对方的健康检查通过再启动”。MySQL 首次初始化需要几十秒，`start_period: 30s` 给它留出宽限期，避免被误判为不健康。

### 5.3 日志轮转

```yaml
x-logging: &default-logging
  driver: json-file
  options:
    max-size: 10m
    max-file: "3"
```

Docker 默认日志**不限制大小**。长期运行的服务可以把 40 GB 磁盘写满。这里限制每个容器最多 3×10 MB，是单机部署必须做的防守。

### 5.4 安全加固

```yaml
read_only: true
tmpfs:
  - /tmp
security_opt:
  - no-new-privileges:true
cap_drop:
  - ALL
```

| 选项 | 作用 |
| --- | --- |
| `read_only: true` | 根文件系统只读，应用被攻破也无法写入 |
| `tmpfs: /tmp` | 只读环境下仍需临时空间，用内存盘提供 |
| `no-new-privileges` | 禁止进程通过 setuid 提权 |
| `cap_drop: ALL` | 丢弃全部 Linux capabilities |

因为根文件系统只读，Web 容器的 Nginx 必须把 `pid` 和各类 `temp_path` 改到 `/tmp`，见 [`src/web/nginx.conf`](../../src/web/nginx.conf)。

### 5.5 MySQL 参数

```yaml
command:
  - --character-set-server=utf8mb4
  - --collation-server=utf8mb4_0900_ai_ci
  - --innodb-buffer-pool-size=268435456   # 256 MB
  - --max-connections=50
```

字符集必须显式设为 `utf8mb4`，否则中文和 emoji 会出问题。`max-connections=50` 对单机博客足够，防止连接数吃光内存。

## 6. 密钥与凭据管理

### 6.1 什么密钥放在哪里

| 凭据 | 存放位置 | GitHub 能读到吗 |
| --- | --- | --- |
| 部署 SSH 私钥 | GitHub `production` Environment Secret | 能（仅批准后） |
| 服务器 Host Key | GitHub `production` Environment Secret | 能（用于校验主机） |
| 数据库密码 | 服务器 `/etc/hxy-blog/runtime.env` | **不能** |
| COS SecretId/SecretKey | 服务器 `/etc/hxy-blog/cos-backup.yaml` | **不能** |
| JWT 签名密钥 | 服务器 `/etc/hxy-blog/runtime.env`（Sprint 1 落地） | **不能** |
| GHCR 拉取凭据 | 不需要（镜像公开） | — |

原则：**GitHub 只保存“如何连接服务器”，不保存“应用如何运行”。**

### 6.2 COS 凭据的安全写入方式

不要把密钥写在命令行参数里（会进 Shell 历史和 `ps` 输出），也不要用 `export`（会进环境）。正确做法是交互式隐藏输入：

```bash
sudo /usr/local/sbin/hxy-blog-cos-configure
# 依次提示：SecretId、SecretKey、再次确认 SecretKey
# SecretKey 输入时不显示字符，属于正常现象
```

脚本用 `read -s` 读取并写入 `root:root 0600` 的配置文件。验证时会检查两件事：

```bash
grep -Fq '    mode: SecretKey' /etc/hxy-blog/cos-backup.yaml          # 用的是密钥而非匿名
grep -Fq '    disableencryption: "false"' /etc/hxy-blog/cos-backup.yaml  # 本地密钥加密已启用
```

## 7. 备份与恢复

### 7.1 三个层次

| 层次 | 位置 | 保留 | 作用 |
| --- | --- | --- | --- |
| 部署前备份 | `./mysql/` | 7 份 | 每次发布会新建一份，迁移前的安全点 |
| 每日定时备份 | `./mysql/` | 7 份 | 覆盖“只改数据不改代码”的日常变化 |
| 异地备份 | COS `mysql/` | 90 天 | 服务器整机损坏时的最后防线 |

### 7.2 备份文件名的设计

```text
20260930T034110603421034Z-sha-c0e704c6bdba.sql.gz
└──────┬───────────────┘     └──────┬─────┘
   UTC 纳秒时间戳              前 12 位 Commit
```

用**纳秒**而不是秒，因为 Sprint 0 的演练发现过：秒级时间戳在同一秒内连续备份会撞名，导致 COS 端无法取回对应版本。这个坑通过给时间戳加纳秒修复。

`--forbid-overwrite` 进一步保证已归档的备份不会被静默覆盖。

### 7.3 备份命令的关键参数

```bash
mysqldump --single-transaction --quick --routines --triggers --events \
          --hex-blob --no-tablespaces --set-gtid-purged=OFF \
          --default-character-set=utf8mb4 "${MYSQL_DATABASE}"
```

| 参数 | 为什么需要 |
| --- | --- |
| `--single-transaction` | InnoDB 下不加锁获得一致性快照 |
| `--quick` | 逐行读取，避免大表全部载入内存（2 GiB 机器很关键） |
| `--routines --triggers --events` | 存储过程/触发器/事件也要备份 |
| `--hex-blob` | 二进制字段用十六进制导出，避免编码损坏 |
| `--no-tablespaces` | 避免需要 `PROCESS` 权限导致导入失败 |
| `--set-gtid-purged=OFF` | 导出的文件能从别处导入 |
| `--default-character-set=utf8mb4` | 中文安全 |

密码的处理方式值得单独说明：

```bash
docker exec "${mysql_container}" sh -ec '
  MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysqldump ...
'
```

密码只在**容器内部**通过环境变量传入，不出现在宿主机命令行参数、`ps` 输出或日志里。这是“日志与命令行不得含密码”规范的具体实现。

### 7.4 COS 异地备份流程

```text
本地备份完成
   ↓ 计算 SHA-256 校验文件
   ↓ 上传 .sql.gz 与 .sha256（SSE-COS AES-256，禁止覆盖）
   ↓ 立即从 COS 回读
   ↓ 校验远端文件与本地一致 + SHA-256 通过 + gzip 完整
   ↓ 只有以上全通过，才轮换服务器本地旧备份
```

**上传后立即回读**是这段设计里最有价值的一点：它把“能上传”和“能恢复”在同一次任务里一起验证了。很多备份事故是“文件静静躺在对象存储里，但从没验证过能否恢复”。

JSON 策略（[`cos-backup-policy.json`](../../deploy/cam/cos-backup-policy.json)）只授予：

```text
action:   PutObject, HeadObject, GetObject, 分片上传相关
resource: qcs::cos:ap-nanjing:uid/<APPID>:hxy-blog-backup-<APPID>/mysql/*
```

两个关键限制：

- 资源限定到 `mysql/*` 前缀，不能碰其他目录。
- **没有删除权限**。即使服务器被攻破，攻击者也删不掉历史备份（删除靠 COS 生命周期规则）。

### 7.5 定时备份

```ini
[Timer]
OnCalendar=*-*-* 03:30:00 Asia/Shanghai
Persistent=true
RandomizedDelaySec=10min
```

| 配置 | 含义 |
| --- | --- |
| `Asia/Shanghai` | 明确时区，避免服务器 UTC 导致凌晨误跑 |
| `Persistent=true` | 关机错过的任务在下次开机后补跑 |
| `RandomizedDelaySec=10min` | 在 0–10 分钟内随机抖动，避免整点资源争抢 |
| `Nice=10`、`IOSchedulingClass=best-effort` | 降低优先级，不影响线上服务 |

`hxy-blog-db-backup-current` 包装脚本有个好习惯：**不 `source` 环境文件**（那会把整份文件当代码执行），只用 `sed` 提取并严格校验 `APP_REVISION`：

```bash
mapfile -t revisions < <(sed -n 's/^APP_REVISION=//p' "${RELEASE_ENV}")
[[ ${#revisions[@]} -eq 1 && ${revisions[0]} =~ ^[0-9a-f]{40}$ ]] || fail '...'
```

### 7.6 恢复演练：为什么必须用隔离库

```bash
sudo /usr/local/sbin/hxy-blog-db-restore-drill \
  /var/backups/hxy-blog/mysql/<backup>.sql.gz
```

演练脚本的行为：

1. 校验 SHA-256 与 gzip 完整性。
2. 创建隔离库 `hxy_blog_restore_drill`（**不碰生产库**）。
3. 导入备份。
4. 输出恢复后的表数量与 Goose 版本。
5. 通过 `trap cleanup EXIT` 自动删除隔离库。

演练输出示例：

```text
数据库恢复演练成功：tables=1 goose_version=1
```

这个输出就是 Sprint 0 验收记录里那行证据的来源。

> **生产库的破坏性恢复不由任何自动脚本执行**，也不通过 GitHub 的部署密钥开放。它是人工故障处理流程。

## 8. 日常运维命令

### 8.1 查看状态

```bash
# 容器与健康状态
sudo docker compose --project-name hxy-blog-production \
  --env-file /etc/hxy-blog/runtime.env \
  --env-file /opt/hxy-blog/release.env \
  --file /opt/hxy-blog/compose.yaml ps

# 当前部署版本
sudo cat /opt/hxy-blog/release.env
sudo cat /opt/hxy-blog/previous.env

# 资源占用
sudo docker stats --no-stream
free -h
df -h /
```

### 8.2 查看日志

```bash
# 最近 200 行 API 日志
sudo docker logs --tail 200 hxy-blog-production-api-1

# 定时备份任务的历史与最近一次输出
sudo journalctl -u hxy-blog-db-backup.service --since '7 days ago' --no-pager

# 下一次定时备份时间
systemctl list-timers hxy-blog-db-backup.timer --no-pager
```

### 8.3 手动备份与从 COS 取回

```bash
# 手动触发每日备份任务
sudo systemctl start hxy-blog-db-backup.service

# 从 COS 取回指定备份（必须用规范文件名）
sudo /usr/local/sbin/hxy-blog-db-fetch \
  20260930T033000Z-sha-0123456789ab.sql.gz

# 对取回的备份做隔离恢复演练
sudo /usr/local/sbin/hxy-blog-db-restore-drill \
  /var/backups/hxy-blog/mysql/20260930T033000Z-sha-0123456789ab.sql.gz
```

**不要用 `coscli ls` 核验备份是否存在**——它会报 403，原因是 CAM 策略有意不授予 `ListObjects`（列举是桶级操作），这是最小权限的特性而非故障。核验请用上面需要 `GetObject` 的取回命令，或查看部署是否整体成功（备份步骤成功才会走到应用切换）。详见[排错记录第 10 条](./troubleshooting-and-faq.md)。

### 8.4 验证容器没有公网暴露

```bash
sudo ss -tlnp | grep -E ':80|:443|:8080'
# 期望：只有 127.0.0.1:8080，没有 0.0.0.0:80 / 0.0.0.0:443
```

## 9. 常见故障与处理

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| 部署立刻失败（root 相关） | 脚本未以 root 运行 | 确认经过 `sudo -n` 固定入口 |
| 部署报 runtime.env 权限错误 | 权限宽于 600 | `sudo chmod 600 /etc/hxy-blog/runtime.env` |
| MySQL 容器不健康 | 内存不足被 OOM | `docker stats`、`dmesg \| grep -i oom`，检查 `mem_limit` |
| 磁盘将满 | 日志或备份堆积 | 检查 `max-size` 生效、本地备份是否只留 7 份 |
| COS 上传校验失败 | 密钥错误或权限不足 | 核对 CAM 策略资源前缀与桶名 |
| 恢复演练失败 | 备份文件损坏或校验文件缺失 | 用更早的备份重试，检查本地保留策略 |

## 10. 上线清单（备案通过后执行）

当前**故意未做**的事情，以及备案通过后要补的步骤：

1. DNS 解析：`hxy2333.site`、`www`、`media` 指向目标地址。
2. 开放 80/443（同时改安全组和应用监听）。
3. 引入 TLS 反向代理（证书申请与自动续期）。
4. 公网 HTTPS 冒烟检查加入部署流程。
5. 公安备案（ICP 通过后按规定办理）。
6. 媒体桶与 `media.hxy2333.site` 按 ADR-0002 创建并核验。

> 这些动作**不改变** Sprint 0 已验证的镜像、部署和恢复模型，只是在最外层加一个 TLS 入口。

## 相关文档

- [`生产部署运行手册`](./production-deployment.md)（操作命令）
- [`CI/CD 深度解析`](./ci-cd-deep-dive.md)
- [`ADR-0005 生产持续交付架构`](../architecture/0005-continuous-delivery.md)
- [`ADR-0006 异地数据库备份`](../architecture/0006-offsite-database-backup.md)

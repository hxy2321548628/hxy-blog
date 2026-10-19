# 生产部署运行手册

## 目标

GitHub Actions 只能通过专用 SSH 密钥调用固定部署入口，不能获得服务器交互式 Shell。生产运行时密钥仅保存在服务器，不上传到 GitHub。

## 服务器边界

| 项目 | 约定 |
| --- | --- |
| 部署用户 | `hxy-deploy`，密码锁定，仅接受受限公钥 |
| 应用目录 | `/opt/hxy-blog`，root 拥有 |
| 运行时密钥 | `/etc/hxy-blog/runtime.env`，root:root `0600` |
| 固定 SSH 入口 | `/usr/local/sbin/hxy-blog-ssh-entry` |
| root 部署入口 | `/usr/local/sbin/hxy-blog-deploy` |
| root 备份入口 | `/usr/local/sbin/hxy-blog-db-backup` |
| root COS 同步入口 | `/usr/local/sbin/hxy-blog-db-sync` |
| root COS 取回入口 | `/usr/local/sbin/hxy-blog-db-fetch` |
| root 定时备份入口 | `/usr/local/sbin/hxy-blog-db-backup-current` |
| root 恢复演练入口 | `/usr/local/sbin/hxy-blog-db-restore-drill` |
| 即时备份目录 | `/var/backups/hxy-blog/mysql`，root:root `0700` |
| COSCLI 配置 | `/etc/hxy-blog/cos-backup.yaml`，root:root `0600` |
| 博客媒体凭据 | `/etc/hxy-blog/runtime.env` 中的 `MEDIA_*`，root:root `0600` |
| 生产端口 | 应用仅 `127.0.0.1:8080`；宿主机 Nginx 监听公网 80/443 |

部署公钥必须使用 `restrict` 和强制命令：

```text
restrict,command="/usr/local/sbin/hxy-blog-ssh-entry" ssh-ed25519 <public-key>
```

`hxy-deploy` 只允许免密 sudo 执行 root 拥有的部署脚本。部署脚本固定 GHCR 仓库名，并严格校验 Commit 和两个 `sha256` Digest，不能通过参数选择任意镜像或执行任意命令。

## GitHub production Environment

Environment 保存以下 Secret：

| 名称 | 含义 |
| --- | --- |
| `DEPLOY_HOST` | 腾讯云服务器地址 |
| `DEPLOY_PORT` | SSH 端口 |
| `DEPLOY_USER` | 固定为 `hxy-deploy` |
| `DEPLOY_SSH_KEY` | 专用部署私钥 |
| `DEPLOY_HOST_KEY` | 从服务器本机读取并固定的 ED25519 Host Key |

Environment 只允许受保护的 `main` 分支部署，并要求人工批准。Pull Request 工作流无法读取这些 Secret。

## 发布与回滚

1. `main` CI 成功后，Release 构建并推送两个不可变 GHCR 镜像。
2. GitHub 等待 `production` Environment 人工批准。
3. 服务器先拉取两个 Digest，不改动当前版本。
4. 确保 MySQL 健康，创建 `mysqldump` 逻辑备份并校验压缩文件。
5. 使用候选 API 镜像中与 Commit 一致的迁移二进制执行 Goose `up`。
6. 备份或迁移失败时停止发布，旧应用继续运行。
7. 保存当前 `release.env` 为 `previous.env`，再切换镜像。
8. 等待 Compose 健康并检查本机 Web/API 健康接口。
9. 失败时恢复 `previous.env`；首次部署失败则停止应用，但保留 MySQL 命名卷。

应用回滚不会自动执行数据库 down migration。Schema 必须采用先扩展、后收缩的兼容迁移；需要回退数据库时由管理员评估数据兼容性后单独处理。

## 公网 HTTPS 入口

应用容器仍然只监听 `127.0.0.1:8080`，公网入口由宿主机 Nginx 独立承担。入口与应用发布解耦：更换镜像不需要改 Nginx，回滚应用也不影响证书。

| 项目 | 约定 |
| --- | --- |
| 站点配置 | `/etc/nginx/conf.d/hxy-blog.conf`，由脚本整体渲染，不手工修改 |
| 配置模板 | `deploy/edge/hxy-blog-http.conf.tmpl`、`deploy/edge/hxy-blog-tls.conf.tmpl` |
| 安装脚本 | `deploy/scripts/install-edge.sh`，只在服务器上以 root 手工执行 |
| 公网自检 | `deploy/scripts/check-public-entry.sh`，在任意机器执行并绕过本机代理 |
| 证书目录 | `/etc/letsencrypt/live/hxy2333.site/` |
| ACME webroot | `/var/www/certbot` |
| 配置备份 | `/var/backups/hxy-blog/edge/`，root:root `0700` |
| 规范域名 | `hxy2333.site`；`www` 永久 301 到主域名 |

### 首次启用

1. 确认容器健康：

   ```bash
   curl -fsS http://127.0.0.1:8080/healthz
   curl -fsS http://127.0.0.1:8080/api/health
   ```

2. 在腾讯云轻量应用服务器控制台的防火墙放行 TCP 80 和 443。只在系统 `ufw`/`iptables` 放行不会生效。
3. 在服务器执行安装脚本。脚本先写只含 80 的配置完成 ACME 校验，再切换为 HTTPS 配置：

   ```bash
   sudo ./deploy/scripts/install-edge.sh --email <证书通知邮箱>
   ```

4. 在开发机执行公网自检，直到全部通过：

   ```bash
   task edge:check -- --expect-ip <服务器公网 IP>
   ```

   自检脚本默认使用 `curl --noproxy '*'`，避免把本机代理的错误页误判为站点响应。

5. 确认 `/etc/hxy-blog/runtime.env` 的来源包含 HTTPS 主域名，然后重启 `api` 容器：

   ```dotenv
   AUTH_ALLOWED_ORIGINS=https://hxy2333.site
   ```

   缺少该来源时后台登录会返回 `ORIGIN_NOT_ALLOWED`。

模板改动可以先用 `task edge:config` 在本地做离线渲染断言，不需要 root、Nginx 或网络。

### 证书续期

`certbot` 由系统 timer 自动续期，续期成功后通过 `--deploy-hook 'systemctl reload nginx'` 让新证书生效。

```bash
systemctl list-timers 'certbot*'
sudo certbot renew --dry-run
```

### 回滚

```bash
sudo rm -f /etc/nginx/conf.d/hxy-blog.conf
sudo nginx -t && sudo systemctl reload nginx
```

删除后站点回到只能通过回环地址或 SSH 隧道访问的状态，证书与 ACME 记录保留；重新执行安装脚本即可再次启用。安装脚本是幂等的，`nginx -t` 失败时会自动恢复上一份配置。

### 排障

- 浏览器返回无响应体的 `502 Bad Gateway`，且响应头包含 `Keep-Alive: timeout=4` 或 `Proxy-Connection` 时，通常是本机代理的错误页；用 `task edge:check` 绕过代理确认。
- 公网返回 `Connection refused`：宿主机没有监听 80/443，或腾讯云防火墙未放行。
- 首页 502 但 `http://127.0.0.1:8080/healthz` 正常：检查 `docker compose ps` 中 `api` 的健康状态与容器日志。

## 备份和恢复演练

- 每次部署在 Goose `up` 前创建一份 root 专属的 gzip 逻辑备份和 SHA-256 校验文件。
- 备份和校验文件会写入 COS `hxy-blog-backup-1497610660/mysql/`，上传后立即回读并重新校验。
- Lighthouse 使用仅启用编程访问的专用 CAM 子用户 `hxy-blog-backup`，SecretId/SecretKey 只保存在 root 可读的 COSCLI 配置中。
- 每日 03:30（Asia/Shanghai）由 systemd timer 补充执行一次备份，最多随机延迟 10 分钟。
- 单机目录只保留最近七份即时备份；被轮换的旧文件无法从服务器恢复。
- COS `mysql/` 前缀保留 90 天，专用子用户没有删除远端对象的权限。
- 恢复演练只导入固定隔离库 `hxy_blog_restore_drill`，完成校验后自动删除，不覆盖生产库。

管理员可在服务器执行：

```bash
sudo /usr/local/sbin/hxy-blog-db-restore-drill \
  /var/backups/hxy-blog/mysql/<backup>.sql.gz
```

演练会先验证 SHA-256 和 gzip 完整性，再输出恢复后的表数量和 Goose 版本。生产库的破坏性恢复不通过 GitHub 部署密钥开放，也不在无人值守脚本中自动执行。

### COS 首次安装

1. 用 [`deploy/cam/cos-backup-policy.json`](../../deploy/cam/cos-backup-policy.json) 创建 `HxyBlogBackupPolicy`，只关联到专用 CAM 子用户 `hxy-blog-backup`。
2. 子用户只启用编程访问，不启用控制台登录；密钥不得进入 GitHub、聊天、Shell 历史或命令行参数。
3. 安装固定版本 COSCLI 和脚本，再从交互式终端隐藏输入密钥：

```bash
sudo ./deploy/scripts/install-coscli.sh
sudo install -o root -g root -m 755 deploy/scripts/configure-coscli.sh /usr/local/sbin/hxy-blog-cos-configure
sudo install -o root -g root -m 755 deploy/scripts/sync-backup-cos.sh /usr/local/sbin/hxy-blog-db-sync
sudo install -o root -g root -m 755 deploy/scripts/fetch-backup-cos.sh /usr/local/sbin/hxy-blog-db-fetch
sudo install -o root -g root -m 755 deploy/scripts/backup-current-db.sh /usr/local/sbin/hxy-blog-db-backup-current
sudo install -o root -g root -m 755 deploy/scripts/backup-db.sh /usr/local/sbin/hxy-blog-db-backup
sudo install -o root -g root -m 644 deploy/systemd/hxy-blog-db-backup.service /etc/systemd/system/hxy-blog-db-backup.service
sudo install -o root -g root -m 644 deploy/systemd/hxy-blog-db-backup.timer /etc/systemd/system/hxy-blog-db-backup.timer
sudo systemctl daemon-reload
sudo /usr/local/sbin/hxy-blog-cos-configure
```

4. 先手工执行一次任务，完成 COS 上传、回读和隔离库恢复验证，再启用定时器：

```bash
sudo systemctl start hxy-blog-db-backup.service
sudo journalctl -u hxy-blog-db-backup.service --since today --no-pager
sudo systemctl enable --now hxy-blog-db-backup.timer
systemctl list-timers hxy-blog-db-backup.timer --no-pager
```

### 从 COS 取回并演练恢复

COS 取回入口只接受规范备份文件名，会同时下载 `.sha256` 并校验：

```bash
sudo /usr/local/sbin/hxy-blog-db-fetch \
  20260930T033000Z-sha-0123456789ab.sql.gz
sudo /usr/local/sbin/hxy-blog-db-restore-drill \
  /var/backups/hxy-blog/mysql/20260930T033000Z-sha-0123456789ab.sql.gz
```

生产数据恢复仍属于人工故障处理，不由上述脚本自动覆盖。

## 博客媒体 COS

媒体存储与数据库备份必须使用不同的桶、CAM 身份和密钥。正式媒体资源按以下清单准备：

1. 在 `ap-nanjing` 创建标准存储桶 `hxy-blog-media-1497610660`，公开读取、私有写入，关闭版本控制。
2. `media/` 正式对象不设置自动过期；生命周期规则在 1 天后终止未完成的分片上传，并在 7 天后删除 `tmp/` 对象。
3. 用 [`deploy/cam/cos-media-policy.json`](../../deploy/cam/cos-media-policy.json) 创建 `HxyBlogMediaPolicy`，只关联到专用编程访问子用户 `hxy-blog-media`。
4. 在完成 ICP 备案和 TLS 配置后，将 `media.hxy2333.site` 绑定到媒体桶；应用正文只使用该稳定域名，不保存 COS 默认域名。
5. 将以下四项写入服务器 `/etc/hxy-blog/runtime.env`，不得写入 GitHub、聊天、Shell 历史或命令行参数：

```dotenv
MEDIA_COS_BUCKET_URL=https://hxy-blog-media-1497610660.cos.ap-nanjing.myqcloud.com
MEDIA_PUBLIC_BASE_URL=https://media.hxy2333.site
MEDIA_COS_SECRET_ID=<媒体专用 CAM SecretId>
MEDIA_COS_SECRET_KEY=<媒体专用 CAM SecretKey>
```

### 隔离桶集成测试

自动化测试不得使用正式媒体桶。先创建独立测试桶和测试身份，把四个 `MEDIA_TEST_*` 变量写入临时的受限权限环境文件，再在加载该文件的终端运行：

```dotenv
MEDIA_TEST_COS_BUCKET_URL=https://<隔离测试桶>.cos.ap-nanjing.myqcloud.com
MEDIA_TEST_PUBLIC_BASE_URL=https://<隔离测试桶>.cos.ap-nanjing.myqcloud.com
MEDIA_TEST_COS_SECRET_ID=<测试身份 SecretId>
MEDIA_TEST_COS_SECRET_KEY=<测试身份 SecretKey>
```

```bash
task test-media-cos
```

测试会在 `media/integration/` 下生成不可预测对象键，经 HTTPS 回读并核对 SHA-256，最后删除测试对象。测试缺少任一变量、回读不是 HTTPS 200、字节变化或清理失败时都会失败。

应用部署后还需在管理端上传一张无 EXIF 的小图，确认返回 URL 使用 `https://media.hxy2333.site/`、文章预览可读，并在日志中看到：

- `media object upload completed`：COS 请求对象键、字节数和耗时。
- `media upload request completed`：结果分类、错误码、状态码、媒体 ID、字节数和总耗时。
- 任意 `media compensation deletion failed`：必须立即人工检查无元数据孤立对象。

## 公网开放与备案

- ICP 备案已通过，`hxy2333.site` 与 `www.hxy2333.site` 均解析到生产服务器。
- 公网入口按「公网 HTTPS 入口」一节启用：80 只用于 ACME 校验和 301 跳转，站点只在 443 提供。
- 应用容器继续保持 `127.0.0.1:8080` 绑定，不直接暴露端口；公网流量必须经过宿主机 Nginx。
- 公安备案与内容合规由站点所有者另行办理。
- 需要临时关闭公网入口时按上一节回滚；验证仍可通过 SSH 隧道进行：

```bash
ssh -N -L 8080:127.0.0.1:8080 tencent
```

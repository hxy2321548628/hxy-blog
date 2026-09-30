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
| root 恢复演练入口 | `/usr/local/sbin/hxy-blog-db-restore-drill` |
| 即时备份目录 | `/var/backups/hxy-blog/mysql`，root:root `0700` |
| 生产端口 | 仅 `127.0.0.1:8080`，备案完成前不开放 80/443 |

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

## 备份和恢复演练

- 每次部署在 Goose `up` 前创建一份 root 专属的 gzip 逻辑备份和 SHA-256 校验文件。
- 单机目录只保留最近七份即时备份；被轮换的旧文件无法从服务器恢复。
- 即时备份不能替代异机备份，正式公网开放前仍需接入服务器之外的加密存储。
- 恢复演练只导入固定隔离库 `hxy_blog_restore_drill`，完成校验后自动删除，不覆盖生产库。

管理员可在服务器执行：

```bash
sudo /usr/local/sbin/hxy-blog-db-restore-drill \
  /var/backups/hxy-blog/mysql/<backup>.sql.gz
```

演练会先验证 SHA-256 和 gzip 完整性，再输出恢复后的表数量和 Goose 版本。生产库的破坏性恢复不通过 GitHub 部署密钥开放，也不在无人值守脚本中自动执行。

## 备案期限制

- 不配置公网 DNS 切换、TLS 证书或 80/443 监听。
- 只通过 SSH 在服务器本机访问 `http://127.0.0.1:8080` 验证。
- ICP 备案完成后，通过独立上线清单启用公网入口，不修改本部署权限模型。

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
4. 保存当前 `release.env` 为 `previous.env`，再切换镜像。
5. 等待 Compose 健康并检查本机 Web/API 健康接口。
6. 失败时恢复 `previous.env`；首次部署失败则停止应用，但保留 MySQL 命名卷。

应用回滚不会自动执行数据库 down migration。当前应用尚无数据库 Schema，Release 会拒绝包含迁移文件的版本；在首个 Goose 迁移进入仓库前，必须先补齐部署前备份、Goose up 和恢复演练。

## 备案期限制

- 不配置公网 DNS 切换、TLS 证书或 80/443 监听。
- 只通过 SSH 在服务器本机访问 `http://127.0.0.1:8080` 验证。
- ICP 备案完成后，通过独立上线清单启用公网入口，不修改本部署权限模型。

# Sprint 9 开发计划

- 日期：2026-10-19
- 状态：生产公网入口已启用；管理员登录与图片上传待人工验收
- Sprint 目标：在 ICP 备案通过后启用公网 HTTPS 入口，让访客可通过 `https://hxy2333.site` 访问博客，管理员可在 HTTPS 下登录。

## 背景

当前生产栈只把 Web 容器绑定在 `127.0.0.1:8080`（[`deploy/compose.production.yaml`](../../deploy/compose.production.yaml) 第 90 行），宿主机没有任何 80/443 监听。2026-10-19 的排查确认：`146.56.243.24:80` 无监听，浏览器看到的 502 来自本机代理而非服务器。备案与 DNS 只是解除了限制，公网入口仍需单独建立。

## 目标与范围

1. 宿主机 Nginx 终结 TLS，把请求反代到 `127.0.0.1:8080`，不改变容器端口绑定。
2. 用 Let's Encrypt 为 `hxy2333.site` 和 `www.hxy2333.site` 签发证书，采用 webroot 校验，不依赖 certbot 改写站点配置。
3. HTTP 统一 301 到 HTTPS；`www` 统一 301 到规范域名 `hxy2333.site`。
4. 与容器内 Nginx 对齐 11 MiB 上传上限，并保持请求体流式转发和 60 秒代理超时。
5. 输出最小安全响应头：HSTS（不启用 `includeSubDomains`/preload）、`X-Content-Type-Options`、`Referrer-Policy`，并隐藏 Nginx 版本。
6. 提供幂等的安装脚本、离线渲染测试和公网自检脚本，使上线过程可重复、可验证。
7. 同步更新生产运行手册、产品 Backlog 与代码阅读指南。

## 非目标

- 不修改 Go API、前端、数据库 Schema、鉴权逻辑或 Compose 端口绑定。
- 不引入 CLB、CDN、EdgeOne 等云端入口；不改 `media.hxy2333.site` 的 COS 绑定。
- 不自动调用腾讯云 API 开放防火墙：80/443 放行仍需人工在轻量应用服务器控制台完成。
- 不监听 IPv6（当前 DNS 无 AAAA 记录），不启用 HTTP/3、OCSP stapling 或多站点托管。
- 不代办公安备案。

## 已确认决策与假设

- 主站规范域名沿用 ADR-0005 的 `hxy2333.site`，`www` 作为别名永久跳转。
- 服务器可从发行版仓库安装 `nginx` 与 `certbot`；脚本同时支持 `apt-get` 和 `dnf`。
- 证书目录固定为 `/etc/letsencrypt/live/hxy2333.site/`，ACME webroot 固定为 `/var/www/certbot`。
- 站点配置固定为 `/etc/nginx/conf.d/hxy-blog.conf`，由脚本整体渲染，不手工编辑。
- 生产 `runtime.env` 的 `AUTH_ALLOWED_ORIGINS` 必须包含 `https://hxy2333.site`；否则后台登录会被来源校验拒绝。
- 后台登录依赖 `Secure` Cookie，因此 HTTPS 是本 Sprint 的硬性验收项，不是可选项。

## 故事与验收标准

### 故事 1：证书签发与 HTTP 跳转

- Given 容器在 `127.0.0.1:8080` 健康且 80 端口可从公网访问，When 在服务器执行安装脚本，Then 脚本完成 HTTP 配置、webroot 校验和证书签发，并把站点切换为 HTTPS 配置。
- Given 安装完成，When 访问 `http://hxy2333.site/`，Then 返回 301 且 `Location` 为 `https://hxy2333.site/`。
- Given 安装完成，When 检查续期，Then `certbot renew` 由 systemd timer 驱动，续期后通过 deploy hook 自动 reload Nginx。
- Given 上游不健康，When 执行安装脚本，Then 脚本在写入任何配置前中止并给出排查命令。
- Given 新配置未通过 `nginx -t`，When 脚本 reload，Then 恢复上一份配置并返回非零状态。

### 故事 2：站点、API 与后台经 HTTPS 可用

- Given 公网入口启用，When 访问 `https://hxy2333.site/`、文章详情和任意前端路由，Then 返回 200 且 SPA 回退正常。
- Given 公网入口启用，When 请求 `https://hxy2333.site/api/health`，Then 返回 `{"status":"ok"}`，证明宿主机 Nginx → 容器 Nginx → Go API 全链路可用。
- Given 管理员在 HTTPS 页面登录，When 刷新页面，Then 刷新令牌 Cookie 生效，会话可恢复（人工验收）。
- Given 上传不超过 10 MiB 的图片，When 经公网 HTTPS 上传，Then 请求不被 413 拒绝并可正常插入文章（人工验收）。
- Given 访问 `https://www.hxy2333.site/`，When 请求任意路径，Then 301 到同路径的 `https://hxy2333.site`。

### 故事 3：可重复安装与可执行自检

- Given 任意一台装有 bash 的机器，When 执行 `task edge:config`，Then 渲染出的 HTTP/TLS 配置包含预期的监听、证书、代理与安全头，且没有残留占位符。
- Given 非法域名或非法上游地址，When 执行渲染，Then 脚本以非零状态拒绝。
- Given 公网入口已启用，When 在开发机执行 `task edge:check`，Then DNS、跳转、证书、健康接口、SPA 回退、安全头和 `www` 跳转全部 PASS；任一项失败时脚本返回非零状态。
- 自检脚本默认绕过本机 HTTP 代理，避免把代理错误页误判为站点响应。

## 风险与依赖

- 腾讯云防火墙未放行 80/443 时，ACME 校验会失败；脚本在失败时保留 HTTP 配置并给出排查提示，回滚只需删除站点配置。
- HSTS 下发后浏览器会在一段时间内强制 HTTPS；本 Sprint 只设置 1 年且不包含子域，避免媒体域名和后续子域被锁定。
- 双层 Nginx 必须正确传递 `X-Forwarded-Proto`；否则后台来源校验和 Cookie 行为与浏览器地址栏不一致。容器内 Nginx 已经传递该头，宿主机入口必须继续传递。
- `certbot` 版本差异（发行版 1.x 与 2.x）通过只使用 `certonly --webroot` 兼容子集规避。
- 自动续期依赖 systemd timer；安装脚本会输出检查命令，自检脚本会报告证书剩余天数。
- ICP 备案已完成，但公安备案与内容合规仍由站点所有者负责。

## 执行顺序与验证命令

1. 更新 Backlog 与本计划，确认范围。
2. 先写离线测试 `deploy/scripts/test-edge-config.sh` 并确认它对缺失实现失败。
3. 新增 `deploy/scripts/install-edge.sh`（内嵌 HTTP/TLS 两阶段 Nginx 模板），让离线测试通过。
4. 新增 `deploy/scripts/check-public-entry.sh`，接入 `task edge:check`。
5. 在 Taskfile 中接入 `edge:config`、`edge:check`，并把新脚本纳入 `check-deploy`。
6. 更新 `doc/engineering/production-deployment.md` 与 `doc/engineering/code-reading-guide.md`。
7. 运行 `task check`，提交并等待合并。
8. 合并后按下方上线步骤在服务器执行，并完成人工验收。

```bash
bash deploy/scripts/test-edge-config.sh   # 离线渲染与参数校验
task edge:config                          # 同上，走 Task 入口
task check                                # 提交前完整门禁
task edge:check                           # 上线后从开发机执行公网自检
```

## 上线步骤（人工）

1. 在服务器确认应用健康：`curl -fsS http://127.0.0.1:8080/healthz`。
2. 在腾讯云轻量应用服务器控制台放行 TCP 80/443；仅在系统防火墙放行无效。
3. 在服务器仓库目录执行：

   ```bash
   sudo ./deploy/scripts/install-edge.sh --email <证书通知邮箱>
   ```

4. 在开发机执行 `task edge:check -- --expect-ip 146.56.243.24`，直到全部 PASS。
5. 浏览器验证后台登录、文章保存与图片上传；确认 `AUTH_ALLOWED_ORIGINS` 含 `https://hxy2333.site`。

## 回滚

公网入口与应用镜像相互独立，回滚不影响数据库与已部署版本：

```bash
sudo rm -f /etc/nginx/conf.d/hxy-blog.conf
sudo nginx -t && sudo systemctl reload nginx
```

删除后站点回到只能通过 `127.0.0.1:8080` 或 SSH 隧道访问的状态；证书与 ACME 记录保留，重新执行安装脚本即可再次启用。

## 验收记录

本地验证（2026-10-19）：

- `bash deploy/scripts/test-edge-config.sh`：31 项断言全部通过，覆盖 HTTP/TLS 两阶段渲染、`www` 跳转、上传上限、`X-Forwarded-Proto`、安全头、旧版 Nginx 语法、非法参数与渲染幂等性。
- `task edge:config`：通过，输出与离线断言一致。
- `task check`：通过；Go 全部包测试、React 12 个测试文件共 28 个测试、ESLint、生产构建、Compose 配置校验和部署脚本语法检查均成功。
- `task edge:check`：当前按预期报 9 项失败，因为 80/443 尚未在服务器启用；该结果同时验证了脚本在入口未开放时返回非零状态。
- 未验证项：真实 Let's Encrypt 签发、`nginx -t` 在服务器上的结果、腾讯云防火墙放行、后台登录与图片上传。以上需要在合并后按上线步骤执行并人工确认。

生产验收（2026-10-19）：

- 服务器 Ubuntu 24.04 上安装 Nginx 1.24.0 与 Certbot 2.9.0；应用仍由 `127.0.0.1:8080` 提供，未改动容器端口绑定。
- Let's Encrypt 证书签发成功，覆盖 `hxy2333.site` 与 `www.hxy2333.site`，有效期至 2027-01-17；首次签发因 CA 二次校验超时失败，原样重试后成功。
- `certbot renew --dry-run --no-random-sleep-on-renew` 成功，续期配置包含 `renew_hook = systemctl reload nginx`，`certbot.timer` 已启用。
- `task edge:check -- --expect-ip 146.56.243.24` 共 11 项全部通过；另有 4 个境外节点（保加利亚、西班牙、印度、乌克兰）访问 `https://hxy2333.site/` 均返回 200。
- 80/443 实测公网可达，腾讯云控制台防火墙未做额外改动；HTTP 301、`www` 301、SPA 回退、安全响应头和 API 健康检查均符合预期。
- `/etc/hxy-blog/runtime.env` 的 `AUTH_ALLOWED_ORIGINS` 已追加 `https://hxy2333.site`（备份保留在 `/var/backups/hxy-blog/edge/`），`api` 容器重建后健康。
- 待人工验收：浏览器管理员登录、文章编辑与图片上传；这些依赖真实凭据，未在自动化中执行。

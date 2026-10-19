# Sprint 10 开发计划

- 日期：2026-10-19
- 状态：生产已启用；管理员重新上传图片的人工验收待完成
- Sprint 目标：让新上传图片生成 `https://media.hxy2333.site/...` 的稳定地址，并让该域名具备自动续期的有效 HTTPS；同时回填历史文章中残留的 COS 默认域名。

## 背景

2026-10-19 排查确认：

- 编辑器插入的 Markdown 地址来自后端 `MEDIA_PUBLIC_BASE_URL`（[`src/backend/internal/media/media.go`](../../src/backend/internal/media/media.go) 第 143 行），当前生产环境把它配置成了 COS 默认域名，因此插入的一定是 COS 地址。
- `media.hxy2333.site` 已通过 CNAME 指向媒体桶，HTTP 读取对象正常，但 HTTPS 返回的是 `*.cos.ap-nanjing.myqcloud.com` 通配证书，与自定义域名不匹配。
- `posts` 表 5 篇文章中有 1 篇（`bubble-sort`）正文仍包含 COS 默认域名。
- 腾讯云 COS 自定义源站域名的 HTTPS 需要绑定证书，而[腾讯云免费证书有效期 90 天且不支持续费](https://cloud.tencent.com.cn/document/product/400/89868)，每季度都要重新申请并绑定。

## 已确认决策

**media 域名指回博客服务器，由宿主机 Nginx 反代 COS，复用现有 Let's Encrypt 自动续期。**

理由：

- 与主站同一套 certbot 续期链路，不需要每 90 天手工重新申请和绑定证书。
- 腾讯云官方文档也把「为域名配置反向代理」列为 COS 自定义域名的 HTTPS 方案之一（[配置自定义域名支持 HTTPS 访问](https://cloud.tencent.com/document/product/436/11142)）。
- 代价是图片流量经过 2C2G 服务器；以个人博客的访问量可以接受，后续如有需要再加缓存或 CDN，不阻塞本次修复。

该决策作为 ADR-0002 的实施补充记录，不改变「文章只保存稳定媒体域名」的原始结论。

## 目标与范围

1. 修正生产 `runtime.env` 的 `MEDIA_PUBLIC_BASE_URL` 为 `https://media.hxy2333.site`，重启 `api` 容器；新上传与上传响应都使用稳定域名。
2. 扩展 `deploy/scripts/install-edge.sh`：为 `media.hxy2333.site` 增加 80（ACME + 跳转）与 443（反代 COS）站点，证书独立签发、独立续期，只允许 GET/HEAD。
3. 新增 `deploy/scripts/check-media-entry.sh` 与 `task media:check`，验证 DNS、跳转、证书、对象读取和响应头。
4. 新增 `deploy/scripts/rewrite-media-urls.sh`：把历史文章中的 COS 默认域名回填为媒体域名，默认试运行，`--apply` 才写入。
5. 扩展离线测试，覆盖 media 站点渲染、证书路径、反代 Host 与只读方法限制。
6. 更新生产运行手册、代码阅读指南、ADR-0002 补充说明、Backlog 与验收记录。

## 非目标

- 不修改上传流程、图片校验、鉴权、数据库 Schema 或媒体桶/CAM 配置。
- 不引入 CDN、EdgeOne 或对象存储迁移。
- 不做 Nginx 磁盘缓存、图片压缩或格式转换；这些留到出现真实带宽压力时再评估。
- 不改变主站 `hxy2333.site` / `www` 的现有行为。
- 不改 `deploy/compose.production.yaml` 的端口绑定。

## 故事与验收标准

### 故事 1：新上传使用媒体域名

- Given 生产 `MEDIA_PUBLIC_BASE_URL=https://media.hxy2333.site`，When 管理员上传一张图片并插入 Markdown，Then 插入的地址以 `https://media.hxy2333.site/` 开头，且不再出现 `cos.ap-nanjing.myqcloud.com`。
- Given 上传接口返回 201，When 浏览器读取返回的 `url`，Then 该地址可直接打开且证书有效。
- Given 历史文章 `bubble-sort`，When 执行回填脚本，Then 正文中的 COS 地址全部替换为媒体域名，图片仍可加载。

### 故事 2：media 域名 HTTPS 与反代

- Given `media.hxy2333.site` 的 A 记录指向博客服务器，When 在服务器执行安装脚本，Then 80 端口完成 ACME 校验并 301 到 HTTPS，443 使用覆盖该域名的 Let's Encrypt 证书。
- Given 请求 `https://media.hxy2333.site/media/...png`，When 对象存在，Then 返回 200 与正确的 `Content-Type`；对象不存在时返回 COS 的 404，而不是 Nginx 默认页。
- Given 使用 POST/PUT 请求媒体域名，When 到达 Nginx，Then 返回 403，图片入口保持只读。
- Given 证书续期，When `certbot renew` 执行，Then 两张证书都通过 deploy hook reload Nginx，不需要人工介入。
- Given 主站证书已存在但 media 证书尚未签发，When 安装脚本中途失败，Then 主站 443 保持可用，不会被降级为 HTTP。

### 故事 3：可重复验证

- Given 任意机器，When 执行 `task edge:config`，Then 渲染结果同时包含主站与 media 的站点块、证书路径、反代 Host 和只读限制，且无残留占位符。
- Given 媒体入口已启用，When 执行 `task media:check`，Then DNS、301、证书、对象读取全部 PASS；失败时返回非零状态。
- Given 修改了回填脚本，When 不带 `--apply` 执行，Then 只输出匹配数量和示例，不修改数据库。

## 风险与依赖

- **DNS 切换依赖人工**：需要在 DNSPod 把 `media` 的 CNAME 改成 A 记录 `146.56.243.24`；切换前 media 域名不影响线上，因为图片仍使用 COS 地址。
- **服务器带宽**：图片下载会占用 Lighthouse 流量配额与出口带宽；个人博客量级可控，后续可加缓存或 CDN。
- **证书签发顺序**：media 证书依赖 80 端口可达；脚本在证书缺失时先保证主站 HTTPS，再补 media，避免中途失败导致主站降级。
- **回填不可逆风险**：脚本默认 dry-run，并支持 `--from/--to` 反向替换；执行前会输出影响行数。
- **缓存与 COS 回源**：本次不引入缓存，所有图片请求都会回源 COS；若后续流量上升，单独评估缓存策略。

## 执行顺序与验证命令

1. 更新 Backlog 与本计划。
2. 先扩展 `deploy/scripts/test-edge-config.sh`，确认 media 断言在实现前失败。
3. 扩展 `install-edge.sh` 与内嵌模板，让离线测试通过。
4. 新增 `check-media-entry.sh`、`rewrite-media-urls.sh`，接入 Taskfile 与 `check-deploy`。
5. 更新运行手册、代码阅读指南与 ADR-0002 补充说明。
6. 运行 `task check`，提交并合并。
7. 用户切换 DNS 后，在服务器执行安装脚本，完成证书与反代。
8. 切换 `MEDIA_PUBLIC_BASE_URL`、回填历史文章并做上传验收。

```bash
bash deploy/scripts/test-edge-config.sh
task edge:config
task check
task media:check -- --object media/2026/10/<key>.png   # 上线后
```

## 上线步骤（人工）

1. DNSPod：把 `media.hxy2333.site` 的 CNAME 改为 A 记录 `146.56.243.24`，确认解析生效：

   ```bash
   dig +short media.hxy2333.site
   ```

2. 分发并执行新版安装脚本（会自动签发 media 证书并启用反代）：

   ```bash
   scp deploy/scripts/install-edge.sh tencent:/tmp/install-edge.sh
   ssh tencent 'sudo install -o root -g root -m 755 /tmp/install-edge.sh /usr/local/sbin/hxy-blog-install-edge'
   ssh tencent 'sudo /usr/local/sbin/hxy-blog-install-edge'
   ```

3. 在开发机验证媒体入口：

   ```bash
   task media:check -- --object media/2026/10/ebb6700ad8907ad24797f52672b564cd.png
   ```

4. 切换生产环境变量并重启 API：

   ```dotenv
   MEDIA_PUBLIC_BASE_URL=https://media.hxy2333.site
   ```

5. 回填历史文章（先试运行，再 `--apply`）。

6. 管理员在后台重新上传一张图片，确认插入的 Markdown 使用媒体域名。

## 回滚

```bash
# 1. 恢复环境变量并重启 API
#    MEDIA_PUBLIC_BASE_URL=https://hxy-blog-media-1497610660.cos.ap-nanjing.myqcloud.com

# 2. 反向回填（把媒体域名改回 COS）
sudo /usr/local/sbin/hxy-blog-media-rewrite --apply \
  --from https://media.hxy2333.site \
  --to https://hxy-blog-media-1497610660.cos.ap-nanjing.myqcloud.com

# 3. DNS 把 media 改回 CNAME：hxy-blog-media-1497610660.cos.ap-nanjing.myqcloud.com
```

media 站点块与主站相互独立；即使 media 证书或反代异常，主站 443 不受影响。

## 验收记录

本地验证（2026-10-19）：

- `bash deploy/scripts/test-edge-config.sh`：42 项断言全部通过，覆盖媒体域名 80/443 渲染、独立证书、反代 Host/SNI、只读方法限制、非法参数与渲染幂等性。
- `task check`：通过；Go 测试、React 28 个测试、ESLint、生产构建、Compose 配置与部署脚本语法检查均成功。

生产验证（2026-10-19）：

- `media.hxy2333.site` 的 A 记录已指向 `146.56.243.24`；安装脚本为其签发独立 Let's Encrypt 证书（有效期至 2027-01-17），并启用只读反代。
- `task media:check -- --object media/2026/10/40ccd6eaaf2bf7d79e541d98bf007e44.png`：6 项全部通过，证书覆盖媒体域名、POST 返回 403、对象读取 200。
- 文章 `bubble-sort` 的 3 个图片地址回填为媒体域名；PNG、BMP、GIF 均返回 200 且 Content-Type 正确。
- `runtime.env` 的 `MEDIA_PUBLIC_BASE_URL` 已改为 `https://media.hxy2333.site` 并重建 `api`；容器内环境变量确认生效，公开详情接口已无 `cos.ap-nanjing` 地址。
- `task edge:check`：10 项全部通过，主站 443、www 跳转与安全响应头未受媒体改动影响。
- 首次安装时媒体证书签发后的本机校验因 reload 时序失败一次；已为媒体校验补上与主站相同的重试，重新执行安装脚本后通过。
- 待人工验收：管理员在后台重新上传一张图片，确认插入的 Markdown 使用 `https://media.hxy2333.site/`。

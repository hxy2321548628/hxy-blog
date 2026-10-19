#!/usr/bin/env bash

# 安装或更新生产公网入口：宿主机 Nginx 终结 TLS，把请求反代到
# 127.0.0.1:8080 上的容器栈，应用容器的端口绑定保持不变。
#
# 该脚本只在服务器上以 root 手工执行。CI 和开发机只使用 --print-config
# 做离线渲染断言，因此渲染路径不能依赖 root、网络或已安装的 Nginx。

set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# 这些路径与运行手册绑定；改名会破坏续期、备份和回滚说明，必须同步文档。
readonly NGINX_CONF="/etc/nginx/conf.d/hxy-blog.conf"
readonly ACME_ROOT="/var/www/certbot"
readonly CERT_ROOT="/etc/letsencrypt/live"
readonly BACKUP_DIR="/var/backups/hxy-blog/edge"
readonly RUNTIME_ENV="/etc/hxy-blog/runtime.env"

PRIMARY_DOMAIN="hxy2333.site"
WWW_DOMAIN="www.hxy2333.site"
MEDIA_DOMAIN="media.hxy2333.site"
# 媒体桶的默认域名；反代时同时作为 Host 和 SNI，不能改成自定义域名。
MEDIA_BUCKET="hxy-blog-media-1497610660.cos.ap-nanjing.myqcloud.com"
UPSTREAM="127.0.0.1:8080"
EMAIL=""
HTTP2="auto"
INSTALL_PACKAGES=0
PRINT_STAGE=""

log() { printf '[edge] %s\n' "$*"; }
warn() { printf '[edge] 警告：%s\n' "$*" >&2; }
die() {
  printf '[edge] 错误：%s\n' "$*" >&2
  exit 1
}

# 参数错误使用 sysexits 的 EX_USAGE，便于区分“调用方写错”和“运行环境失败”。
usage_die() {
  printf '[edge] 错误：%s\n' "$*" >&2
  exit 64
}

usage() {
  cat <<'EOF'
用法：install-edge.sh [选项]

在服务器上以 root 执行时，脚本会：
  1. 确认 127.0.0.1:8080 的 Web/API 健康；
  2. 写入 80 端口配置并完成 Let's Encrypt webroot 校验；
  3. 切换为 HTTPS 配置，校验通过后 reload Nginx。

选项：
  --domain <域名>        主域名，默认 hxy2333.site
  --www <域名>           www 别名，默认 www.<主域名>
  --media-domain <域名>  媒体域名，默认 media.<主域名>
  --media-bucket <域名>  媒体桶默认域名，默认 hxy-blog-media-1497610660.cos.ap-nanjing.myqcloud.com
  --upstream <host:port> 应用上游，默认 127.0.0.1:8080
  --email <邮箱>         首次签发证书的通知邮箱
  --http2 <auto|modern|legacy>  Nginx 版本适配，默认 auto
  --install-packages     缺少 nginx/certbot 时用系统包管理器安装
  --print-config <http|tls>     只渲染配置到标准输出，不读取系统状态
  -h, --help             显示本帮助

离线渲染示例：
  install-edge.sh --print-config tls --http2 legacy
EOF
}

parse_args() {
  while (($# > 0)); do
    case "$1" in
      --domain)
        [[ $# -ge 2 ]] || usage_die '--domain 缺少参数'
        PRIMARY_DOMAIN=$2
        shift 2
        ;;
      --www)
        [[ $# -ge 2 ]] || usage_die '--www 缺少参数'
        WWW_DOMAIN=$2
        shift 2
        ;;
      --media-domain)
        [[ $# -ge 2 ]] || usage_die '--media-domain 缺少参数'
        MEDIA_DOMAIN=$2
        shift 2
        ;;
      --media-bucket)
        [[ $# -ge 2 ]] || usage_die '--media-bucket 缺少参数'
        MEDIA_BUCKET=$2
        shift 2
        ;;
      --upstream)
        [[ $# -ge 2 ]] || usage_die '--upstream 缺少参数'
        UPSTREAM=$2
        shift 2
        ;;
      --email)
        [[ $# -ge 2 ]] || usage_die '--email 缺少参数'
        EMAIL=$2
        shift 2
        ;;
      --http2)
        [[ $# -ge 2 ]] || usage_die '--http2 缺少参数'
        HTTP2=$2
        shift 2
        ;;
      --install-packages)
        INSTALL_PACKAGES=1
        shift
        ;;
      --print-config)
        [[ $# -ge 2 ]] || usage_die '--print-config 缺少参数'
        PRINT_STAGE=$2
        shift 2
        ;;
      -h | --help)
        usage
        exit 0
        ;;
      *)
        usage >&2
        exit 64
        ;;
    esac
  done

  validate_domain "$PRIMARY_DOMAIN" || usage_die "主域名格式无效：${PRIMARY_DOMAIN}"
  validate_domain "$WWW_DOMAIN" || usage_die "www 域名格式无效：${WWW_DOMAIN}"
  validate_domain "$MEDIA_DOMAIN" || usage_die "媒体域名格式无效：${MEDIA_DOMAIN}"
  validate_domain "$MEDIA_BUCKET" || usage_die "媒体桶域名格式无效：${MEDIA_BUCKET}"
  validate_upstream "$UPSTREAM" || usage_die "上游地址必须是 host:port：${UPSTREAM}"

  case "$HTTP2" in
    auto | modern | legacy) ;;
    *) usage_die '--http2 只接受 auto、modern 或 legacy' ;;
  esac

  if [[ -n $PRINT_STAGE && $PRINT_STAGE != http && $PRINT_STAGE != tls ]]; then
    usage_die '--print-config 只接受 http 或 tls'
  fi
}

# 域名只允许小写字母、数字、点和连字符；拒绝空格、协议前缀和路径，
# 因为这些字符会破坏 server_name 与证书参数。
validate_domain() {
  [[ $1 =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$ ]]
}

# 上游只接受 host:port，避免把 http:// 前缀或路径写进 proxy_pass。
validate_upstream() {
  [[ $1 =~ ^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?:([0-9]{1,5})$ ]] || return 1
  local port=${BASH_REMATCH[2]}
  ((port >= 1 && port <= 65535))
}

# Nginx 1.25.1 起推荐使用独立的 http2 指令；旧版本只能写在 listen 参数里。
# 没有 Nginx 时按旧语法渲染，保证 CI 的离线断言稳定。
detect_http2_mode() {
  case "$HTTP2" in
    modern | legacy)
      printf '%s\n' "$HTTP2"
      return 0
      ;;
  esac

  local version major minor
  if version=$(nginx -v 2>&1) && [[ $version =~ nginx/([0-9]+)\.([0-9]+) ]]; then
    major=${BASH_REMATCH[1]}
    minor=${BASH_REMATCH[2]}
    if ((major > 1 || (major == 1 && minor >= 25))); then
      printf 'modern\n'
      return 0
    fi
  fi
  printf 'legacy\n'
}

# 公网入口配置内嵌在脚本里：安装时只需要一个 root 拥有的文件，
# 不依赖服务器上是否存在仓库检出目录，也避免模板与脚本版本错配。
#
# 80 端口块由主站和媒体域名共用，单独抽出后，
# “只渲染 HTTP”和“渲染 TLS”两种阶段会看到完全一致的 ACME 入口。
http_server_blocks() {
  cat <<'NGINX_TEMPLATE'
# 由 deploy/scripts/install-edge.sh 渲染，请勿手工修改。
#
# 80 端口只做两件事：提供 ACME 校验目录，把其他请求跳转到 HTTPS。
# 这里不能出现任何 ssl_* 指令：首次安装时证书还不存在。

server {
    listen 80;
    server_name __PRIMARY_DOMAIN__ __WWW_DOMAIN__;

    # Certbot HTTP-01 校验必须能从公网读到这个目录；它优先于下面的跳转规则。
    location ^~ /.well-known/acme-challenge/ {
        root __ACME_ROOT__;
        default_type text/plain;
    }

    # 其余请求统一跳到规范域名，避免 www 与主域名各自维护内容。
    location / {
        return 301 https://__PRIMARY_DOMAIN__$request_uri;
    }
}

server {
    listen 80;
    server_name __MEDIA_DOMAIN__;

    # 媒体域名使用独立证书，ACME 校验同样落在宿主机上。
    location ^~ /.well-known/acme-challenge/ {
        root __ACME_ROOT__;
        default_type text/plain;
    }

    location / {
        return 301 https://__MEDIA_DOMAIN__$request_uri;
    }
}
NGINX_TEMPLATE
}

# 主站 443 与 www 跳转块；行为与 Sprint 9 保持一致。
primary_tls_blocks() {
  cat <<'NGINX_TEMPLATE'
server {
    listen 443 ssl__HTTP2_PARAM__;
    __HTTP2_DIRECTIVE__
    server_name __PRIMARY_DOMAIN__;

    ssl_certificate __CERT_BASE__/fullchain.pem;
    ssl_certificate_key __CERT_BASE__/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    # 共享会话缓存减少重复握手；单机博客不需要更大的缓存区。
    ssl_session_cache shared:hxy_blog_tls:10m;
    ssl_session_timeout 1d;
    ssl_session_tickets off;

    # 隐藏版本号，减少针对具体 Nginx 版本的探测。
    server_tokens off;
    # 1 年 HSTS 但不包含子域：media 使用独立证书，避免被子域策略锁定。
    add_header Strict-Transport-Security "max-age=31536000" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    # 容器内 Nginx 的上传上限是 11 MiB；外层必须不小于该值，否则先被 413 拒绝。
    client_max_body_size 11m;

    location / {
        proxy_pass http://__UPSTREAM__;
        proxy_http_version 1.1;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        # 必须传递真实协议：后台来源校验与 Secure Cookie 都依赖它。
        proxy_set_header X-Forwarded-Proto $scheme;
        # 上传接近上限时直传请求体，避免先写满宿主机磁盘。
        proxy_request_buffering off;
        proxy_connect_timeout 5s;
        proxy_send_timeout 60s;
        proxy_read_timeout 60s;
    }
}

server {
    listen 443 ssl__HTTP2_PARAM__;
    __HTTP2_DIRECTIVE__
    server_name __WWW_DOMAIN__;

    ssl_certificate __CERT_BASE__/fullchain.pem;
    ssl_certificate_key __CERT_BASE__/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_session_cache shared:hxy_blog_tls:10m;
    ssl_session_timeout 1d;
    ssl_session_tickets off;
    server_tokens off;

    # www 只作为别名存在，保留路径与查询串跳转到规范域名。
    return 301 https://__PRIMARY_DOMAIN__$request_uri;
}
NGINX_TEMPLATE
}

# 媒体域名 443：只读反代到 COS 默认域名。
# COS 要求 Host 必须等于桶域名，因此不能透传 $host；
# 入口只允许 GET/HEAD，避免它被当成任意对象写通道。
media_tls_block() {
  cat <<'NGINX_TEMPLATE'
server {
    listen 443 ssl__HTTP2_PARAM__;
    __HTTP2_DIRECTIVE__
    server_name __MEDIA_DOMAIN__;

    ssl_certificate __MEDIA_CERT_BASE__/fullchain.pem;
    ssl_certificate_key __MEDIA_CERT_BASE__/privkey.pem;
    ssl_protocols TLSv1.2 TLSv1.3;
    ssl_session_cache shared:hxy_blog_tls:10m;
    ssl_session_timeout 1d;
    ssl_session_tickets off;
    server_tokens off;
    add_header X-Content-Type-Options "nosniff" always;

    location / {
        # 图片只需要读取；拒绝其他方法可以防止入口被用来写对象或探测 API。
        limit_except GET HEAD {
            deny all;
        }

        proxy_pass https://__MEDIA_BUCKET__;
        proxy_ssl_server_name on;
        # COS 自定义域名回源必须使用桶域名作为 Host。
        proxy_set_header Host __MEDIA_BUCKET__;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_connect_timeout 5s;
        proxy_send_timeout 30s;
        proxy_read_timeout 30s;
    }
}
NGINX_TEMPLATE
}

render_config() {
  local stage=$1 include_media_tls=${2:-1}
  local mode http2_param='' http2_directive='' content=''

  mode=$(detect_http2_mode)
  if [[ $mode == modern ]]; then
    http2_directive='http2 on;'
  else
    http2_param=' http2'
  fi

  case "$stage" in
    http) content=$(http_server_blocks) ;;
    tls)
      content="$(http_server_blocks)"$'\n\n'"$(primary_tls_blocks)"
      # 媒体证书尚未签发时不渲染 443 块，主站 HTTPS 因此不会被中途降级。
      if ((include_media_tls)); then
        content+=$'\n\n'"$(media_tls_block)"
      fi
      ;;
    *) die "未知的配置阶段：${stage}" ;;
  esac

  # 只替换受校验的占位符；模板里的 $host、$request_uri 等 Nginx 变量保持原样。
  sed \
    -e "s|__PRIMARY_DOMAIN__|${PRIMARY_DOMAIN}|g" \
    -e "s|__WWW_DOMAIN__|${WWW_DOMAIN}|g" \
    -e "s|__MEDIA_DOMAIN__|${MEDIA_DOMAIN}|g" \
    -e "s|__MEDIA_BUCKET__|${MEDIA_BUCKET}|g" \
    -e "s|__UPSTREAM__|${UPSTREAM}|g" \
    -e "s|__ACME_ROOT__|${ACME_ROOT}|g" \
    -e "s|__CERT_BASE__|${CERT_ROOT}/${PRIMARY_DOMAIN}|g" \
    -e "s|__MEDIA_CERT_BASE__|${CERT_ROOT}/${MEDIA_DOMAIN}|g" \
    -e "s|__HTTP2_PARAM__|${http2_param}|g" \
    -e "s|__HTTP2_DIRECTIVE__|${http2_directive}|g" \
    <<<"$content"
}

require_root() {
  # 77 是 sysexits 的 EX_NOPERM，便于运维区分“权限不足”和“配置失败”。
  if [[ ${EUID} -ne 0 ]]; then
    printf '[edge] 错误：请以 root 执行（sudo %s --email <邮箱>）\n' "$0" >&2
    exit 77
  fi
}

require_command() {
  command -v "$1" >/dev/null 2>&1 || die "缺少命令：$1"
}

ensure_packages() {
  if command -v nginx >/dev/null 2>&1 && command -v certbot >/dev/null 2>&1; then
    return 0
  fi
  if ((INSTALL_PACKAGES == 0)); then
    die '未安装 nginx 或 certbot；确认后追加 --install-packages 重新执行'
  fi
  log '安装 nginx 与 certbot'
  if command -v apt-get >/dev/null 2>&1; then
    export DEBIAN_FRONTEND=noninteractive
    apt-get update
    apt-get install -y nginx certbot
  elif command -v dnf >/dev/null 2>&1; then
    dnf install -y nginx certbot
  else
    die '无法识别包管理器，请手工安装 nginx 与 certbot'
  fi
}

ensure_nginx_running() {
  systemctl is-active --quiet nginx && return 0
  # 首次安装后服务尚未启动；enable 让重启后入口自动恢复。
  systemctl enable --now nginx || die 'Nginx 启动失败，请检查端口占用'
}

check_upstream() {
  log "检查回环应用健康：http://${UPSTREAM}/healthz"
  curl --noproxy '*' --fail --silent --show-error --max-time 5 \
    "http://${UPSTREAM}/healthz" >/dev/null ||
    die "Web 未在 ${UPSTREAM} 健康响应；先运行 docker compose ps 并查看容器日志"

  # Web 健康只说明容器 Nginx 存活；API 代理仍需单独确认，否则上线后接口会 502。
  curl --noproxy '*' --fail --silent --show-error --max-time 5 \
    "http://${UPSTREAM}/api/health" | grep --quiet --fixed-strings '"status":"ok"' ||
    die 'API 健康检查失败；先修复容器依赖再执行安装'
}

restore_previous_config() {
  local latest=''
  latest=$(ls -1t "${BACKUP_DIR}"/hxy-blog.conf.* 2>/dev/null | head -n 1 || true)
  if [[ -n $latest ]]; then
    install -m 0644 "$latest" "$NGINX_CONF"
    log "已恢复上一份配置：${latest}"
  else
    rm -f "$NGINX_CONF"
    log '没有历史配置，已移除站点文件'
  fi
}

write_config() {
  local stage=$1 rendered tmp

  rendered=$(render_config "$stage" "${2:-1}")
  # 部分发行版的 Nginx 没有 conf.d 目录，先创建再写入，避免首次安装失败。
  install -d -m 0755 "$(dirname "$NGINX_CONF")"
  tmp="${NGINX_CONF}.tmp.$$"
  printf '%s\n' "$rendered" >"$tmp"

  # 备份保留时间戳，便于 nginx -t 失败时人工比对；目录仅 root 可读。
  install -d -m 0700 "$BACKUP_DIR"
  if [[ -f $NGINX_CONF ]]; then
    cp -a "$NGINX_CONF" "${BACKUP_DIR}/hxy-blog.conf.$(date +%Y%m%dT%H%M%S%N)"
  fi

  install -m 0644 "$tmp" "$NGINX_CONF"
  rm -f "$tmp"

  if ! nginx -t; then
    restore_previous_config
    die 'nginx -t 未通过，已恢复上一份配置'
  fi
  systemctl reload nginx
  log "已应用 ${stage} 阶段配置：${NGINX_CONF}"
}

# 为指定证书名签发一张覆盖所有域名的证书；已存在时直接复用。
# 用独立的 --cert-name 可以让主站与媒体域名各自续期，互不影响。
ensure_certificate() {
  local cert_name=$1
  shift
  local cert="${CERT_ROOT}/${cert_name}/fullchain.pem"
  local domains=("$@")

  if [[ -f $cert ]]; then
    log "证书已存在，跳过签发：${cert}"
    return 0
  fi
  [[ -n $EMAIL ]] || die '首次签发证书需要 --email 提供通知邮箱'

  local args=()
  local domain
  for domain in "${domains[@]}"; do
    args+=(-d "$domain")
  done

  install -d -m 0755 "$ACME_ROOT"
  log "申请 Let's Encrypt 证书：${domains[*]}"
  # 只使用 certonly + webroot：让站点配置完全由本脚本渲染，
  # 避免 certbot 改写 Nginx 配置后在下次安装时被覆盖。
  certbot certonly --webroot --webroot-path "$ACME_ROOT" \
    --cert-name "$cert_name" "${args[@]}" \
    --email "$EMAIL" --agree-tos --non-interactive \
    --deploy-hook 'systemctl reload nginx' ||
    die "证书签发失败：${domains[*]}；确认 80 端口已放行且域名解析指向本机"
}

verify_local() {
  log '验证本机 HTTPS 入口'
  # reload 返回时旧 worker 可能仍在收尾，新监听端口需要短暂时间生效；
  # 因此这里重试而不是一次失败就回滚。
  local attempt
  for attempt in $(seq 1 10); do
    if curl --noproxy '*' --fail --silent --max-time 5 \
      --resolve "${PRIMARY_DOMAIN}:443:127.0.0.1" \
      "https://${PRIMARY_DOMAIN}/healthz" >/dev/null 2>&1; then
      break
    fi
    if ((attempt == 10)); then
      die '本机 HTTPS 健康检查失败'
    fi
    sleep 1
  done

  curl --noproxy '*' --fail --silent --show-error --max-time 10 \
    --resolve "${PRIMARY_DOMAIN}:443:127.0.0.1" \
    "https://${PRIMARY_DOMAIN}/api/health" |
    grep --quiet --fixed-strings '"status":"ok"' ||
    die '本机 HTTPS API 代理检查失败'

  local redirect
  redirect=$(curl --noproxy '*' --silent --show-error --max-time 10 \
    --resolve "${PRIMARY_DOMAIN}:80:127.0.0.1" \
    --output /dev/null --write-out '%{http_code} %{redirect_url}' \
    "http://${PRIMARY_DOMAIN}/")
  [[ $redirect == "301 https://${PRIMARY_DOMAIN}/" ]] ||
    die "HTTP 跳转不符合预期：${redirect}"

  # 媒体根路径通常返回 COS 的 404；这里只要求 TLS 校验通过且反代可达，
  # 具体对象的可用性由 check-media-entry.sh 用真实 key 验证。
  local media_code
  media_code=$(curl --noproxy '*' --silent --show-error --max-time 10 \
    --resolve "${MEDIA_DOMAIN}:443:127.0.0.1" \
    --output /dev/null --write-out '%{http_code}' \
    "https://${MEDIA_DOMAIN}/" 2>/dev/null) || media_code=000
  if [[ -z $media_code || $media_code == 000 ]]; then
    die "媒体域名 HTTPS 校验失败：${MEDIA_DOMAIN}"
  fi
  log '本机校验通过'
}

check_runtime_origins() {
  [[ -f $RUNTIME_ENV ]] || return 0
  # 来源必须与浏览器地址栏的 https:// 主域名完全一致，否则后台登录返回 403。
  if ! grep --quiet -E "AUTH_ALLOWED_ORIGINS=.*https://${PRIMARY_DOMAIN}([,]|$)" "$RUNTIME_ENV"; then
    warn "${RUNTIME_ENV} 的 AUTH_ALLOWED_ORIGINS 未包含 https://${PRIMARY_DOMAIN}，后台登录会被拒绝"
  fi
}

print_summary() {
  cat <<EOF

公网入口已启用：https://${PRIMARY_DOMAIN}
  - www 别名：https://${WWW_DOMAIN} → 301 到主域名
  - 媒体域名：https://${MEDIA_DOMAIN} → 只读反代 ${MEDIA_BUCKET}
  - 上游：http://${UPSTREAM}

还需要人工完成：
  1. 腾讯云控制台 → 轻量应用服务器 → 防火墙，放行 TCP 80 和 443。
     只在系统 ufw/iptables 放行不会生效。
  2. 在开发机执行公网自检：
     task edge:check -- --expect-ip <服务器公网 IP>
     task media:check -- --object <对象键>
  3. 确认续期任务存在：
     systemctl list-timers 'certbot*'
     certbot renew --dry-run --no-random-sleep-on-renew

证书续期后由 deploy hook 自动 reload Nginx。
回滚：sudo rm -f ${NGINX_CONF} && sudo nginx -t && sudo systemctl reload nginx
EOF
}

main() {
  parse_args "$@"

  if [[ -n $PRINT_STAGE ]]; then
    render_config "$PRINT_STAGE"
    exit 0
  fi

  require_root
  require_command systemctl
  require_command curl
  ensure_packages
  ensure_nginx_running
  check_upstream
  install -d -m 0755 "$ACME_ROOT"

  local primary_cert="${CERT_ROOT}/${PRIMARY_DOMAIN}/fullchain.pem"
  local media_cert="${CERT_ROOT}/${MEDIA_DOMAIN}/fullchain.pem"

  if [[ -f $primary_cert && -f $media_cert ]]; then
    write_config tls
  else
    if [[ ! -f $primary_cert ]]; then
      # 首次安装：先写入只含 80 的配置，让 Certbot 完成 HTTP-01 校验。
      write_config http
      ensure_certificate "$PRIMARY_DOMAIN" "$PRIMARY_DOMAIN" "$WWW_DOMAIN"
    else
      # 主站证书已存在时保持 HTTPS 现状，只补媒体域名的 80 入口，
      # 这样媒体证书签发失败也不会把主站降级为 HTTP。
      write_config tls 0
    fi
    ensure_certificate "$MEDIA_DOMAIN" "$MEDIA_DOMAIN"
    write_config tls
  fi

  verify_local
  check_runtime_origins
  print_summary
}

main "$@"

#!/usr/bin/env bash

# 安装或更新生产公网入口：宿主机 Nginx 终结 TLS，把请求反代到
# 127.0.0.1:8080 上的容器栈，应用容器的端口绑定保持不变。
#
# 该脚本只在服务器上以 root 手工执行。CI 和开发机只使用 --print-config
# 做离线渲染断言，因此渲染路径不能依赖 root、网络或已安装的 Nginx。

set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly HTTP_TEMPLATE="${SCRIPT_DIR}/../edge/hxy-blog-http.conf.tmpl"
readonly TLS_TEMPLATE="${SCRIPT_DIR}/../edge/hxy-blog-tls.conf.tmpl"

# 这些路径与运行手册绑定；改名会破坏续期、备份和回滚说明，必须同步文档。
readonly NGINX_CONF="/etc/nginx/conf.d/hxy-blog.conf"
readonly ACME_ROOT="/var/www/certbot"
readonly CERT_ROOT="/etc/letsencrypt/live"
readonly BACKUP_DIR="/var/backups/hxy-blog/edge"
readonly RUNTIME_ENV="/etc/hxy-blog/runtime.env"

PRIMARY_DOMAIN="hxy2333.site"
WWW_DOMAIN="www.hxy2333.site"
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

render_config() {
  local stage=$1 mode http2_param='' http2_directive='' template

  mode=$(detect_http2_mode)
  if [[ $mode == modern ]]; then
    http2_directive='http2 on;'
  else
    http2_param=' http2'
  fi

  case "$stage" in
    http) template=$HTTP_TEMPLATE ;;
    tls) template=$TLS_TEMPLATE ;;
    *) die "未知的配置阶段：${stage}" ;;
  esac
  [[ -f $template ]] || die "缺少模板：${template}"

  # 只替换受校验的占位符；模板里的 $host、$request_uri 等 Nginx 变量保持原样。
  sed \
    -e "s|__PRIMARY_DOMAIN__|${PRIMARY_DOMAIN}|g" \
    -e "s|__WWW_DOMAIN__|${WWW_DOMAIN}|g" \
    -e "s|__UPSTREAM__|${UPSTREAM}|g" \
    -e "s|__ACME_ROOT__|${ACME_ROOT}|g" \
    -e "s|__CERT_BASE__|${CERT_ROOT}/${PRIMARY_DOMAIN}|g" \
    -e "s|__HTTP2_PARAM__|${http2_param}|g" \
    -e "s|__HTTP2_DIRECTIVE__|${http2_directive}|g" \
    "$template"
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

  rendered=$(render_config "$stage")
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

issue_certificate() {
  local cert="${CERT_ROOT}/${PRIMARY_DOMAIN}/fullchain.pem"

  if [[ -f $cert ]]; then
    log "证书已存在，跳过签发：${cert}"
    return 0
  fi
  [[ -n $EMAIL ]] || die '首次签发证书需要 --email 提供通知邮箱'

  install -d -m 0755 "$ACME_ROOT"
  log "申请 Let's Encrypt 证书：${PRIMARY_DOMAIN}、${WWW_DOMAIN}"
  # 只使用 certonly + webroot：让站点配置完全由本脚本渲染，
  # 避免 certbot 改写 Nginx 配置后在下次安装时被覆盖。
  certbot certonly --webroot --webroot-path "$ACME_ROOT" \
    --cert-name "$PRIMARY_DOMAIN" \
    -d "$PRIMARY_DOMAIN" -d "$WWW_DOMAIN" \
    --email "$EMAIL" --agree-tos --non-interactive \
    --deploy-hook 'systemctl reload nginx' ||
    die '证书签发失败；确认腾讯云防火墙已放行 80，且域名解析指向本机'
}

verify_local() {
  log '验证本机 HTTPS 入口'
  curl --noproxy '*' --fail --silent --show-error --max-time 10 \
    --resolve "${PRIMARY_DOMAIN}:443:127.0.0.1" \
    "https://${PRIMARY_DOMAIN}/healthz" >/dev/null ||
    die '本机 HTTPS 健康检查失败'

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
  - 上游：http://${UPSTREAM}

还需要人工完成：
  1. 腾讯云控制台 → 轻量应用服务器 → 防火墙，放行 TCP 80 和 443。
     只在系统 ufw/iptables 放行不会生效。
  2. 在开发机执行公网自检：
     task edge:check -- --expect-ip <服务器公网 IP>
  3. 确认续期任务存在：
     systemctl list-timers 'certbot*'
     certbot renew --dry-run

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

  local cert="${CERT_ROOT}/${PRIMARY_DOMAIN}/fullchain.pem"
  if [[ -f $cert ]]; then
    log '检测到现有证书，直接应用 HTTPS 配置'
    write_config tls
  else
    # 先写入只含 80 的配置，让 Certbot 能完成 HTTP-01 校验。
    write_config http
    issue_certificate
    write_config tls
  fi

  verify_local
  check_runtime_origins
  print_summary
}

main "$@"

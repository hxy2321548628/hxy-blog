#!/usr/bin/env bash

# 从任意一台开发机验证公网入口是否真正可用。
# 检查项覆盖 DNS、HTTP 跳转、TLS 证书、首页、API、SPA 回退、安全头和 www 跳转。
#
# 脚本默认绕过本机 HTTP/HTTPS 代理：浏览器里的 502 很可能来自代理自身的
# 错误页，而不是站点响应；只有直连结果才能代表公网真实状态。

set -Eeuo pipefail

readonly CURL_BIN="${CURL_BIN:-curl}"

PRIMARY_DOMAIN="hxy2333.site"
WWW_DOMAIN="www.hxy2333.site"
EXPECT_IP=""
CHECK_WWW=1

failures=0
checks=0

pass() {
  checks=$((checks + 1))
  printf 'ok   %s\n' "$1"
}

fail() {
  checks=$((checks + 1))
  failures=$((failures + 1))
  printf 'FAIL %s\n' "$1" >&2
}

usage() {
  cat <<'EOF'
用法：check-public-entry.sh [选项]

选项：
  --domain <域名>    主域名，默认 hxy2333.site
  --www <域名>       www 别名，默认 www.<主域名>
  --expect-ip <IP>   要求 DNS 解析包含该地址
  --skip-www         跳过 www 跳转检查
  -h, --help         显示本帮助

退出码：0 全部通过；1 存在失败项；64 参数错误。
EOF
}

die_usage() {
  printf '错误：%s\n' "$*" >&2
  exit 64
}

while (($# > 0)); do
  case "$1" in
    --domain)
      [[ $# -ge 2 ]] || die_usage '--domain 缺少参数'
      PRIMARY_DOMAIN=$2
      shift 2
      ;;
    --www)
      [[ $# -ge 2 ]] || die_usage '--www 缺少参数'
      WWW_DOMAIN=$2
      shift 2
      ;;
    --expect-ip)
      [[ $# -ge 2 ]] || die_usage '--expect-ip 缺少参数'
      EXPECT_IP=$2
      shift 2
      ;;
    --skip-www)
      CHECK_WWW=0
      shift
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

# 统一的 curl 调用：禁用代理、限制超时，并把 HTTP 状态码写入标准输出。
# 网络失败时返回空字符串，由调用方给出可读的失败原因。
http_get() {
  local url=$1
  shift
  "$CURL_BIN" --noproxy '*' --silent --show-error --connect-timeout 3 --max-time 8 \
    --write-out '%{http_code}' "$@" "$url" 2>/dev/null || true
}

# 跳转检查需要同时拿到状态码和 Location，因此单独保留一个格式化输出函数。
http_get_redirect() {
  local url=$1
  shift
  "$CURL_BIN" --noproxy '*' --silent --show-error --connect-timeout 3 --max-time 8 \
    --write-out '%{http_code} %{redirect_url}' "$@" "$url" 2>/dev/null || true
}

check_dns() {
  local ips
  ips=$(getent ahostsv4 "$PRIMARY_DOMAIN" 2>/dev/null | awk '{print $1}' | sort -u || true)
  if [[ -z $ips ]]; then
    ips=$(dig +short A "$PRIMARY_DOMAIN" 2>/dev/null | sort -u || true)
  fi
  if [[ -z $ips ]]; then
    fail "DNS 无法解析 ${PRIMARY_DOMAIN}"
    return
  fi
  pass "DNS 解析 ${PRIMARY_DOMAIN} → $(tr '\n' ' ' <<<"$ips")"

  if [[ -n $EXPECT_IP ]] && ! grep --quiet --fixed-strings -- "$EXPECT_IP" <<<"$ips"; then
    fail "DNS 未包含期望地址 ${EXPECT_IP}"
  elif [[ -n $EXPECT_IP ]]; then
    pass "DNS 包含期望地址 ${EXPECT_IP}"
  fi
}

check_http_redirect() {
  local result code location
  result=$(http_get_redirect "http://${PRIMARY_DOMAIN}/" --output /dev/null)
  code=${result%% *}
  location=${result#* }
  if [[ -z $result || $code == 000 ]]; then
    fail "http://${PRIMARY_DOMAIN}/ 不可达（检查 80 端口与防火墙）"
    return
  fi
  if [[ $code == 301 && $location == "https://${PRIMARY_DOMAIN}/" ]]; then
    pass 'HTTP 301 跳转到 HTTPS 主域名'
  else
    fail "HTTP 跳转不符合预期：${code} ${location}"
  fi
}

check_https_home() {
  local code
  code=$(http_get "https://${PRIMARY_DOMAIN}/" --output /dev/null)
  if [[ -z $code || $code == 000 ]]; then
    fail "https://${PRIMARY_DOMAIN}/ 不可达（检查 443 端口、证书与 Nginx）"
  elif [[ $code == 200 ]]; then
    pass 'HTTPS 首页返回 200'
  else
    fail "HTTPS 首页返回 ${code}"
  fi
}

# 证书有效期只作为运维提示；真正的证书校验由 curl 在 HTTPS 请求中完成。
check_certificate() {
  if ! command -v openssl >/dev/null 2>&1; then
    printf 'skip 未安装 openssl，跳过证书有效期检查\n' >&2
    return
  fi

  local summary not_after days
  summary=$(echo | timeout 6 openssl s_client -connect "${PRIMARY_DOMAIN}:443" \
    -servername "$PRIMARY_DOMAIN" 2>/dev/null |
    openssl x509 -noout -issuer -dates 2>/dev/null || true)
  if [[ -z $summary ]]; then
    fail '无法读取 TLS 证书'
    return
  fi

  not_after=$(sed -n 's/^notAfter=//p' <<<"$summary" | head -n 1)
  if [[ -z $not_after ]]; then
    fail '证书缺少 notAfter 字段'
    return
  fi
  days=$((($(date -d "$not_after" +%s) - $(date +%s)) / 86400))
  if ((days >= 14)); then
    pass "证书有效，剩余 ${days} 天（$(sed -n 's/^issuer=//p' <<<"$summary" | head -n 1)）"
  else
    fail "证书剩余 ${days} 天，需要检查自动续期"
  fi
}

check_api() {
  local body code
  body=$(mktemp)
  code=$(http_get "https://${PRIMARY_DOMAIN}/api/health" --output "$body")
  if [[ $code == 200 ]] && grep --quiet --fixed-strings '"status":"ok"' "$body"; then
    pass 'HTTPS API 健康检查返回 ok'
  else
    fail "HTTPS API 健康检查失败：HTTP ${code:-无响应}（000 表示连接失败）"
  fi
  rm -f "$body"
}

check_spa_fallback() {
  local body code
  body=$(mktemp)
  code=$(http_get "https://${PRIMARY_DOMAIN}/__edge_check__" --output "$body")
  if [[ $code == 200 ]] && grep --quiet --fixed-strings '<div id="root"></div>' "$body"; then
    pass 'SPA 未知路径回退到 index.html'
  else
    fail "SPA 回退失败：HTTP ${code:-无响应}"
  fi
  rm -f "$body"
}

check_headers() {
  local headers
  headers=$(mktemp)
  http_get "https://${PRIMARY_DOMAIN}/" --dump-header "$headers" --output /dev/null >/dev/null

  local name
  for name in 'strict-transport-security' 'x-content-type-options' 'referrer-policy'; do
    if grep --ignore-case --quiet -- "^${name}:" "$headers"; then
      pass "响应头包含 ${name}"
    else
      fail "响应头缺少 ${name}"
    fi
  done
  rm -f "$headers"
}

check_www_redirect() {
  local result code location
  result=$(http_get_redirect "https://${WWW_DOMAIN}/" --output /dev/null)
  code=${result%% *}
  location=${result#* }
  if [[ -z $result || $code == 000 ]]; then
    fail "https://${WWW_DOMAIN}/ 不可达"
    return
  fi
  if [[ $code == 301 && $location == "https://${PRIMARY_DOMAIN}/" ]]; then
    pass 'www 301 跳转到主域名'
  else
    fail "www 跳转不符合预期：${code} ${location}"
  fi
}

printf '检查公网入口：https://%s\n\n' "$PRIMARY_DOMAIN"
check_dns
check_http_redirect
check_https_home
check_certificate
check_api
check_spa_fallback
check_headers
if ((CHECK_WWW == 1)); then
  check_www_redirect
fi

printf '\n%d 项检查，%d 项失败\n' "$checks" "$failures"
if ((failures > 0)); then
  printf '提示：本脚本已绕过本机代理；若仍失败，请检查腾讯云防火墙与服务器上的 Nginx。\n' >&2
  exit 1
fi
printf '公网入口检查全部通过。\n'

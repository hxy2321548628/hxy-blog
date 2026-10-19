#!/usr/bin/env bash

# 媒体域名的公网自检：DNS、HTTP 跳转、TLS 证书、只读限制和对象读取。
# 与 check-public-entry.sh 一样默认绕过本机代理，避免代理错误页干扰判断。
#
# 该脚本既可在上线前用来确认失败原因，也可在上线后作为回归检查。

set -Eeuo pipefail

readonly CURL_BIN="${CURL_BIN:-curl}"

MEDIA_DOMAIN="media.hxy2333.site"
EXPECT_IP=""
OBJECT_KEY=""

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
用法：check-media-entry.sh [选项]

选项：
  --domain <域名>    媒体域名，默认 media.hxy2333.site
  --expect-ip <IP>   要求 DNS 解析包含该地址
  --object <对象键>  额外验证一个真实对象，例如 media/2026/10/xxx.png
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
      MEDIA_DOMAIN=$2
      shift 2
      ;;
    --expect-ip)
      [[ $# -ge 2 ]] || die_usage '--expect-ip 缺少参数'
      EXPECT_IP=$2
      shift 2
      ;;
    --object)
      [[ $# -ge 2 ]] || die_usage '--object 缺少参数'
      OBJECT_KEY=$2
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

[[ $MEDIA_DOMAIN =~ ^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$ ]] ||
  die_usage "媒体域名格式无效：${MEDIA_DOMAIN}"

# 对象键只允许安全字符，避免把任意 URL 或查询串拼进请求。
if [[ -n $OBJECT_KEY ]]; then
  [[ $OBJECT_KEY =~ ^[A-Za-z0-9._/-]+$ ]] || die_usage '--object 只接受对象键，不能包含协议、查询串或空白'
  OBJECT_KEY=${OBJECT_KEY#/}
fi

http_get() {
  local url=$1
  shift
  "$CURL_BIN" --noproxy '*' --silent --show-error --connect-timeout 3 --max-time 8 \
    --write-out '%{http_code}' "$@" "$url" 2>/dev/null || true
}

check_dns() {
  local ips
  ips=$(getent ahostsv4 "$MEDIA_DOMAIN" 2>/dev/null | awk '{print $1}' | sort -u || true)
  if [[ -z $ips ]]; then
    ips=$(dig +short A "$MEDIA_DOMAIN" 2>/dev/null | sort -u || true)
  fi
  if [[ -z $ips ]]; then
    fail "DNS 无法解析 ${MEDIA_DOMAIN}"
    return
  fi
  pass "DNS 解析 ${MEDIA_DOMAIN} → $(tr '\n' ' ' <<<"$ips")"

  if [[ -n $EXPECT_IP ]] && ! grep --quiet --fixed-strings -- "$EXPECT_IP" <<<"$ips"; then
    fail "DNS 未包含期望地址 ${EXPECT_IP}"
  elif [[ -n $EXPECT_IP ]]; then
    pass "DNS 包含期望地址 ${EXPECT_IP}"
  fi
}

check_http_redirect() {
  local result code location
  result=$("$CURL_BIN" --noproxy '*' --silent --show-error --connect-timeout 3 --max-time 8 \
    --output /dev/null --write-out '%{http_code} %{redirect_url}' \
    "http://${MEDIA_DOMAIN}/" 2>/dev/null || true)
  code=${result%% *}
  location=${result#* }
  if [[ -z $result || $code == 000 ]]; then
    fail "http://${MEDIA_DOMAIN}/ 不可达"
    return
  fi
  # 媒体域名的 80 只做 ACME 和跳转；COS 直接响应 404/200 说明 DNS 还没切到服务器。
  if [[ $code == 301 && $location == "https://${MEDIA_DOMAIN}/" ]]; then
    pass 'HTTP 301 跳转到媒体域名 HTTPS'
  else
    fail "HTTP 跳转不符合预期：${code} ${location:-（无 Location）}"
  fi
}

check_https() {
  local code
  code=$(http_get "https://${MEDIA_DOMAIN}/" --output /dev/null)
  if [[ -z $code || $code == 000 ]]; then
    fail "https://${MEDIA_DOMAIN}/ TLS 或连接失败"
  else
    # 根路径返回 COS 的 404 是正常现象，能完成 TLS 握手并通过校验即可。
    pass "HTTPS 可访问（根路径 HTTP ${code}，证书校验通过）"
  fi
}

check_certificate() {
  if ! command -v openssl >/dev/null 2>&1; then
    printf 'skip 未安装 openssl，跳过证书有效期检查\n' >&2
    return
  fi

  local cert_pem summary not_after days match
  cert_pem=$(echo | timeout 6 openssl s_client -connect "${MEDIA_DOMAIN}:443" \
    -servername "$MEDIA_DOMAIN" 2>/dev/null |
    openssl x509 -outform PEM 2>/dev/null || true)
  if [[ -z $cert_pem ]]; then
    fail '无法读取媒体域名证书'
    return
  fi

  # 只检查有效期不够：COS 默认域名的通配证书同样“有效”，但不覆盖自定义域名。
  match=$(openssl x509 -noout -checkhost "$MEDIA_DOMAIN" <<<"$cert_pem" 2>&1 || true)
  if [[ $match != *"does match certificate"* || $match == *"does NOT match"* ]]; then
    fail "证书不覆盖 ${MEDIA_DOMAIN}：${match}"
    return
  fi

  summary=$(openssl x509 -noout -issuer -dates <<<"$cert_pem" 2>/dev/null || true)
  not_after=$(sed -n 's/^notAfter=//p' <<<"$summary" | head -n 1)
  if [[ -z $not_after ]]; then
    fail '证书缺少 notAfter 字段'
    return
  fi
  days=$((($(date -d "$not_after" +%s) - $(date +%s)) / 86400))
  if ((days >= 14)); then
    pass "证书覆盖 ${MEDIA_DOMAIN}，剩余 ${days} 天（$(sed -n 's/^issuer=//p' <<<"$summary" | head -n 1)）"
  else
    fail "证书剩余 ${days} 天，需要检查自动续期"
  fi
}

check_read_only() {
  local code
  code=$("$CURL_BIN" --noproxy '*' --silent --show-error --connect-timeout 3 --max-time 8 \
    --request POST --output /dev/null --write-out '%{http_code}' \
    "https://${MEDIA_DOMAIN}/" 2>/dev/null || true)
  if [[ $code == 403 ]]; then
    pass '媒体入口拒绝 POST（只读）'
  else
    fail "媒体入口未拒绝 POST：HTTP ${code:-无响应}"
  fi
}

check_object() {
  [[ -n $OBJECT_KEY ]] || return 0

  local body code content_type
  body=$(mktemp)
  code=$("$CURL_BIN" --noproxy '*' --silent --show-error --connect-timeout 3 --max-time 20 \
    --output "$body" --write-out '%{http_code} %{content_type}' \
    "https://${MEDIA_DOMAIN}/${OBJECT_KEY}" 2>/dev/null || true)
  content_type=${code#* }
  code=${code%% *}
  if [[ $code == 200 && $content_type == image/* ]]; then
    pass "对象读取成功：${OBJECT_KEY}（${content_type}）"
  else
    fail "对象读取失败：${OBJECT_KEY}（HTTP ${code:-无响应}，Content-Type ${content_type:-无}）"
  fi
  rm -f "$body"
}

printf '检查媒体入口：https://%s\n\n' "$MEDIA_DOMAIN"
check_dns
check_http_redirect
check_https
check_certificate
check_read_only
check_object

printf '\n%d 项检查，%d 项失败\n' "$checks" "$failures"
if ((failures > 0)); then
  printf '提示：媒体域名必须解析到博客服务器，且安装脚本已签发独立证书。\n' >&2
  exit 1
fi
printf '媒体入口检查全部通过。\n'

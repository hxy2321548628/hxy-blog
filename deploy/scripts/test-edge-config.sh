#!/usr/bin/env bash

# 离线校验公网入口配置的渲染结果与参数边界。
# 该测试不读取 /etc、不启动 Nginx、不访问网络，因此可以在 CI 和开发机上无条件执行；
# 服务器上的真实签发与 reload 由 install-edge.sh 负责，并由 check-public-entry.sh 验证。
set -Eeuo pipefail

readonly SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly INSTALL="${SCRIPT_DIR}/install-edge.sh"
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

# assert_contains 检查渲染结果必须出现的片段；固定字符串匹配避免正则误伤。
assert_contains() {
  local label=$1 needle=$2 haystack=$3
  if grep --fixed-strings --quiet -- "$needle" <<<"${haystack}"; then
    pass "${label}"
  else
    fail "${label}（缺少：${needle}）"
  fi
}

assert_not_contains() {
  local label=$1 needle=$2 haystack=$3
  if grep --fixed-strings --quiet -- "$needle" <<<"${haystack}"; then
    fail "${label}（不应出现：${needle}）"
  else
    pass "${label}"
  fi
}

# assert_exit 验证失败路径的退出码；命令输出不进入测试日志。
assert_exit() {
  local expected=$1 label=$2
  shift 2
  local actual=0
  "$@" >/dev/null 2>&1 || actual=$?
  if [[ ${actual} -eq ${expected} ]]; then
    pass "${label}"
  else
    fail "${label}（期望退出码 ${expected}，实际 ${actual}）"
  fi
}

if [[ ! -f ${INSTALL} ]]; then
  printf '缺少 %s\n' "${INSTALL}" >&2
  exit 1
fi

# ---- HTTP 阶段：只做 ACME 校验和跳转，不引用尚不存在的证书 ----
http_config="$("${INSTALL}" --print-config http)"

assert_contains 'HTTP 监听 80' 'listen 80;' "${http_config}"
assert_contains 'HTTP 同时服务主域名与 www' 'server_name hxy2333.site www.hxy2333.site;' "${http_config}"
assert_contains 'HTTP 暴露 ACME 校验目录' 'location ^~ /.well-known/acme-challenge/' "${http_config}"
assert_contains 'ACME webroot 固定' 'root /var/www/certbot;' "${http_config}"
assert_contains 'HTTP 跳转到 HTTPS 主域名' 'return 301 https://hxy2333.site$request_uri;' "${http_config}"
assert_not_contains 'HTTP 阶段不引用证书' 'ssl_certificate' "${http_config}"

# ---- TLS 阶段：现代 Nginx 使用 http2 指令 ----
tls_modern="$("${INSTALL}" --print-config tls --http2 modern)"

assert_contains 'TLS 监听 443' 'listen 443 ssl;' "${tls_modern}"
assert_contains 'TLS 启用 http2 指令' 'http2 on;' "${tls_modern}"
assert_contains 'TLS 证书路径' 'ssl_certificate /etc/letsencrypt/live/hxy2333.site/fullchain.pem;' "${tls_modern}"
assert_contains 'TLS 私钥路径' 'ssl_certificate_key /etc/letsencrypt/live/hxy2333.site/privkey.pem;' "${tls_modern}"
assert_contains 'TLS 主域名' 'server_name hxy2333.site;' "${tls_modern}"
assert_contains 'www 跳转规范域名' 'server_name www.hxy2333.site;' "${tls_modern}"
assert_contains 'www 保留原路径跳转' 'return 301 https://hxy2333.site$request_uri;' "${tls_modern}"
assert_contains '代理到回环应用端口' 'proxy_pass http://127.0.0.1:8080;' "${tls_modern}"
assert_contains '传递原始协议' 'proxy_set_header X-Forwarded-Proto $scheme;' "${tls_modern}"
assert_contains '上传上限与容器一致' 'client_max_body_size 11m;' "${tls_modern}"
assert_contains '上传请求体流式转发' 'proxy_request_buffering off;' "${tls_modern}"
assert_contains '下发 HSTS' 'add_header Strict-Transport-Security "max-age=31536000" always;' "${tls_modern}"
assert_contains '隐藏 Nginx 版本' 'server_tokens off;' "${tls_modern}"
assert_not_contains 'TLS 阶段没有残留占位符' '__' "${tls_modern}"

# ---- TLS 阶段：旧版 Nginx 只能把 http2 写在 listen 参数里 ----
tls_legacy="$("${INSTALL}" --print-config tls --http2 legacy)"

assert_contains '旧版 Nginx 使用 listen 参数启用 http2' 'listen 443 ssl http2;' "${tls_legacy}"
assert_not_contains '旧版 Nginx 不重复使用 http2 指令' 'http2 on;' "${tls_legacy}"

# ---- 媒体域名：独立证书与只读反代 ----
assert_contains 'HTTP 为媒体域名提供 ACME' 'server_name media.hxy2333.site;' "${http_config}"
assert_contains 'HTTP 把媒体域名跳转到 HTTPS' 'return 301 https://media.hxy2333.site$request_uri;' "${http_config}"
assert_contains '媒体域名使用独立证书' 'ssl_certificate /etc/letsencrypt/live/media.hxy2333.site/fullchain.pem;' "${tls_modern}"
assert_contains '媒体域名反代到 COS 桶' 'proxy_pass https://hxy-blog-media-1497610660.cos.ap-nanjing.myqcloud.com;' "${tls_modern}"
assert_contains '反代保留 COS 的 Host' 'proxy_set_header Host hxy-blog-media-1497610660.cos.ap-nanjing.myqcloud.com;' "${tls_modern}"
assert_contains '反代启用 SNI' 'proxy_ssl_server_name on;' "${tls_modern}"
assert_contains '媒体入口只允许读取' 'limit_except GET HEAD' "${tls_modern}"

# ---- 参数覆盖：域名与上游端口必须来自参数而不是硬编码 ----
custom_config="$("${INSTALL}" --print-config tls --domain example.com --www blog.example.com --upstream 127.0.0.1:9090)"
assert_contains '自定义主域名' 'server_name example.com;' "${custom_config}"
assert_contains '自定义 www 域名' 'server_name blog.example.com;' "${custom_config}"
assert_contains '自定义上游端口' 'proxy_pass http://127.0.0.1:9090;' "${custom_config}"
assert_contains '自定义证书目录' 'ssl_certificate /etc/letsencrypt/live/example.com/fullchain.pem;' "${custom_config}"

media_custom="$("${INSTALL}" --print-config tls --domain example.com --www blog.example.com --media-domain cdn.example.com --media-bucket bucket-1.cos.ap-shanghai.myqcloud.com)"
assert_contains '自定义媒体域名' 'server_name cdn.example.com;' "${media_custom}"
assert_contains '自定义媒体证书目录' 'ssl_certificate /etc/letsencrypt/live/cdn.example.com/fullchain.pem;' "${media_custom}"
assert_contains '自定义媒体桶' 'proxy_pass https://bucket-1.cos.ap-shanghai.myqcloud.com;' "${media_custom}"

# ---- 失败路径：非法输入必须在渲染前被拒绝 ----
assert_exit 64 '拒绝非法域名' "${INSTALL}" --print-config tls --domain 'bad domain!'
assert_exit 64 '拒绝非法上游' "${INSTALL}" --print-config tls --upstream 'http://127.0.0.1:8080'
assert_exit 64 '拒绝未知的 http2 模式' "${INSTALL}" --print-config tls --http2 h3
assert_exit 64 '拒绝未知的配置阶段' "${INSTALL}" --print-config tls,http
assert_exit 64 '拒绝非法媒体桶' "${INSTALL}" --print-config tls --media-bucket 'http://bucket.example.com'

# ---- 幂等：相同参数必须渲染出完全一致的结果，便于重复安装 ----
first="$("${INSTALL}" --print-config tls)"
second="$("${INSTALL}" --print-config tls)"
if [[ ${first} == "${second}" ]]; then
  pass '相同参数渲染结果稳定'
else
  fail '相同参数渲染结果不稳定'
fi

printf '\n%d 项检查，%d 项失败\n' "${checks}" "${failures}"
[[ ${failures} -eq 0 ]]

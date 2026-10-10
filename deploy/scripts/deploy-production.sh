#!/usr/bin/env bash

# 部署入口只接受审批后的 production 环境和两个不可变制品身份。
set -euo pipefail
: "${DEPLOY_ENV:?必须显式指定部署环境}"
: "${HEAD_SHA:?必须指定已验证提交}"
: "${API_DIGEST:?必须指定 API Digest}"
: "${WEB_DIGEST:?必须指定 Web Digest}"
: "${DEPLOY_HOST:?必须指定部署主机}"
: "${DEPLOY_PORT:?必须指定 SSH 端口}"
: "${DEPLOY_USER:?必须指定部署用户}"
: "${DEPLOY_SSH_KEY:?必须指定部署密钥}"
: "${DEPLOY_HOST_KEY:?必须指定主机公钥}"

[[ "$DEPLOY_ENV" == production ]] || { echo '当前只支持 production 部署环境' >&2; exit 1; }
[[ "$HEAD_SHA" =~ ^[0-9a-f]{40}$ ]] || { echo 'HEAD_SHA 无效' >&2; exit 1; }
[[ "$API_DIGEST" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo 'API_DIGEST 无效' >&2; exit 1; }
[[ "$WEB_DIGEST" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo 'WEB_DIGEST 无效' >&2; exit 1; }
[[ "$DEPLOY_PORT" =~ ^[0-9]+$ ]] || { echo 'DEPLOY_PORT 无效' >&2; exit 1; }

# 临时文件只给当前 Runner 用户读取；退出时删除，避免凭据留在工作区。
umask 077
ssh_dir="$(mktemp -d)"
trap 'rm -rf "$ssh_dir"' EXIT
printf '%s\n' "$DEPLOY_SSH_KEY" > "$ssh_dir/deploy_key"
printf '%s\n' "$DEPLOY_HOST_KEY" > "$ssh_dir/known_hosts"

# 主机公钥必须匹配；远端公钥绑定服务器固定命令，无法执行任意 Shell。
ssh \
  -o BatchMode=yes \
  -o IdentitiesOnly=yes \
  -o StrictHostKeyChecking=yes \
  -o UserKnownHostsFile="$ssh_dir/known_hosts" \
  -i "$ssh_dir/deploy_key" \
  -p "$DEPLOY_PORT" \
  "${DEPLOY_USER}@${DEPLOY_HOST}" \
  "deploy ${HEAD_SHA} ${API_DIGEST} ${WEB_DIGEST}"

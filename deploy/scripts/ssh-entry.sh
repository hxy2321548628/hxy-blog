#!/usr/bin/env bash

set -Eeuo pipefail

readonly original_command=${SSH_ORIGINAL_COMMAND:-}

# 禁止 eval，只接受四个由空格分隔且格式固定的字段。
read -r action revision api_digest web_digest extra <<< "${original_command}"

if [[ ${action:-} != deploy || -n ${extra:-} ]]; then
  printf '拒绝：只允许 deploy <commit> <api-digest> <web-digest>。\n' >&2
  exit 64
fi

if [[ ! ${revision:-} =~ ^[0-9a-f]{40}$ ]]; then
  printf '拒绝：Commit 格式无效。\n' >&2
  exit 64
fi

if [[ ! ${api_digest:-} =~ ^sha256:[0-9a-f]{64}$ || ! ${web_digest:-} =~ ^sha256:[0-9a-f]{64}$ ]]; then
  printf '拒绝：镜像 Digest 格式无效。\n' >&2
  exit 64
fi

exec sudo -n /usr/local/sbin/hxy-blog-deploy "${revision}" "${api_digest}" "${web_digest}"

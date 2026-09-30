#!/usr/bin/env bash

# 该脚本是 hxy-deploy 公钥绑定的 forced-command。即使私钥泄露，调用方也
# 不能获得交互式 Shell，只能请求部署一个格式正确的 Commit 与两个 Digest。
set -Eeuo pipefail

# sshd 把客户端原始命令放在 SSH_ORIGINAL_COMMAND；未提供命令时按空字符串处理。
readonly original_command=${SSH_ORIGINAL_COMMAND:-}

# 禁止 eval，只接受四个由空格分隔且格式固定的字段。
# 第五个 extra 用于发现并拒绝任何额外参数。
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

# -n 禁止 sudo 询问密码；sudoers 只允许这一条 root 脚本。exec 让退出码原样返回 Actions。
exec sudo -n /usr/local/sbin/hxy-blog-deploy "${revision}" "${api_digest}" "${web_digest}"

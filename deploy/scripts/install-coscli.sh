#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly COSCLI_VERSION=1.0.9
readonly COSCLI_URL="https://github.com/tencentyun/coscli/releases/download/v${COSCLI_VERSION}/coscli-v${COSCLI_VERSION}-linux-amd64"
readonly COSCLI_SHA256=a07de5ba2800147a700ed29036b0c76a4229088cee68e1682d0eae19b638a915
readonly COSCLI_PATH=/usr/local/bin/coscli

fail() {
  printf 'COSCLI 安装失败：%s\n' "$1" >&2
  exit 1
}

if [[ ${EUID} -ne 0 ]]; then
  fail '必须由 root 执行'
fi

[[ $(uname -m) == x86_64 ]] || fail '当前只固定了 Linux AMD64 二进制文件'

for command_name in curl install mktemp sha256sum; do
  command -v "${command_name}" >/dev/null || fail "缺少命令 ${command_name}"
done

temporary_directory=$(mktemp -d)
trap 'rm -rf -- "${temporary_directory}"' EXIT
readonly download_path="${temporary_directory}/coscli"

# 使用固定版本和 SHA-256，避免服务器在无审核的情况下跟随最新版本。
curl \
  --fail \
  --location \
  --retry 3 \
  --connect-timeout 10 \
  --max-time 300 \
  --output "${download_path}" \
  "${COSCLI_URL}"

printf '%s  %s\n' "${COSCLI_SHA256}" "${download_path}" | sha256sum --check --status ||
  fail 'COSCLI SHA-256 校验失败'

install -o root -g root -m 755 "${download_path}" "${COSCLI_PATH}"
printf 'COSCLI 安装成功：version=%s path=%s\n' "${COSCLI_VERSION}" "${COSCLI_PATH}"

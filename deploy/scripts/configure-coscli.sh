#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly COSCLI_PATH=/usr/local/bin/coscli
readonly COSCLI_SHA256=a07de5ba2800147a700ed29036b0c76a4229088cee68e1682d0eae19b638a915
readonly CONFIG_DIRECTORY=/etc/hxy-blog
readonly CONFIG_PATH="${CONFIG_DIRECTORY}/cos-backup.yaml"
readonly BUCKET_NAME=hxy-blog-backup-1497610660
readonly BUCKET_ENDPOINT=cos.ap-nanjing.myqcloud.com
readonly BUCKET_ALIAS=hxy-backup

temporary_config=

fail() {
  printf 'COSCLI 配置失败：%s\n' "$1" >&2
  exit 1
}

cleanup() {
  [[ -z ${temporary_config} ]] || rm -f -- "${temporary_config}"
}
trap cleanup EXIT

if [[ ${EUID} -ne 0 ]]; then
  fail '必须由 root 执行'
fi

[[ -t 0 ]] || fail '必须在交互式终端中执行，禁止通过管道传入密钥'

for command_name in cut grep install mktemp sha256sum stat; do
  command -v "${command_name}" >/dev/null || fail "缺少命令 ${command_name}"
done

[[ -x ${COSCLI_PATH} ]] || fail "缺少 ${COSCLI_PATH}"
[[ $(stat -c '%u' "${COSCLI_PATH}") == 0 ]] || fail "${COSCLI_PATH} 必须由 root 拥有"
[[ $(sha256sum "${COSCLI_PATH}" | cut -d ' ' -f 1) == "${COSCLI_SHA256}" ]] ||
  fail 'COSCLI 二进制文件校验失败'

printf '请输入新建 CAM 子用户的 SecretId（输入内容不会上传）：'
IFS= read -r secret_id
printf '请输入 SecretKey（屏幕不显示）：'
IFS= read -r -s secret_key
printf '\n请再次输入 SecretKey：'
IFS= read -r -s secret_key_confirmation
printf '\n'

[[ ${secret_id} =~ ^AKID[0-9A-Za-z]{16,128}$ ]] || fail 'SecretId 格式无效'
[[ ${secret_key} =~ ^[0-9A-Za-z]{16,128}$ ]] || fail 'SecretKey 格式无效'
[[ ${secret_key} == "${secret_key_confirmation}" ]] || fail '两次输入的 SecretKey 不一致'

install -d -o root -g root -m 700 "${CONFIG_DIRECTORY}"
temporary_config=$(mktemp --suffix=.yaml "${CONFIG_DIRECTORY}/.cos-backup.XXXXXX")

# 密钥只经标准输入传给 COSCLI，不进入命令行、Shell 历史或日志。
{
  printf '%s\n' \
    "${temporary_config}" \
    SecretKey \
    "${secret_id}" \
    "${secret_key}" \
    '' \
    false \
    true \
    false \
    '' \
    "${BUCKET_NAME}" \
    "${BUCKET_ENDPOINT}" \
    "${BUCKET_ALIAS}" \
    false
} | "${COSCLI_PATH}" config init -c "${temporary_config}" --disable-log >/dev/null

# COSCLI 内置加密只是辅助防护，root 专属文件权限才是主要边界。
grep -Fq '    mode: SecretKey' "${temporary_config}" || fail '身份模式写入失败'
grep -Fq '    disableencryption: "false"' "${temporary_config}" || fail '密钥加密未启用'
if grep -Fq "${secret_id}" "${temporary_config}" || grep -Fq "${secret_key}" "${temporary_config}"; then
  fail '拒绝保存明文密钥'
fi

unset secret_id secret_key secret_key_confirmation
install -o root -g root -m 600 "${temporary_config}" "${CONFIG_PATH}"
printf 'COSCLI 凭证配置成功：path=%s\n' "${CONFIG_PATH}"

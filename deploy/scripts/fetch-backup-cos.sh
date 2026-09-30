#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly BACKUP_DIR=/var/backups/hxy-blog/mysql
readonly BACKUP_LOCK=/run/lock/hxy-blog-db-backup.lock
readonly COSCLI_PATH=/usr/local/bin/coscli
readonly COSCLI_SHA256=a07de5ba2800147a700ed29036b0c76a4229088cee68e1682d0eae19b638a915
readonly COS_CONFIG=/etc/hxy-blog/cos-backup.yaml
readonly COS_ALIAS=hxy-backup

fail() {
  printf '从 COS 取回备份失败：%s\n' "$1" >&2
  exit 1
}

if [[ ${EUID} -ne 0 ]]; then
  fail '必须由 root 执行'
fi

# 兼容早期秒级时间戳和当前 GNU date 生成的 9 位纳秒时间戳。
if [[ $# -ne 1 || ! $1 =~ ^[0-9]{8}T[0-9]{6}([0-9]{9})?Z-sha-[0-9a-f]{12}\.sql\.gz$ ]]; then
  fail '参数必须是 COS 中的备份文件名'
fi

for command_name in cut find flock grep gzip install mktemp sha256sum stat; do
  command -v "${command_name}" >/dev/null || fail "缺少命令 ${command_name}"
done

[[ -x ${COSCLI_PATH} ]] || fail "缺少 ${COSCLI_PATH}"
[[ $(stat -c '%u' "${COSCLI_PATH}") == 0 ]] || fail "${COSCLI_PATH} 必须由 root 拥有"
[[ $(sha256sum "${COSCLI_PATH}" | cut -d ' ' -f 1) == "${COSCLI_SHA256}" ]] ||
  fail 'COSCLI 二进制文件校验失败'
[[ -f ${COS_CONFIG} ]] || fail "缺少 ${COS_CONFIG}"
[[ $(stat -c '%u' "${COS_CONFIG}") == 0 ]] || fail "${COS_CONFIG} 必须由 root 拥有"
if [[ -n $(find "${COS_CONFIG}" -maxdepth 0 -perm /077 -print) ]]; then
  fail "${COS_CONFIG} 权限必须不高于 600"
fi
grep -Fq '    mode: SecretKey' "${COS_CONFIG}" || fail 'COSCLI 未配置专用 CAM 子用户'
grep -Fq '    disableencryption: "false"' "${COS_CONFIG}" || fail 'COSCLI 密钥加密未启用'

exec 9>"${BACKUP_LOCK}"
flock -n 9 || fail '数据库备份或其他恢复操作正在执行'

readonly archive_name=$1
readonly checksum_name="${archive_name}.sha256"
readonly archive_path="${BACKUP_DIR}/${archive_name}"
readonly checksum_path="${BACKUP_DIR}/${checksum_name}"
[[ ! -e ${archive_path} && ! -e ${checksum_path} ]] || fail '本地已存在同名备份'

temporary_directory=$(mktemp -d)
trap 'rm -rf -- "${temporary_directory}"' EXIT
readonly downloaded_archive="${temporary_directory}/${archive_name}"
readonly downloaded_checksum="${temporary_directory}/${checksum_name}"

cos_cp() {
  "${COSCLI_PATH}" cp "$@" \
    --config-path "${COS_CONFIG}" \
    --bucket-type COS \
    --disable-log \
    --process-log=false \
    --fail-output=false
}

cos_cp "cos://${COS_ALIAS}/mysql/${archive_name}" "${downloaded_archive}" ||
  fail '备份文件下载失败'
cos_cp "cos://${COS_ALIAS}/mysql/${checksum_name}" "${downloaded_checksum}" ||
  fail '校验文件下载失败'
(
  cd "${temporary_directory}"
  sha256sum --check --status "${checksum_name}"
) || fail '远端备份 SHA-256 校验失败'
gzip -t "${downloaded_archive}" || fail '远端备份压缩文件损坏'

install -d -o root -g root -m 700 "${BACKUP_DIR}"
install -o root -g root -m 600 "${downloaded_archive}" "${archive_path}"
install -o root -g root -m 600 "${downloaded_checksum}" "${checksum_path}"

printf '已从 COS 取回并校验备份：%s\n' "${archive_path}"

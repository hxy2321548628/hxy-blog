#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly BACKUP_DIR=/var/backups/hxy-blog/mysql
readonly COSCLI_PATH=/usr/local/bin/coscli
readonly COSCLI_SHA256=a07de5ba2800147a700ed29036b0c76a4229088cee68e1682d0eae19b638a915
readonly COS_CONFIG=/etc/hxy-blog/cos-backup.yaml
readonly COS_ALIAS=hxy-backup

fail() {
  printf 'COS 备份同步失败：%s\n' "$1" >&2
  exit 1
}

if [[ ${EUID} -ne 0 ]]; then
  fail '必须由 root 执行'
fi

if [[ $# -ne 1 ]]; then
  fail '参数必须是备份文件绝对路径'
fi

for command_name in cmp cut find gzip mktemp realpath sha256sum stat; do
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

backup_path=$(realpath -e -- "$1") || fail '备份文件不存在'
[[ ${backup_path} == "${BACKUP_DIR}/"* && ${backup_path} == *.sql.gz ]] ||
  fail '只允许同步受管备份目录中的 .sql.gz 文件'

readonly archive_name=$(basename "${backup_path}")
readonly checksum_path="${backup_path}.sha256"
readonly checksum_name="${archive_name}.sha256"
[[ -f ${checksum_path} ]] || fail '缺少 SHA-256 校验文件'

(
  cd "${BACKUP_DIR}"
  sha256sum --check --status "${checksum_name}"
) || fail '本地备份 SHA-256 校验失败'
gzip -t "${backup_path}" || fail '本地备份压缩文件损坏'

cos_cp() {
  "${COSCLI_PATH}" cp "$@" \
    --config-path "${COS_CONFIG}" \
    --bucket-type COS \
    --disable-log \
    --process-log=false \
    --fail-output=false
}

readonly archive_object="cos://${COS_ALIAS}/mysql/${archive_name}"
readonly checksum_object="cos://${COS_ALIAS}/mysql/${checksum_name}"

# 禁止覆盖同名对象，避免一份已存档备份被静默替换。
cos_cp "${backup_path}" "${archive_object}" \
  --forbid-overwrite \
  --encryption-type SSE-COS \
  --server-side-encryption AES256 || fail '备份文件上传失败'
cos_cp "${checksum_path}" "${checksum_object}" \
  --forbid-overwrite \
  --encryption-type SSE-COS \
  --server-side-encryption AES256 || fail '校验文件上传失败'

verification_directory=$(mktemp -d)
trap 'rm -rf -- "${verification_directory}"' EXIT
readonly downloaded_archive="${verification_directory}/${archive_name}"
readonly downloaded_checksum="${verification_directory}/${checksum_name}"

# 上传成功后立即回读，使“可上传”和“可恢复”在同一次任务中得到验证。
cos_cp "${archive_object}" "${downloaded_archive}" || fail '备份文件回读失败'
cos_cp "${checksum_object}" "${downloaded_checksum}" || fail '校验文件回读失败'
cmp --silent "${checksum_path}" "${downloaded_checksum}" || fail '远端校验文件与本地不一致'
(
  cd "${verification_directory}"
  sha256sum --check --status "${checksum_name}"
) || fail '从 COS 回读的备份 SHA-256 校验失败'
gzip -t "${downloaded_archive}" || fail '从 COS 回读的压缩文件损坏'

printf 'COS 备份同步并回读校验成功：%s\n' "${archive_object}"

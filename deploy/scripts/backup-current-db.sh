#!/usr/bin/env bash

# systemd Timer 调用的薄封装：从当前 release.env 读取 Commit，再复用与部署
# 相同的备份脚本。这样定时备份和部署前备份具有相同格式、校验和 COS 语义。
set -Eeuo pipefail

readonly RELEASE_ENV=/opt/hxy-blog/release.env
readonly BACKUP_SCRIPT=/usr/local/sbin/hxy-blog-db-backup

fail() {
  printf '定时数据库备份失败：%s\n' "$1" >&2
  exit 1
}

if [[ ${EUID} -ne 0 ]]; then
  fail '必须由 root 执行'
fi

for command_name in sed stat; do
  command -v "${command_name}" >/dev/null || fail "缺少命令 ${command_name}"
done

# 只信任 root 拥有的发布状态，避免用攻击者伪造的 Commit 命名备份。
[[ -f ${RELEASE_ENV} ]] || fail "缺少 ${RELEASE_ENV}"
[[ $(stat -c '%u' "${RELEASE_ENV}") == 0 ]] || fail "${RELEASE_ENV} 必须由 root 拥有"
[[ -x ${BACKUP_SCRIPT} ]] || fail "缺少 ${BACKUP_SCRIPT}"

# 不直接 source 环境文件，只提取并校验定时备份所需的 Commit。
# source 会执行文件中的任意 Shell；sed 只读取一个明确字段，攻击面更小。
mapfile -t revisions < <(sed -n 's/^APP_REVISION=//p' "${RELEASE_ENV}")
[[ ${#revisions[@]} -eq 1 && ${revisions[0]} =~ ^[0-9a-f]{40}$ ]] ||
  fail 'APP_REVISION 缺失、重复或格式无效'

# exec 保留真实备份脚本的退出码，使 systemd 能准确记录成功或失败。
exec "${BACKUP_SCRIPT}" "${revisions[0]}"

#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly BACKUP_DIR=/var/backups/hxy-blog/mysql
readonly BACKUP_LOCK=/run/lock/hxy-blog-db-backup.lock
readonly RETAINED_BACKUPS=7

fail() {
  printf '数据库备份失败：%s\n' "$1" >&2
  exit 1
}

if [[ ${EUID} -ne 0 ]]; then
  fail '必须由 root 执行'
fi

if [[ $# -ne 1 || ! $1 =~ ^[0-9a-f]{40}$ ]]; then
  fail '参数必须是完整的 40 位 Commit'
fi

for command_name in docker gzip sha256sum flock; do
  command -v "${command_name}" >/dev/null || fail "缺少命令 ${command_name}"
done

exec 9>"${BACKUP_LOCK}"
flock -n 9 || fail '已有数据库备份正在执行'

mapfile -t mysql_containers < <(
  docker ps \
    --filter label=com.docker.compose.project=hxy-blog-production \
    --filter label=com.docker.compose.service=mysql \
    --format '{{.ID}}'
)
[[ ${#mysql_containers[@]} -eq 1 ]] || fail '必须存在且只能存在一个生产 MySQL 容器'

readonly mysql_container=${mysql_containers[0]}
[[ $(docker inspect --format '{{if .State.Health}}{{.State.Health.Status}}{{end}}' "${mysql_container}") == healthy ]] ||
  fail '生产 MySQL 容器未处于 healthy 状态'

install -d -o root -g root -m 700 "${BACKUP_DIR}"

readonly timestamp=$(date -u +%Y%m%dT%H%M%S%NZ)
readonly archive_name="${timestamp}-sha-${1:0:12}.sql.gz"
readonly archive_path="${BACKUP_DIR}/${archive_name}"
dump_file=$(mktemp "${BACKUP_DIR}/.dump.XXXXXX.sql")
archive_file=$(mktemp "${BACKUP_DIR}/.archive.XXXXXX.sql.gz")
trap 'rm -f -- "${dump_file}" "${archive_file}"' EXIT

# 密码只在 MySQL 容器内部通过环境变量读取，不写入宿主机命令参数或日志。
docker exec "${mysql_container}" sh -ec '
  MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysqldump \
    --host=127.0.0.1 \
    --user=root \
    --single-transaction \
    --quick \
    --routines \
    --triggers \
    --events \
    --hex-blob \
    --no-tablespaces \
    --set-gtid-purged=OFF \
    --default-character-set=utf8mb4 \
    "${MYSQL_DATABASE}"
' > "${dump_file}"

[[ -s ${dump_file} ]] || fail 'mysqldump 结果为空'
gzip -c "${dump_file}" > "${archive_file}"
gzip -t "${archive_file}"
install -o root -g root -m 600 "${archive_file}" "${archive_path}"
(
  cd "${BACKUP_DIR}"
  sha256sum "${archive_name}" > "${archive_name}.sha256"
  chmod 600 "${archive_name}.sha256"
)

# 单机即时备份只保留最近七份，异机备份另行建设。
mapfile -t backups < <(
  find "${BACKUP_DIR}" -maxdepth 1 -type f -name '*.sql.gz' -printf '%f\n' | sort --reverse
)
for ((index = RETAINED_BACKUPS; index < ${#backups[@]}; index++)); do
  old_backup="${BACKUP_DIR}/${backups[index]}"
  rm -f -- "${old_backup}" "${old_backup}.sha256"
done

printf '数据库备份成功：%s\n' "${archive_path}"

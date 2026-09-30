#!/usr/bin/env bash

# 创建生产 MySQL 的一致性逻辑备份，生成 SHA-256，上传 COS 并回读校验，
# 最后才轮换本地旧文件。脚本不会删除任何 COS 对象。
set -Eeuo pipefail
# 077 让临时 SQL、压缩包和校验文件从创建时起就只有 root 可读写。
umask 077

readonly BACKUP_DIR=/var/backups/hxy-blog/mysql
readonly BACKUP_LOCK=/run/lock/hxy-blog-db-backup.lock
readonly SYNC_SCRIPT=/usr/local/sbin/hxy-blog-db-sync
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
[[ -x ${SYNC_SCRIPT} ]] || fail "缺少 ${SYNC_SCRIPT}"
[[ $(stat -c '%u' "${SYNC_SCRIPT}") == 0 ]] || fail "${SYNC_SCRIPT} 必须由 root 拥有"

exec 9>"${BACKUP_LOCK}"
# 部署前备份、定时备份、取回和恢复演练共享此锁，避免同时操作同一数据目录。
flock -n 9 || fail '已有数据库备份正在执行'

# Compose 自动写入 project/service 标签；按标签查找比依赖易变化的容器名更稳定。
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

# UTC 纳秒时间戳加 Commit 前 12 位，使文件名可排序、可追溯且极难碰撞。
readonly timestamp=$(date -u +%Y%m%dT%H%M%S%NZ)
readonly archive_name="${timestamp}-sha-${1:0:12}.sql.gz"
readonly archive_path="${BACKUP_DIR}/${archive_name}"
dump_file=$(mktemp "${BACKUP_DIR}/.dump.XXXXXX.sql")
archive_file=$(mktemp "${BACKUP_DIR}/.archive.XXXXXX.sql.gz")
# 无论成功或失败都删除未完成临时文件；正式文件由 install 原子复制后才出现。
trap 'rm -f -- "${dump_file}" "${archive_file}"' EXIT

# 密码只在 MySQL 容器内部通过环境变量读取，不写入宿主机命令参数或日志。
# single-transaction 在 InnoDB 上获得一致性快照，quick 流式读取以控制内存占用。
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

# 同时验证非空、gzip 结构和 SHA-256；三层检查分别覆盖命令失败、压缩损坏和传输损坏。
[[ -s ${dump_file} ]] || fail 'mysqldump 结果为空'
gzip -c "${dump_file}" > "${archive_file}"
gzip -t "${archive_file}"
install -o root -g root -m 600 "${archive_file}" "${archive_path}"
(
  cd "${BACKUP_DIR}"
  sha256sum "${archive_name}" > "${archive_name}.sha256"
  chmod 600 "${archive_name}.sha256"
)

# 先完成 COS 异地副本和回读校验，再轮换服务器本地旧备份。
# 因此即使 COS 不可用，本地已有备份也不会被本次任务提前删除。
"${SYNC_SCRIPT}" "${archive_path}"

# 服务器本地只保留最近七份，COS 由存储桶生命周期单独管理。
mapfile -t backups < <(
  find "${BACKUP_DIR}" -maxdepth 1 -type f -name '*.sql.gz' -printf '%f\n' | sort --reverse
)
# 文件名按时间倒序排列，从索引 7 起都是超过保留数量的旧备份。
for ((index = RETAINED_BACKUPS; index < ${#backups[@]}; index++)); do
  old_backup="${BACKUP_DIR}/${backups[index]}"
  rm -f -- "${old_backup}" "${old_backup}.sha256"
done

printf '数据库备份成功：%s\n' "${archive_path}"

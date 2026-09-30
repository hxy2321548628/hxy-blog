#!/usr/bin/env bash

# 灾难恢复演练入口：验证备份后导入固定隔离库，读取表数量与 Goose 版本，
# 最后自动删除隔离库。任何路径都不会连接或覆盖生产业务库。
set -Eeuo pipefail
umask 077

readonly BACKUP_DIR=/var/backups/hxy-blog/mysql
readonly BACKUP_LOCK=/run/lock/hxy-blog-db-backup.lock
readonly DRILL_DATABASE=hxy_blog_restore_drill

fail() {
  printf '数据库恢复演练失败：%s\n' "$1" >&2
  exit 1
}

if [[ ${EUID} -ne 0 ]]; then
  fail '必须由 root 执行'
fi

if [[ $# -ne 1 ]]; then
  fail '参数必须是备份文件绝对路径'
fi

for command_name in docker flock gzip realpath sha256sum; do
  command -v "${command_name}" >/dev/null || fail "缺少命令 ${command_name}"
done

exec 9>"${BACKUP_LOCK}"
# 恢复和备份不能并发，否则演练可能读到尚未完成的文件。
flock -n 9 || fail '数据库备份或其他恢复演练正在执行'

# 规范化并限制绝对路径，拒绝恢复备份目录以外的任意 gzip 文件。
backup_path=$(realpath -e -- "$1") || fail '备份文件不存在'
[[ ${backup_path} == "${BACKUP_DIR}/"* && ${backup_path} == *.sql.gz ]] ||
  fail '只允许恢复受管备份目录中的 .sql.gz 文件'
[[ -f ${backup_path}.sha256 ]] || fail '缺少 SHA-256 校验文件'

(
  cd "${BACKUP_DIR}"
  sha256sum --check --status "$(basename "${backup_path}").sha256"
) || fail '备份文件 SHA-256 校验失败'
gzip -t "${backup_path}" || fail '备份压缩文件损坏'

# 必须精确找到一个生产 MySQL 容器；零个或多个都可能表示部署状态异常。
mapfile -t mysql_containers < <(
  docker ps \
    --filter label=com.docker.compose.project=hxy-blog-production \
    --filter label=com.docker.compose.service=mysql \
    --format '{{.ID}}'
)
[[ ${#mysql_containers[@]} -eq 1 ]] || fail '必须存在且只能存在一个生产 MySQL 容器'
readonly mysql_container=${mysql_containers[0]}

mysql_root() {
  # 密码仅在容器内展开；外层参数通过 "$@" 传给 mysql，不做 eval。
  docker exec "${mysql_container}" sh -ec '
    MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysql \
      --host=127.0.0.1 \
      --user=root \
      --batch \
      --skip-column-names \
      "$@"
  ' sh "$@"
}

cleanup() {
  # 即使导入或统计失败，也尽力删除隔离库；清理失败不覆盖原始错误退出码。
  mysql_root --execute "DROP DATABASE IF EXISTS \`${DRILL_DATABASE}\`;" >/dev/null 2>&1 || true
}
trap cleanup EXIT

# 恢复只发生在隔离数据库中，不覆盖生产库。
mysql_root --execute "
  DROP DATABASE IF EXISTS \`${DRILL_DATABASE}\`;
  CREATE DATABASE \`${DRILL_DATABASE}\`
    CHARACTER SET utf8mb4
    COLLATE utf8mb4_0900_ai_ci;
"
# 流式解压并导入，不在磁盘上生成第二份明文 SQL。
gzip --decompress --stdout "${backup_path}" |
  docker exec --interactive "${mysql_container}" sh -ec '
    MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysql \
      --host=127.0.0.1 \
      --user=root \
      --default-character-set=utf8mb4 \
      "$1"
  ' sh "${DRILL_DATABASE}"

# 表数量证明导入结果可查询；Goose 版本证明备份包含预期 Schema 状态。
table_count=$(mysql_root --execute "
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = '${DRILL_DATABASE}';
")

goose_version=not-in-backup
if [[ $(mysql_root --execute "
  SELECT COUNT(*)
  FROM information_schema.tables
  WHERE table_schema = '${DRILL_DATABASE}'
    AND table_name = 'goose_db_version';
") == 1 ]]; then
  goose_version=$(mysql_root --execute '
    SELECT COALESCE(MAX(version_id), 0)
    FROM goose_db_version
    WHERE is_applied = 1;
  ' "${DRILL_DATABASE}")
fi

printf '数据库恢复演练成功：tables=%s goose_version=%s\n' "${table_count}" "${goose_version}"

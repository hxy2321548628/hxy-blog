#!/usr/bin/env bash

# 回填历史文章中的媒体地址。
#
# 生产数据修复脚本，只在服务器上以 root 手工执行：
# 默认只统计受影响行数并列出示例，加 --apply 才会写入。
# 通过 --from/--to 也可以反向替换，用于回滚到 COS 默认域名。

set -Eeuo pipefail

readonly APP_DIR=/opt/hxy-blog
readonly COMPOSE_FILE="${APP_DIR}/compose.yaml"
readonly RUNTIME_ENV=/etc/hxy-blog/runtime.env
readonly RELEASE_ENV="${APP_DIR}/release.env"

FROM_BASE=""
TO_BASE="https://media.hxy2333.site"
APPLY=0

log() { printf '[media-rewrite] %s\n' "$*"; }
die() {
  printf '[media-rewrite] 错误：%s\n' "$*" >&2
  exit 1
}
die_usage() {
  printf '[media-rewrite] 错误：%s\n' "$*" >&2
  exit 64
}

usage() {
  cat <<'EOF'
用法：rewrite-media-urls.sh [选项]

默认读取生产 runtime.env 的 MEDIA_COS_BUCKET_URL 作为旧地址，
把文章正文中的它替换为 https://media.hxy2333.site。
不带 --apply 时只统计并列出受影响文章，不修改数据库。

选项：
  --from <https://旧域名>   旧地址，默认取 MEDIA_COS_BUCKET_URL
  --to <https://新域名>     新地址，默认 https://media.hxy2333.site
  --apply                   真正执行 UPDATE
  -h, --help                显示本帮助

执行顺序建议：先回填，再修改 MEDIA_PUBLIC_BASE_URL 并重启 api，
这样重启会同时清掉公开文章的进程内缓存。
EOF
}

while (($# > 0)); do
  case "$1" in
    --from)
      [[ $# -ge 2 ]] || die_usage '--from 缺少参数'
      FROM_BASE=$2
      shift 2
      ;;
    --to)
      [[ $# -ge 2 ]] || die_usage '--to 缺少参数'
      TO_BASE=$2
      shift 2
      ;;
    --apply)
      APPLY=1
      shift
      ;;
    -h | --help)
      usage
      exit 0
      ;;
    *)
      usage >&2
      exit 64
      ;;
  esac
done

# 只接受不带路径的 HTTPS 源，避免把引号、分号或路径拼进 SQL。
validate_base() {
  [[ $1 =~ ^https://[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?(\.[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?)+$ ]]
}

validate_base "$TO_BASE" || die_usage "新地址必须是 HTTPS 源：${TO_BASE}"

if [[ ${EUID} -ne 0 ]]; then
  printf '[media-rewrite] 错误：请以 root 执行（sudo %s）\n' "$0" >&2
  exit 77
fi
[[ -f $COMPOSE_FILE ]] || die "缺少 ${COMPOSE_FILE}"
[[ -f $RUNTIME_ENV ]] || die "缺少 ${RUNTIME_ENV}"
[[ -f $RELEASE_ENV ]] || die "缺少 ${RELEASE_ENV}"

if [[ -z $FROM_BASE ]]; then
  FROM_BASE=$(grep -E '^MEDIA_COS_BUCKET_URL=' "$RUNTIME_ENV" | tail -n 1 | cut -d= -f2- | tr -d '"' || true)
fi
[[ -n $FROM_BASE ]] || die '无法确定旧地址，请显式传入 --from'
validate_base "$FROM_BASE" || die_usage "旧地址必须是 HTTPS 源：${FROM_BASE}"
[[ $FROM_BASE != "$TO_BASE" ]] || die '新旧地址相同，无需回填'

umask 077
sql_file=$(mktemp)
trap 'rm -f "$sql_file"' EXIT

# 使用容器内的 root 密码环境变量，避免密码出现在宿主机命令行和进程列表。
run_sql() {
  docker compose \
    --project-name hxy-blog-production \
    --env-file "$RUNTIME_ENV" \
    --env-file "$RELEASE_ENV" \
    --file "$COMPOSE_FILE" \
    exec -T mysql \
    sh -c 'exec env MYSQL_PWD="$MYSQL_ROOT_PASSWORD" mysql --default-character-set=utf8mb4 -uroot "$MYSQL_DATABASE"' <"$1"
}

cat >"$sql_file" <<SQL
SELECT COUNT(*) AS affected_posts
FROM posts
WHERE content_markdown LIKE CONCAT('%', '${FROM_BASE}', '%');

SELECT id, slug, title
FROM posts
WHERE content_markdown LIKE CONCAT('%', '${FROM_BASE}', '%')
ORDER BY id;
SQL

log "旧地址：${FROM_BASE}"
log "新地址：${TO_BASE}"

if ((APPLY == 0)); then
  log '试运行，不修改数据库；确认无误后加 --apply 执行'
  run_sql "$sql_file"
  exit 0
fi

cat >"$sql_file" <<SQL
UPDATE posts
SET content_markdown = REPLACE(content_markdown, '${FROM_BASE}', '${TO_BASE}')
WHERE content_markdown LIKE CONCAT('%', '${FROM_BASE}', '%');

SELECT ROW_COUNT() AS updated_posts;

SELECT COUNT(*) AS remaining_posts
FROM posts
WHERE content_markdown LIKE CONCAT('%', '${FROM_BASE}', '%');
SQL

log '开始回填'
run_sql "$sql_file"
log '回填完成；请随后修改 MEDIA_PUBLIC_BASE_URL 并重启 api，以清除公开文章缓存'

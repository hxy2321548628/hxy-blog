#!/usr/bin/env bash
# 任一断言失败即停止；无论成功或失败，都删除隔离 MySQL 卷。
set -euo pipefail
# 每次使用独立 Compose 项目名，清理 --volumes 时不会碰开发者的 hxy-blog 数据卷。
export COMPOSE_PROJECT_NAME="hxy-blog-ci-migration-$$"
cleanup() {
  local status=$?
  docker compose --env-file .env.example -f deploy/compose.yaml down --volumes || status=1
  exit "$status"
}
trap cleanup EXIT
# --wait 会等到 Compose 中的 MySQL healthcheck 成功后再返回。
docker compose --env-file .env.example -f deploy/compose.yaml up -d mysql --wait

# 使用即将发布的 API 镜像执行真实 MySQL 迁移，避免只验证 SQL 语法。
docker compose --env-file .env.example -f deploy/compose.yaml \
  run --rm --no-deps --entrypoint /app/migrate api up
docker compose --env-file .env.example -f deploy/compose.yaml \
  run --rm --no-deps --entrypoint /app/migrate api up

version="$(
  docker compose --env-file .env.example -f deploy/compose.yaml exec -T mysql sh -ec '
    MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysql \
      --host=127.0.0.1 \
      --user=root \
      --batch \
      --skip-column-names \
      "${MYSQL_DATABASE}" \
      --execute "SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1;"
  '
)"
# 当前最新版本应为 6；同时证明第二次 up 没有重复执行副作用。
test "$version" = 6

# 显式标记的集成测试连接真实 MySQL，验证索引、CHECK 和外键而不是仅检查 SQL 文本。
set -a
. ./.env.example
set +a
go -C src/backend test -tags=integration \
  ./internal/migrations ./internal/post ./internal/auth ./internal/media -count=1

# down 后再次 up，证明迁移具备最小可回滚性且能重新应用。
docker compose --env-file .env.example -f deploy/compose.yaml \
  run --rm --no-deps --entrypoint /app/migrate api down

# 一次 down 只恢复旧 MIME 白名单；检查约束内容，避免误回退分类等前序迁移。
mime_check="$(
  docker compose --env-file .env.example -f deploy/compose.yaml exec -T mysql sh -ec '
    MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysql \
      --host=127.0.0.1 \
      --user=root \
      --batch \
      --skip-column-names \
      "${MYSQL_DATABASE}" \
      --execute "SELECT CHECK_CLAUSE FROM information_schema.check_constraints
        WHERE constraint_schema = DATABASE()
        AND constraint_name = '"'"'chk_media_assets_mime_type'"'"';"
  '
)"
test -n "$mime_check"
test "${mime_check#*image/gif}" = "$mime_check"

# 前序分类和基础表仍须存在，一次 down 不能连带删除已有内容模型。
remaining_taxonomy_tables="$(
  docker compose --env-file .env.example -f deploy/compose.yaml exec -T mysql sh -ec '
    MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysql \
      --host=127.0.0.1 \
      --user=root \
      --batch \
      --skip-column-names \
      "${MYSQL_DATABASE}" \
      --execute "SELECT COUNT(*) FROM information_schema.tables
        WHERE table_schema = DATABASE()
        AND table_name IN ('"'"'categories'"'"', '"'"'tags'"'"', '"'"'post_tags'"'"');"
  '
)"
test "${remaining_taxonomy_tables}" = 3

base_tables="$(
  docker compose --env-file .env.example -f deploy/compose.yaml exec -T mysql sh -ec '
    MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysql \
      --host=127.0.0.1 \
      --user=root \
      --batch \
      --skip-column-names \
      "${MYSQL_DATABASE}" \
      --execute "SELECT COUNT(*) FROM information_schema.tables
        WHERE table_schema = DATABASE()
        AND table_name IN ('"'"'posts'"'"', '"'"'admins'"'"', '"'"'refresh_tokens'"'"', '"'"'media_assets'"'"');"
  '
)"
test "${base_tables}" = 4

version="$(
  docker compose --env-file .env.example -f deploy/compose.yaml exec -T mysql sh -ec '
    MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysql \
      --host=127.0.0.1 \
      --user=root \
      --batch \
      --skip-column-names \
      "${MYSQL_DATABASE}" \
      --execute "SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1;"
  '
)"
test "$version" = 5

docker compose --env-file .env.example -f deploy/compose.yaml \
  run --rm --no-deps --entrypoint /app/migrate api up

version="$(
  docker compose --env-file .env.example -f deploy/compose.yaml exec -T mysql sh -ec '
    MYSQL_PWD="${MYSQL_ROOT_PASSWORD}" exec mysql \
      --host=127.0.0.1 \
      --user=root \
      --batch \
      --skip-column-names \
      "${MYSQL_DATABASE}" \
      --execute "SELECT MAX(version_id) FROM goose_db_version WHERE is_applied = 1;"
  '
)"
test "$version" = 6

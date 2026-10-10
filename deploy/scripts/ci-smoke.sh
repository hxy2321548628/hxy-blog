#!/usr/bin/env bash
# 冒烟测试使用一次性数据卷；失败路径也必须清理。
set -euo pipefail
# 本地也可运行冒烟任务；独立项目名保护已有开发数据库卷。
export COMPOSE_PROJECT_NAME="hxy-blog-ci-smoke-$$"
cleanup() {
  local status=$?
  docker compose --env-file .env.example -f deploy/compose.yaml down --volumes || status=1
  exit "$status"
}
trap cleanup EXIT
# Sprint 1 后 API 依赖 MySQL 与迁移；复用 Compose 才能覆盖真实服务发现和启动顺序。
docker compose --env-file .env.example -f deploy/compose.yaml up -d mysql --wait
docker compose --env-file .env.example -f deploy/compose.yaml \
  run --rm --no-deps --entrypoint /app/migrate api up
docker compose --env-file .env.example -f deploy/compose.yaml \
  up -d api web --wait --wait-timeout 60

# 第一条请求验证 Nginx → Go API 代理，第二条验证 React SPA 未知路由回退。
curl --fail --silent http://127.0.0.1:8080/api/health | grep --fixed-strings '"status":"ok"'
curl --fail --silent http://127.0.0.1:8080/not-a-real-page | grep --fixed-strings '<div id="root"></div>'

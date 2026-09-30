#!/usr/bin/env bash

set -Eeuo pipefail
umask 077

readonly APP_DIR=/opt/hxy-blog
readonly COMPOSE_FILE="${APP_DIR}/compose.yaml"
readonly RUNTIME_ENV=/etc/hxy-blog/runtime.env
readonly RELEASE_ENV="${APP_DIR}/release.env"
readonly PREVIOUS_ENV="${APP_DIR}/previous.env"
readonly LOCK_FILE=/run/lock/hxy-blog-deploy.lock
readonly API_REPOSITORY=ghcr.io/hxy2321548628/hxy-blog-api
readonly WEB_REPOSITORY=ghcr.io/hxy2321548628/hxy-blog-web

fail() {
  printf '部署失败：%s\n' "$1" >&2
  exit 1
}

if [[ ${EUID} -ne 0 ]]; then
  fail '必须由 root 执行'
fi

if [[ $# -ne 3 ]]; then
  fail '参数必须是 <commit> <api-digest> <web-digest>'
fi

readonly revision=$1
readonly api_digest=$2
readonly web_digest=$3

[[ ${revision} =~ ^[0-9a-f]{40}$ ]] || fail 'Commit 格式无效'
[[ ${api_digest} =~ ^sha256:[0-9a-f]{64}$ ]] || fail 'API Digest 格式无效'
[[ ${web_digest} =~ ^sha256:[0-9a-f]{64}$ ]] || fail 'Web Digest 格式无效'

for required_file in "${COMPOSE_FILE}" "${RUNTIME_ENV}"; do
  [[ -f ${required_file} ]] || fail "缺少 ${required_file}"
  [[ $(stat -c '%u' "${required_file}") == 0 ]] || fail "${required_file} 必须由 root 拥有"
done

# 运行时环境文件包含数据库密码，拒绝组用户或其他用户可读的权限。
if [[ -n $(find "${RUNTIME_ENV}" -maxdepth 0 -perm /077 -print) ]]; then
  fail "${RUNTIME_ENV} 权限必须不高于 600"
fi

command -v docker >/dev/null || fail '未安装 Docker'
command -v curl >/dev/null || fail '未安装 curl'
command -v flock >/dev/null || fail '未安装 flock'

exec 9>"${LOCK_FILE}"
flock -n 9 || fail '已有部署正在执行'

candidate_env=$(mktemp "${APP_DIR}/.release.env.XXXXXX")
trap 'rm -f -- "${candidate_env}"' EXIT

printf '%s\n' \
  "API_IMAGE=${API_REPOSITORY}@${api_digest}" \
  "WEB_IMAGE=${WEB_REPOSITORY}@${web_digest}" \
  "APP_VERSION=sha-${revision}" \
  "APP_REVISION=${revision}" > "${candidate_env}"

compose_with_env() {
  local release_file=$1
  shift
  docker compose \
    --project-name hxy-blog-production \
    --env-file "${RUNTIME_ENV}" \
    --env-file "${release_file}" \
    --file "${COMPOSE_FILE}" \
    "$@"
}

rollback() {
  printf '新版本未通过健康检查，开始恢复上一应用版本。\n' >&2
  if [[ -f ${PREVIOUS_ENV} ]]; then
    install -o root -g root -m 600 "${PREVIOUS_ENV}" "${RELEASE_ENV}"
    compose_with_env "${RELEASE_ENV}" up -d --no-build --wait --wait-timeout 120
    printf '上一应用版本已恢复。\n' >&2
    return
  fi

  # 首次部署失败时没有上一版本，只删除应用栈；命名卷不会被删除。
  compose_with_env "${RELEASE_ENV}" down --remove-orphans
  rm -f -- "${RELEASE_ENV}"
  printf '首次部署已停止，MySQL 命名卷仍然保留。\n' >&2
}

# 拉取失败不会改动当前 release.env，也不会影响正在运行的版本。
compose_with_env "${candidate_env}" pull api web

if [[ -f ${RELEASE_ENV} ]]; then
  install -o root -g root -m 600 "${RELEASE_ENV}" "${PREVIOUS_ENV}"
else
  rm -f -- "${PREVIOUS_ENV}"
fi

install -o root -g root -m 600 "${candidate_env}" "${RELEASE_ENV}"

if ! compose_with_env "${RELEASE_ENV}" up -d --no-build --wait --wait-timeout 120; then
  rollback
  fail 'Compose 未能启动新版本'
fi

if ! curl --fail --silent --show-error http://127.0.0.1:8080/healthz >/dev/null; then
  rollback
  fail 'Web 健康检查失败'
fi

if ! curl --fail --silent --show-error http://127.0.0.1:8080/api/health >/dev/null; then
  rollback
  fail 'API 健康检查失败'
fi

printf '部署成功：commit=%s api=%s web=%s\n' "${revision}" "${api_digest}" "${web_digest}"

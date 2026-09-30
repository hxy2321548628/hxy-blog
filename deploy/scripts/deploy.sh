#!/usr/bin/env bash

# 生产部署主入口。GitHub Actions 只能把 Commit 和两个镜像 Digest 传到这里；
# 脚本按“校验 → 拉取 → 备份 → 迁移 → 切换 → 健康检查”的顺序执行。
# 任一步失败都停止，只有应用切换后的失败才触发镜像回滚，数据库不会自动 down。
set -Eeuo pipefail
# 新建文件默认仅 root 可读，防止 release.env 或临时文件被其他用户读取。
umask 077

# 路径和镜像仓库写死在 root 拥有的脚本中，受限 SSH 用户不能选择任意文件或镜像。
readonly APP_DIR=/opt/hxy-blog
readonly COMPOSE_FILE="${APP_DIR}/compose.yaml"
readonly RUNTIME_ENV=/etc/hxy-blog/runtime.env
readonly RELEASE_ENV="${APP_DIR}/release.env"
readonly PREVIOUS_ENV="${APP_DIR}/previous.env"
readonly LOCK_FILE=/run/lock/hxy-blog-deploy.lock
readonly BACKUP_SCRIPT=/usr/local/sbin/hxy-blog-db-backup
readonly API_REPOSITORY=ghcr.io/hxy2321548628/hxy-blog-api
readonly WEB_REPOSITORY=ghcr.io/hxy2321548628/hxy-blog-web

fail() {
  # 所有主动失败使用同一前缀写入 stderr，Actions 日志能快速定位部署阶段。
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

# 严格格式白名单也阻止参数被解释成额外 Shell 选项或命令片段。
[[ ${revision} =~ ^[0-9a-f]{40}$ ]] || fail 'Commit 格式无效'
[[ ${api_digest} =~ ^sha256:[0-9a-f]{64}$ ]] || fail 'API Digest 格式无效'
[[ ${web_digest} =~ ^sha256:[0-9a-f]{64}$ ]] || fail 'Web Digest 格式无效'

for required_file in "${COMPOSE_FILE}" "${RUNTIME_ENV}"; do
  # 部署只信任 root 拥有的编排与密钥文件，避免低权限用户替换执行内容。
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
[[ -x ${BACKUP_SCRIPT} ]] || fail "缺少 ${BACKUP_SCRIPT}"
[[ $(stat -c '%u' "${BACKUP_SCRIPT}") == 0 ]] || fail "${BACKUP_SCRIPT} 必须由 root 拥有"

exec 9>"${LOCK_FILE}"
# 非阻塞锁保证同一时刻只有一个部署，避免两个发布交叉覆盖 release.env。
flock -n 9 || fail '已有部署正在执行'

# candidate_env 在所有前置步骤通过前都不会替换当前 release.env。
candidate_env=$(mktemp "${APP_DIR}/.release.env.XXXXXX")
trap 'rm -f -- "${candidate_env}"' EXIT

printf '%s\n' \
  "API_IMAGE=${API_REPOSITORY}@${api_digest}" \
  "WEB_IMAGE=${WEB_REPOSITORY}@${web_digest}" \
  "APP_VERSION=sha-${revision}" \
  "APP_REVISION=${revision}" > "${candidate_env}"

compose_with_env() {
  # 第一份环境文件提供数据库密钥，第二份提供本次候选或已发布镜像；
  # 封装后可确保部署与回滚使用完全相同的 Compose 参数。
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
  # 回滚只恢复上一组 API/Web Digest。Goose down 可能丢数据，因此永不自动执行。
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

# 迁移前先确保 MySQL 健康并创建逻辑备份。任一步失败都不会切换应用镜像。
compose_with_env "${candidate_env}" up -d mysql --wait --wait-timeout 120
"${BACKUP_SCRIPT}" "${revision}"
compose_with_env "${candidate_env}" run --rm --no-deps --entrypoint /app/migrate api up

# 只有备份和迁移成功后才保存上一版本并提交新的 release.env。
if [[ -f ${RELEASE_ENV} ]]; then
  install -o root -g root -m 600 "${RELEASE_ENV}" "${PREVIOUS_ENV}"
else
  rm -f -- "${PREVIOUS_ENV}"
fi

install -o root -g root -m 600 "${candidate_env}" "${RELEASE_ENV}"

# --no-build 保证服务器不会现场构建；--wait 依据三个容器的 healthcheck 判定就绪。
if ! compose_with_env "${RELEASE_ENV}" up -d --no-build --wait --wait-timeout 120; then
  rollback
  fail 'Compose 未能启动新版本'
fi

# Compose 健康后再经过实际 Nginx 入口检查 Web 与 /api 代理，覆盖用户真实请求路径。
if ! curl --fail --silent --show-error http://127.0.0.1:8080/healthz >/dev/null; then
  rollback
  fail 'Web 健康检查失败'
fi

if ! curl --fail --silent --show-error http://127.0.0.1:8080/api/health >/dev/null; then
  rollback
  fail 'API 健康检查失败'
fi

printf '部署成功：commit=%s api=%s web=%s\n' "${revision}" "${api_digest}" "${web_digest}"

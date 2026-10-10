#!/usr/bin/env bash

# 只接受显式提供的提交与镜像地址，避免本地误把工作区发布为生产版本。
set -euo pipefail
: "${HEAD_SHA:?必须指定已通过 main CI 的完整提交 SHA}"
: "${API_IMAGE:?必须指定 API 镜像地址}"
: "${WEB_IMAGE:?必须指定 Web 镜像地址}"
[[ "$HEAD_SHA" =~ ^[0-9a-f]{40}$ ]] || { echo 'HEAD_SHA 必须是完整的 40 位提交 SHA' >&2; exit 1; }

immutable_tag="sha-${HEAD_SHA}"

# 生产主机为 amd64；构建元数据使镜像可追溯到精确提交。
docker build \
  --platform linux/amd64 \
  --build-arg VERSION="$immutable_tag" \
  --build-arg REVISION="$HEAD_SHA" \
  --tag "${API_IMAGE}:${immutable_tag}" \
  --tag "${API_IMAGE}:main" \
  src/backend
docker build \
  --platform linux/amd64 \
  --build-arg VERSION="$immutable_tag" \
  --build-arg REVISION="$HEAD_SHA" \
  --tag "${WEB_IMAGE}:${immutable_tag}" \
  --tag "${WEB_IMAGE}:main" \
  src/web

# 两个不可变版本推送成功后才更新便于查看的 main 指针。
docker push "${API_IMAGE}:${immutable_tag}"
docker push "${WEB_IMAGE}:${immutable_tag}"
docker push "${API_IMAGE}:main"
docker push "${WEB_IMAGE}:main"

api_digest="$(docker buildx imagetools inspect "${API_IMAGE}:${immutable_tag}" | awk '$1 == "Digest:" { print $2; exit }')"
web_digest="$(docker buildx imagetools inspect "${WEB_IMAGE}:${immutable_tag}" | awk '$1 == "Digest:" { print $2; exit }')"
[[ "$api_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo 'API Digest 无效' >&2; exit 1; }
[[ "$web_digest" =~ ^sha256:[0-9a-f]{64}$ ]] || { echo 'Web Digest 无效' >&2; exit 1; }

# GitHub 作业之间只传 Digest，部署步骤不会重新构建或解析可移动标签。
if [[ -n "${GITHUB_OUTPUT:-}" ]]; then
  printf 'api_digest=%s\nweb_digest=%s\n' "$api_digest" "$web_digest" >> "$GITHUB_OUTPUT"
fi

# 发布记录只含公开元数据；批准生产部署时可核对提交、运行链接与镜像身份。
if [[ -n "${GITHUB_STEP_SUMMARY:-}" ]]; then
  {
    printf '## GHCR 发布结果\n\n'
    printf 'Commit: `%s`\n\n' "$HEAD_SHA"
    printf 'CI: %s\n\nRelease: %s\n\n' "${CI_RUN_URL:-未提供}" "${RELEASE_RUN_URL:-未提供}"
    printf '| 镜像 | 版本 | Digest |\n| --- | --- | --- |\n'
    printf '| API | `%s` | `%s` |\n' "$immutable_tag" "$api_digest"
    printf '| Web | `%s` | `%s` |\n' "$immutable_tag" "$web_digest"
  } >> "$GITHUB_STEP_SUMMARY"
fi

printf 'Commit: %s\nAPI: %s@%s\nWeb: %s@%s\n' "$HEAD_SHA" "$API_IMAGE" "$api_digest" "$WEB_IMAGE" "$web_digest"

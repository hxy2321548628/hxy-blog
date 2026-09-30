# syntax=docker/dockerfile:1

# 第一阶段使用 Node 将 TypeScript/React 编译为静态文件；第二阶段只用 Nginx
# 提供静态资源和 /api 反向代理，生产环境不需要 Node.js 常驻。
# Node 只负责构建；最终镜像只保留 Nginx 和静态文件。
ARG NODE_IMAGE=node:24-alpine@sha256:ebfe2f90462722a7a4de65e91990e97fe0d401c70e0e762c5b53302f905ec1c1
ARG NGINX_IMAGE=nginx:stable-alpine@sha256:0985e772fb9f729e6fa0980da05fca5d9c468e870eed43071545afa9d2e27d94

FROM ${NODE_IMAGE} AS builder

WORKDIR /src

# 先复制 lockfile 并执行 npm ci，业务源码变化时仍可命中依赖缓存层。
COPY package.json package-lock.json ./
RUN npm ci

COPY . .
RUN npm run build

FROM ${NGINX_IMAGE} AS runtime

# 标签记录 Commit，但不会把运行时密钥打入前端静态文件。
ARG VERSION=dev
ARG REVISION=unknown

LABEL org.opencontainers.image.title="hxy-blog-web" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.source="https://github.com/hxy2321548628/hxy-blog"

# 使用完整配置以支持 SPA 回退、API 代理和非 root 运行。
COPY nginx.conf /etc/nginx/nginx.conf
COPY --from=builder /src/dist /usr/share/nginx/html

USER 101:101
EXPOSE 8080

# Web 健康检查只验证 Nginx 自身；API 健康由独立端点检查。
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["nginx"]
# 前台运行让 Nginx 成为容器主进程，从而正确接收停止信号。
CMD ["-g", "daemon off;"]

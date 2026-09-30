# syntax=docker/dockerfile:1

# 多阶段构建把“需要 Go 工具链的编译环境”和“实际运行环境”分开，最终镜像
# 不包含编译器、源码和模块缓存，既减小体积也减少攻击面。
# 固定构建镜像 Digest，避免同一标签在不同时间产生不同的发布物。
ARG GO_IMAGE=golang:1.25-alpine@sha256:1ae0735f00daffa3aaf1363a5184c0d2dc55c78e3db4ec70241cdac97bf84b59
ARG RUNTIME_IMAGE=alpine:3.22@sha256:5291449c3df73caf6ed85e649dec1b9e818b39a5d8c871e97afc13e9cd5e8fa8

FROM ${GO_IMAGE} AS builder

WORKDIR /src

# 先复制依赖清单，业务代码变化时可复用依赖缓存层。
COPY go.mod go.sum ./
RUN go mod download

COPY . .
# CGO_ENABLED=0 生成不依赖系统 C 库的二进制；trimpath 去除构建机路径，
# -s -w 删除调试符号以缩小镜像。迁移器与 API 来自同一 Commit。
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migrate

FROM ${RUNTIME_IMAGE} AS runtime

# VERSION/REVISION 只写入 OCI 元数据，便于从镜像追溯发布来源。
ARG VERSION=dev
ARG REVISION=unknown

LABEL org.opencontainers.image.title="hxy-blog-api" \
      org.opencontainers.image.version="${VERSION}" \
      org.opencontainers.image.revision="${REVISION}" \
      org.opencontainers.image.source="https://github.com/hxy2321548628/hxy-blog"

# Go 访问 COS 等 HTTPS 服务时需要系统根证书；证书随固定构建镜像一起版本化。
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /out/api /app/api
COPY --from=builder /out/migrate /app/migrate

# Compose 可以覆盖监听地址；镜像单独运行时默认监听容器内 8080。
ENV HTTP_ADDR=:8080

# 使用无特权数字用户运行，不依赖镜像中的用户名配置。
USER 10001:10001
EXPOSE 8080

# Docker 与 Compose 都用同一个健康端点判断是否可以接收流量。
HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
  CMD wget -q -O /dev/null http://127.0.0.1:8080/api/health || exit 1

ENTRYPOINT ["/app/api"]

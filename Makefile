# Makefile 是本项目的统一命令入口。开发机和 CI 都调用这些目标，避免两套脚本
# 随时间产生差异。命令行前的制表符是 Make 语法要求，不能替换为空格。
# 显式设置默认目标，避免未来调整目标顺序后直接运行 make 意外执行其他动作。
.DEFAULT_GOAL := help

# .PHONY 表示这些名称是“动作”而不是文件；即使目录中出现同名文件也会执行。
.PHONY: help setup fmt fmt-check vet test test-go test-web check check-web check-deploy container-config container-production-config container-build container-up container-down dev-backend dev-migrate dev-web dev-db

# 集中声明路径和 Compose 命令，后续目标只组合这些变量。
WEB_DIR := src/web
# Go 默认把构建缓存写到用户目录；放在仓库的忽略目录中便于沙箱和 CI 使用。
GOCACHE ?= $(CURDIR)/.cache/go-build
COMPOSE_FILE := deploy/compose.yaml
# config 检查使用示例变量，不需要开发者先创建包含真实密码的 .env。
COMPOSE_EXAMPLE := docker compose --env-file .env.example -f $(COMPOSE_FILE)
# 真正启动本地容器时必须读取不提交到 Git 的 .env。
COMPOSE_LOCAL := docker compose --env-file .env -f $(COMPOSE_FILE)
export GOCACHE

# 默认只展示说明，不读取 .env、不安装依赖，也不启动任何进程。
help:
	@printf '%s\n' \
		'开发命令：' \
		'  make help                         显示本帮助（默认目标）' \
		'  make setup                        按锁文件安装前端依赖' \
		'  make dev-db                       启动本地 MySQL' \
		'  make dev-migrate                  对本地 MySQL 执行 Goose up' \
		'  make dev-backend                  启动 Go API' \
		'  make dev-web                      启动 React 开发服务器' \
		'' \
		'代码质量：' \
		'  make fmt                          格式化 Go 代码' \
		'  make fmt-check                    检查 Go 格式但不修改文件' \
		'  make vet                          运行 Go 静态检查' \
		'  make test-go                      运行 Go 测试' \
		'  make test-web                     运行 React 组件测试' \
		'  make test                         运行全部测试' \
		'  make check-web                    检查并构建 React 前端' \
		'  make check-deploy                 检查部署脚本、文件名和 CAM JSON' \
		'  make check                        运行提交前完整质量门禁' \
		'' \
		'容器命令：' \
		'  make container-config             验证开发 Compose 配置' \
		'  make container-production-config  验证生产 Compose 配置' \
		'  make container-build              构建本地 API/Web 镜像' \
		'  make container-up                 构建并启动完整本地容器栈' \
		'  make container-down               停止容器但保留 MySQL 数据卷'

# 安装前端锁文件中精确记录的依赖；ci 比 install 更适合可重复构建。
setup:
	npm --prefix $(WEB_DIR) ci

# 直接修复 Go 格式，供开发者主动执行。
fmt:
	gofmt -w src/backend

# CI 只检查而不改文件；gofmt 输出非空就说明存在未格式化文件。
fmt-check:
	@test -z "$$(gofmt -l src/backend)" || (echo "Go files need formatting; run 'make fmt'" && exit 1)

# vet 检查编译器不一定报错的常见 Go 问题。
vet:
	go -C src/backend vet ./...

# -C 让 Go 在后端模块目录中解析 go.mod。
test-go:
	go -C src/backend test ./...

# 组件测试使用 Node 环境静态渲染，不需要启动浏览器或开发服务器。
test-web:
	npm --prefix $(WEB_DIR) test

# 统一 test 入口同时覆盖前后端，CI 与本地不会漏掉任一侧。
test: test-go test-web

# 前端 check 会依次执行 ESLint、TypeScript 类型检查和生产构建。
check-web:
	npm --prefix $(WEB_DIR) run check

# 提交前总门禁。Make 会先执行右侧每个依赖目标，任一失败都会停止。
check: fmt-check vet test check-web container-config container-production-config check-deploy

# 仅解析开发 Compose，捕获变量插值和 YAML/Compose 结构错误，不启动容器。
container-config:
	$(COMPOSE_EXAMPLE) config --quiet

# 生产编排与开发编排分开检查，防止只在部署时发现生产配置错误。
container-production-config:
	docker compose --env-file .env.example -f deploy/compose.production.yaml config --quiet

# Shell 的 -n 只做语法解析；后两条命令验证新旧两种备份文件名；
# json.tool 确认腾讯云 CAM 策略仍是合法 JSON。
check-deploy:
	bash -n deploy/scripts/deploy.sh deploy/scripts/ssh-entry.sh deploy/scripts/backup-db.sh deploy/scripts/restore-drill.sh deploy/scripts/install-coscli.sh deploy/scripts/configure-coscli.sh deploy/scripts/sync-backup-cos.sh deploy/scripts/fetch-backup-cos.sh deploy/scripts/backup-current-db.sh
	bash -c '[[ "20260930T032152Z-sha-8aa51a66049e.sql.gz" =~ ^[0-9]{8}T[0-9]{6}([0-9]{9})?Z-sha-[0-9a-f]{12}\.sql\.gz$$ ]]'
	bash -c '[[ "20260930T032152643485203Z-sha-8aa51a66049e.sql.gz" =~ ^[0-9]{8}T[0-9]{6}([0-9]{9})?Z-sha-[0-9a-f]{12}\.sql\.gz$$ ]]'
	python3 -m json.tool deploy/cam/cos-backup-policy.json >/dev/null

# 构建与生产一致的 API/Web 本地镜像，CI 的容器冒烟测试依赖这些标签。
container-build:
	$(COMPOSE_EXAMPLE) build

# 开发栈先等待 MySQL，再执行编译进当前 API 镜像的迁移，
# 避免新数据卷出现“容器健康但业务表不存在”的假就绪状态。
container-up:
	$(COMPOSE_LOCAL) build api web
	$(COMPOSE_LOCAL) up -d mysql --wait
	$(COMPOSE_LOCAL) run --rm --no-deps --entrypoint /app/migrate api up
	$(COMPOSE_LOCAL) up -d api web

# down 删除容器和网络，但不带 --volumes，因此不会删除本地 MySQL 数据。
container-down:
	$(COMPOSE_LOCAL) down

# 以下目标用于不启动完整栈时的单组件开发。
# 宿主机开发也共用 .env，不要求开发者再手工导出一遍数据库变量。
dev-backend:
	bash -c 'set -a; source .env; set +a; exec go -C src/backend run ./cmd/api'

dev-migrate:
	bash -c 'set -a; source .env; set +a; exec go -C src/backend run ./cmd/migrate up'

dev-web:
	npm --prefix $(WEB_DIR) run dev

dev-db:
	$(COMPOSE_LOCAL) up -d mysql

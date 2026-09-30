.PHONY: setup fmt fmt-check vet test test-go check check-web check-deploy container-config container-production-config container-build container-up container-down dev-backend dev-web dev-db

WEB_DIR := src/web
GOCACHE ?= $(CURDIR)/.cache/go-build
COMPOSE_FILE := deploy/compose.yaml
COMPOSE_EXAMPLE := docker compose --env-file .env.example -f $(COMPOSE_FILE)
COMPOSE_LOCAL := docker compose --env-file .env -f $(COMPOSE_FILE)
export GOCACHE

setup:
	npm --prefix $(WEB_DIR) ci

fmt:
	gofmt -w src/backend

fmt-check:
	@test -z "$$(gofmt -l src/backend)" || (echo "Go files need formatting; run 'make fmt'" && exit 1)

vet:
	go -C src/backend vet ./...

test-go:
	go -C src/backend test ./...

test: test-go

check-web:
	npm --prefix $(WEB_DIR) run check

check: fmt-check vet test check-web container-config container-production-config check-deploy

container-config:
	$(COMPOSE_EXAMPLE) config --quiet

container-production-config:
	docker compose --env-file .env.example -f deploy/compose.production.yaml config --quiet

check-deploy:
	bash -n deploy/scripts/deploy.sh deploy/scripts/ssh-entry.sh deploy/scripts/backup-db.sh deploy/scripts/restore-drill.sh deploy/scripts/install-coscli.sh deploy/scripts/configure-coscli.sh deploy/scripts/sync-backup-cos.sh deploy/scripts/fetch-backup-cos.sh deploy/scripts/backup-current-db.sh
	bash -c '[[ "20260930T032152Z-sha-8aa51a66049e.sql.gz" =~ ^[0-9]{8}T[0-9]{6}([0-9]{9})?Z-sha-[0-9a-f]{12}\.sql\.gz$$ ]]'
	bash -c '[[ "20260930T032152643485203Z-sha-8aa51a66049e.sql.gz" =~ ^[0-9]{8}T[0-9]{6}([0-9]{9})?Z-sha-[0-9a-f]{12}\.sql\.gz$$ ]]'
	python3 -m json.tool deploy/cam/cos-backup-policy.json >/dev/null

container-build:
	$(COMPOSE_EXAMPLE) build

container-up:
	$(COMPOSE_LOCAL) up -d --build

container-down:
	$(COMPOSE_LOCAL) down

dev-backend:
	go -C src/backend run ./cmd/api

dev-web:
	npm --prefix $(WEB_DIR) run dev

dev-db:
	$(COMPOSE_LOCAL) up -d mysql

.PHONY: setup fmt fmt-check vet test test-go check check-web container-config container-build container-up container-down dev-backend dev-web dev-db

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

check: fmt-check vet test check-web container-config

container-config:
	$(COMPOSE_EXAMPLE) config --quiet

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

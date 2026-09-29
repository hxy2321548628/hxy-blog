.PHONY: setup fmt fmt-check vet test test-go check check-web dev-backend dev-web dev-db

WEB_DIR := src/web
GOCACHE ?= $(CURDIR)/.cache/go-build
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

check: fmt-check vet test check-web

dev-backend:
	go -C src/backend run ./cmd/api

dev-web:
	npm --prefix $(WEB_DIR) run dev

dev-db:
	@echo "MySQL image is not configured yet; set the existing image first."
	@exit 1

.PHONY: up down generate tidy run test-e2e test-browser test-all \
        migrate-up migrate-down migrate-status migrate-create \
        web-install web \
        backup

# `go -C backend run ...` runs the binary with CWD = backend/, so we point
# APOTECH_CONFIG at the repo-root config from there. Using `export` (a Make
# directive, not a shell command) so this works under any shell make picks
# on the host — POSIX sh, bash, or Windows cmd.exe.
export APOTECH_CONFIG := ../config.yaml

GO_BACKEND := go -C backend

# --- Docker ------------------------------------------------------------------
up:
	docker compose up -d

down:
	docker compose down

# --- Proto codegen -----------------------------------------------------------
generate:
	buf generate

# --- Backend (Go) ------------------------------------------------------------
tidy:
	go -C backend mod tidy

run:
	$(GO_BACKEND) run ./cmd/server

# End-to-end / integration tests (in-process httptest server + real dev Postgres).
# Test binaries run with CWD = backend/e2e/, so the APOTECH_CONFIG path needs
# two `..` to reach the repo-root config.yaml.
# `-count=1` disables Go's test-result caching.
test-e2e: export APOTECH_CONFIG := ../../config.yaml
test-e2e:
	$(GO_BACKEND) test ./e2e/... -v -count=1

migrate-up:
	$(GO_BACKEND) run ./cmd/migrate up

migrate-down:
	$(GO_BACKEND) run ./cmd/migrate down

migrate-status:
	$(GO_BACKEND) run ./cmd/migrate status

# Usage: make migrate-create name=add_medicines_table
migrate-create:
	$(GO_BACKEND) run ./cmd/migrate create $(name) sql

# --- Frontend (React + Vite) -------------------------------------------------
web-install:
	npm --prefix frontend install

web:
	npm --prefix frontend run dev

# Browser E2E tests (Playwright). Assumes `make run` and `make web` are
# already running in separate terminals; tests hit http://localhost:5173 and
# share the dev DB. Suite runs against Chromium headless by default.
# (cd into frontend so playwright.config.ts is loaded relative to CWD.)
test-browser:
	cd frontend && npx playwright test

# Convenience: run both Go integration tests and browser E2E tests.
test-all: test-e2e test-browser

# --- Backups -----------------------------------------------------------------
# Snapshot the running Postgres into backups/YYYY-mm-dd_HHMMSS.sql.gz.
# Uses pg_dump from the docker-compose db container so the user doesn't need
# pg_dump installed locally. Wire to cron for nightly backups in production.
backup:
	@mkdir -p backups
	docker compose exec -T db pg_dump -U apotech apotech | gzip > backups/$$(date +%Y-%m-%d_%H%M%S).sql.gz
	@echo "Wrote backups/$$(ls -1t backups | head -n 1)"

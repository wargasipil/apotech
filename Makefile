.PHONY: up down reset-devel-data generate tidy run test-e2e test-browser test-all \
        migrate-up migrate-down migrate-status migrate-create \
        web-install web \
        embed-web build dist-windows docker-build docker-up docker-down installer \
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

# Wipe the dev DB and start fresh. Works in cmd.exe, PowerShell, and bash —
# no shell idioms. `down -v` removes the named volume (the cluster); `up -d
# --wait` blocks on the compose-defined healthcheck until Postgres accepts
# connections. The next `make run` auto-applies migrations and creates the
# bootstrap owner.
reset-devel-data:
	docker compose down -v
	docker compose up -d --wait

# --- Proto codegen -----------------------------------------------------------
generate:
	buf generate

# --- Backend (Go) ------------------------------------------------------------
tidy:
	go -C backend mod tidy

run:
	$(GO_BACKEND) run ./cmd/server

# --- Packaging (single self-contained binary) --------------------------------
# embed-web builds the SPA and copies it into the Go embed dir. `build` and
# `dist-windows` then compile a single binary with the UI + migrations embedded.
embed-web:
	npm --prefix frontend ci
	npm --prefix frontend run build
	rm -rf backend/internal/web/dist/assets
	cp -r frontend/dist/. backend/internal/web/dist/

# Native single binary -> dist/apotech (serves UI + /api + auto-migrates).
build: embed-web
	@mkdir -p dist
	$(GO_BACKEND) build -ldflags "-s -w" -o ../dist/apotech ./cmd/server

# Windows single binary -> dist/apotech.exe (input to the installer build).
# Pure-Go deps mean no CGO, so this cross-compiles from any host.
dist-windows: embed-web
	@mkdir -p dist
	GOOS=windows GOARCH=amd64 $(GO_BACKEND) build -ldflags "-s -w" -o ../dist/apotech.exe ./cmd/server

# --- Docker (production image + compose) --------------------------------------
docker-build:
	docker build -t apotech:latest .

docker-up:
	docker compose -f docker-compose.prod.yml up -d --build

docker-down:
	docker compose -f docker-compose.prod.yml down

# --- Windows installer -------------------------------------------------------
# Assembles the payload (exe + bundled Postgres + WinSW) and runs Inno Setup.
# Requires PowerShell + Inno Setup (ISCC) on PATH; see packaging/windows/.
installer: dist-windows
	powershell -ExecutionPolicy Bypass -File packaging/windows/build-windows.ps1

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
# Snapshot the running Postgres into backups/backup_<timestamp>/.
# Produces the same layout as BackupService (database.sql.gz + manifest.txt)
# so CLI and in-app backups are interchangeable. Uses pg_dump from the
# docker-compose db container so no host pg_dump is required.
# Wire to cron for nightly backups in production.
backup:
	@stamp=$$(date +%Y-%m-%d_%H%M%S); \
	dir=backups/backup_$$stamp; \
	mkdir -p $$dir; \
	docker compose exec -T db pg_dump -U apotech apotech | gzip > $$dir/database.sql.gz; \
	size=$$(wc -c < $$dir/database.sql.gz | tr -d ' '); \
	ver=$$(docker compose exec -T db psql -U apotech -d apotech -tA -c "SELECT COALESCE(MAX(version_id),0) FROM goose_db_version WHERE is_applied" | tr -d '\r '); \
	dbver=$$(docker compose exec -T db psql -U apotech -d apotech -tA -c "SELECT version()" | tr -d '\r'); \
	{ \
	  echo "created_at=$$(date +%s)"; \
	  echo "created_at_iso=$$(date -u +%Y-%m-%dT%H:%M:%SZ)"; \
	  echo "app_version=dev"; \
	  echo "db_version=$$dbver"; \
	  echo "schema_version=$$ver"; \
	  echo "size_bytes=$$size"; \
	} > $$dir/manifest.txt; \
	echo "Wrote $$dir/"

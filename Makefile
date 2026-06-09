.PHONY: up down reset-devel-data generate tidy run test-e2e test-e2e-sqlite test-browser test-all \
        migrate-up migrate-down migrate-status migrate-create \
        web-install web \
        embed-web build build-portable dist-windows dist-windows-portable \
        docker-build docker-up docker-down installer installer-portable zip-portable \
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
# Shell-portable filesystem helpers (work the same under cmd.exe, PowerShell,
# and POSIX sh). Node is already a build prerequisite, so we lean on it
# instead of branching on $(OS). Double-quoted JS with single-quoted string
# literals survives both shells without further escaping.
RM_RF   = node -e "require('fs').rmSync(process.argv[1],{recursive:true,force:true})"
CP_R    = node -e "require('fs').cpSync(process.argv[1],process.argv[2],{recursive:true,force:true})"
MKDIR_P = node -e "require('fs').mkdirSync(process.argv[1],{recursive:true})"

# `dist-windows` then compile a single binary with the UI + migrations embedded.
# Uses `npm install` instead of `npm ci` for Windows-friendliness: `ci` first
# deletes node_modules, which races with editor/IDE language servers holding
# file handles (rollup .node bindings in particular) and EPERMs. `install`
# keeps existing files and resolves only what the lockfile demands. Same
# resulting tree, more tolerant of background processes.
embed-web:
	npm --prefix frontend install
	npm --prefix frontend run build
	$(RM_RF) backend/internal/web/dist/assets
	$(CP_R) frontend/dist backend/internal/web/dist

# Native single binary -> dist/apotech (serves UI + /api + auto-migrates).
build: embed-web
	@$(MKDIR_P) dist
	$(GO_BACKEND) build -ldflags "-s -w" -o ../dist/apotech ./cmd/server

# Windows single binary -> dist/apotech.exe (input to the installer build).
# Pure-Go deps mean no CGO, so this cross-compiles from any host.
# Env-vars are set via target-specific assignment + `export` (above), so Make
# pushes them into the shell's environment instead of relying on inline
# `VAR=val cmd` syntax (POSIX-only — fails under cmd.exe).
dist-windows: export GOOS = windows
dist-windows: export GOARCH = amd64
dist-windows: export CGO_ENABLED = 0
dist-windows: embed-web
	@$(MKDIR_P) dist
	$(GO_BACKEND) build -ldflags "-s -w" -o ../dist/apotech.exe ./cmd/server

# --- Portable (SQLite) flavor ------------------------------------------------
# Same source tree, `-tags sqlite` swaps the DB driver + migrations + backup
# strategy. modernc.org/sqlite is pure-Go so cross-compilation needs no CGO.

# Native single binary -> dist/apotech-portable (SQLite, single-file DB).
build-portable: embed-web
	@$(MKDIR_P) dist
	$(GO_BACKEND) build -tags sqlite -ldflags "-s -w" -o ../dist/apotech-portable ./cmd/server

# Windows single binary -> dist/apotech-portable.exe (input to the portable installer).
dist-windows-portable: export GOOS = windows
dist-windows-portable: export GOARCH = amd64
dist-windows-portable: export CGO_ENABLED = 0
dist-windows-portable: embed-web
	@$(MKDIR_P) dist
	$(GO_BACKEND) build -tags sqlite -ldflags "-s -w" -o ../dist/apotech-portable.exe ./cmd/server

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
# -SkipExeBuild because dist-windows already produced the EXE.
installer: dist-windows
	powershell -ExecutionPolicy Bypass -File packaging/windows/build-windows.ps1 -SkipExeBuild

# Portable installer — no bundled Postgres, no services, no firewall rule.
# Ships apotech-portable.exe + a SQLite-driven config.yaml + a backup .bat.
installer-portable: dist-windows-portable
	powershell -ExecutionPolicy Bypass -File packaging/windows/build-windows-portable.ps1 -SkipExeBuild

# Portable ZIP — drop-anywhere, USB-friendly. Same EXE as the Inno installer.
# Ships apotech-portable.exe + first-run.bat + bilingual README.txt.
zip-portable: dist-windows-portable
	powershell -ExecutionPolicy Bypass -File packaging/windows/build-zip-portable.ps1 -SkipExeBuild

# End-to-end / integration tests (in-process httptest server + real dev Postgres).
# Test binaries run with CWD = backend/e2e/, so the APOTECH_CONFIG path needs
# two `..` to reach the repo-root config.yaml.
# `-count=1` disables Go's test-result caching.
test-e2e: export APOTECH_CONFIG := ../../config.yaml
test-e2e:
	$(GO_BACKEND) test ./e2e/... -v -count=1

# SQLite test variant: same suites, per-test temp DB (helpers.go injects under
# the sqlite tag). No config.yaml needed — the helper sets Driver/Path itself.
test-e2e-sqlite:
	$(GO_BACKEND) test -tags sqlite ./e2e/... -v -count=1

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

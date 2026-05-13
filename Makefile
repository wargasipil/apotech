.PHONY: up down generate tidy run \
        migrate-up migrate-down migrate-status migrate-create \
        web-install web

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

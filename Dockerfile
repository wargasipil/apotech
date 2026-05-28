# Apotech — single self-contained image.
# Multi-stage: build the SPA, embed it + migrations into one Go binary, ship a
# tiny final image. The binary serves the UI + /api on one port and runs goose
# migrations on boot.

# --- Stage 1: build the React SPA --------------------------------------------
FROM node:20-alpine AS web
WORKDIR /app/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# --- Stage 2: build the Go binary (SPA + migrations embedded) -----------------
FROM golang:1.25 AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./backend/
RUN cd backend && go mod download
COPY backend/ ./backend/
# Drop the real SPA build into the embed dir (overlays the committed stub).
COPY --from=web /app/frontend/dist/ ./backend/internal/web/dist/
RUN cd backend && CGO_ENABLED=0 GOOS=linux go build -ldflags "-s -w" -o /out/apotech ./cmd/server

# --- Stage 3: minimal runtime ------------------------------------------------
# Switched from distroless/static to debian:bookworm-slim so we can install
# postgresql-client — BackupService subprocesses `pg_dump` to write
# /var/lib/apotech/backups/backup_<ts>/database.sql.gz. Cost: ~80 MB heavier
# image, no longer "distroless"; in exchange the in-app Create-backup feature
# works server-side without a separate sidecar.
FROM debian:bookworm-slim
RUN apt-get update \
 && apt-get install --no-install-recommends -y postgresql-client ca-certificates tzdata \
 && rm -rf /var/lib/apt/lists/*
# Run as a non-root user (mirrors the distroless `nonroot` posture).
RUN groupadd --system --gid 65532 apotech \
 && useradd  --system --uid 65532 --gid 65532 --home /app apotech \
 && mkdir -p /app /var/lib/apotech/backups \
 && chown -R apotech:apotech /app /var/lib/apotech
WORKDIR /app
COPY --from=build /out/apotech /app/apotech
COPY config.docker.yaml /app/config.yaml
ENV APOTECH_CONFIG=/app/config.yaml
# Default shop timezone for the "today" boundary; override in compose if needed.
ENV TZ=Asia/Jakarta
EXPOSE 8080
USER apotech:apotech
ENTRYPOINT ["/app/apotech"]

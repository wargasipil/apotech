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
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/apotech /app/apotech
COPY config.docker.yaml /app/config.yaml
ENV APOTECH_CONFIG=/app/config.yaml
# Default shop timezone for the "today" boundary; override in compose if needed.
ENV TZ=Asia/Jakarta
EXPOSE 8080
USER nonroot:nonroot
ENTRYPOINT ["/app/apotech"]

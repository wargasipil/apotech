# Apotech — Pharmacy Store Management

Monorepo skeleton for a pharmacy (apotek) store management app.

## Stack
- **Backend**: Go + GORM + PostgreSQL
- **API**: Buf + ConnectRPC (protobuf-driven)
- **Migrations**: goose
- **Frontend**: React + TypeScript + Chakra UI v3 + Vite

## Quickstart
All commands run from the repo root.

```sh
cp config.example.yaml config.yaml       # first time only (gitignored)

make up                                  # start Postgres (docker compose)
make generate                            # buf -> backend/gen + frontend/src/gen
make tidy                                # go mod tidy
make migrate-up                          # apply goose migrations (no-op until you add some)
make run                                 # API server on http://localhost:8080

# in another terminal:
make web-install                         # npm install
make web                                 # Vite on http://localhost:5173
```

Open the browser, click **Ping** — you should see `{ status: "ok", db: "ok" }` (proves proto → Go → Connect → React round-trip).

## Layout
See [CLAUDE.md](CLAUDE.md) for the full project map, conventions, and update policy.

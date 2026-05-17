# Apotech — Deployment runbook

Single-shop local deployment. The app is designed to run on one box (or VM)
behind the apotek's LAN, talking to a thermal printer on the same network.

## Prerequisites
- Linux host (any modern distro) or Windows with WSL2.
- Docker + Docker Compose v2.
- Go 1.25+ (only if you'll rebuild the binary on the box; CI-built binaries
  are fine too).
- Node 20+ (only for rebuilding the frontend; ship `frontend/dist/` from CI).
- Network thermal printer reachable on the LAN (Epson TM-T, Star, or generic
  ESC/POS over TCP 9100). Optional — Phase 7-printer is disabled by default.

## Files
| File | Purpose |
|---|---|
| `config.yaml` | runtime config (gitignored). Copy from `config.example.yaml` and fill in. |
| `docker-compose.yml` | Postgres 18+ container. |
| `backend/cmd/server` | Go server binary (`make run` or build with `go -C backend build`). |
| `frontend/dist/` | static SPA bundle (`npm run build` produces it). Serve via the reverse proxy. |

## First-time setup
1. `cp config.example.yaml config.yaml` and fill in:
   - `auth.jwt_secret` — long random string (`openssl rand -hex 32`).
   - `bootstrap.owner_email` / `owner_password` — the initial OWNER. **Change after first login** via the password-reset flow.
   - `printer.address` if you have a thermal printer; otherwise leave `enabled: false`.
2. `make up` to start Postgres.
3. `make migrate-up` to apply all migrations through #17.
4. `make run` to start the server on `:8080`.
5. Serve `frontend/dist/` and reverse-proxy `/api/*` to `localhost:8080`. Example nginx snippet:
   ```nginx
   server {
     listen 80;
     server_name apotek.local;
     root /var/www/apotech;
     location /api/ { proxy_pass http://127.0.0.1:8080/; }
     location / { try_files $uri /index.html; }
   }
   ```
6. Browse to the host, log in as the bootstrap owner, immediately create proper user accounts and disable the bootstrap account.

## Daily operations
| Action | Command / location |
|---|---|
| Tail server logs (structured JSON via `log/slog`) | `journalctl -fu apotech` or wherever you run `make run` |
| Apply a new migration | `make migrate-up` |
| Backup (one-off) | `make backup` → `backups/YYYY-mm-dd_HHMMSS.sql.gz` |
| Backup (nightly) | cron the same command, e.g. `0 2 * * * cd /opt/apotech && make backup` |
| Rotate the JWT secret | edit `config.yaml`, restart server — all sessions invalidated |
| Reset a forgotten password | OWNER → Users page → "Issue reset token" → hand the displayed token OOB to the user → they redeem at `/reset?token=...` |

## Backups
- `make backup` runs `pg_dump` inside the Postgres container and writes a
  gzipped SQL file to `backups/`.
- Keep at least 7 nightlies on disk; ship the latest off-host weekly (rsync to
  external drive or to S3 with `aws s3 sync backups/ s3://apotech-backups/`).
- Test restore: spin up a second compose stack pointed at a fresh DB, run
  `gunzip -c backups/<file>.sql.gz | docker compose exec -T db psql -U apotech apotech`.

## Observability
- The server emits structured JSON logs via `log/slog` on stdout.
- Every write RPC is recorded in the `audit_log` table (see
  `internal/auth/audit.go`). Query via SQL to investigate "who changed what":
  ```sql
  SELECT created_at, user_id, role, procedure, ok, code, message
  FROM audit_log
  WHERE created_at > now() - interval '24 hours'
  ORDER BY created_at DESC;
  ```
- No external metrics endpoint today. Add a `/metrics` Prometheus endpoint in
  a follow-up if you wire Prometheus/Grafana.

## Security checklist
- Change `auth.jwt_secret` from the example value.
- Change `bootstrap.owner_password` from the example value, then disable the
  bootstrap account after creating real OWNER users.
- Terminate TLS at the reverse proxy (Caddy, nginx, or Traefik). The Go
  server runs plaintext on `localhost:8080`.
- Restrict Postgres to localhost (already true in `docker-compose.yml`).
- Login rate limit: 5 attempts then ~1/minute refill per email (see
  `internal/auth/ratelimit.go`). In-process — single-node only.
- Refresh tokens rotate on every Refresh; replaying a revoked token kills the
  whole family.

## Updates / upgrades
1. `git pull`
2. `make generate` (regenerate proto code if proto changed)
3. `make tidy`
4. `make migrate-up`
5. Rebuild binary + restart: `go -C backend build -o /usr/local/bin/apotech ./cmd/server && systemctl restart apotech`
6. Rebuild frontend if needed: `npm --prefix frontend run build && rsync frontend/dist/ /var/www/apotech/`

## Multi-branch deployment
- Multi-branch is supported at the data layer (Phase 8). The `branches` table
  is seeded with a single `MAIN` row at migration time; create more via the
  `/branches` admin page (OWNER only).
- Users access branches via the `user_branches` join. Grant access via the
  same admin page or directly: `INSERT INTO user_branches (user_id, branch_id, is_default) VALUES (...)`.
- The frontend sends the current `X-Branch-Id` header on every RPC; switching
  branches in the top-bar selector reloads the page.
- Per-list branch filtering is opt-in per RPC and not yet retrofitted across
  every read query — see the "Known gaps" section in `CLAUDE.md`.

## Known limitations
- ESC/POS dispatch assumes the backend and printer share a LAN (raw TCP to
  port 9100). For hosted backends, swap to a local print-bridge daemon.
- BPJS Kesehatan integration is a local-only stub; the actual DJP/BPJS API
  client is pending merchant credentials + Apotek-Vendor certification.
- No SMTP integration for password reset; tokens are minted by OWNER and
  handed to the user out-of-band.
- No HA / load balancing: single-process, in-memory rate limiter; the audit
  log is async-write (loses queued rows if the process crashes mid-write).

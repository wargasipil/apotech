# Apotech — Deployment runbook

Single-shop deployment. Apotech ships as **one self-contained binary** that
serves the web UI and the API on a single port and auto-applies its database
migrations on boot. Three turnkey distribution flavors:

- **Docker image** — Linux box / VM / cloud host, backed by Postgres.
  `docker compose up`.
- **Windows installer (full)** — pharmacy running everything on a Windows PC,
  optionally serving a few LAN registers, backed by Postgres + WinSW services.
  Double-click `ApotechSetup-*.exe`.
- **Windows installer (portable / SQLite)** — single-PC pharmacies who don't
  want a database server, services, or admin rights. Backed by a single
  SQLite file next to the EXE. Double-click `ApotechPortableSetup-*.exe`.

All three embed the SPA + migrations into the binary; there is **no separate
nginx, static host, or manual migrate step**. The portable flavor additionally
bundles no external DB engine — modernc.org/sqlite is linked into the EXE.

**Which flavor?**
- Multi-PC shop with cashiers + back-office, or you want LAN/cloud reach →
  Docker (Linux/cloud) or full Windows installer (Postgres-backed).
- Single PC, single user, USB-stick or per-user install, no admin rights →
  Portable Windows installer (SQLite-backed).

---

## Flavor 1 — Docker

### Prerequisites
- Docker + Docker Compose v2.

### Deploy
```sh
cp .env.example .env          # set APOTECH_JWT_SECRET (openssl rand -hex 32) + owner creds
docker compose -f docker-compose.prod.yml up -d --build
```
- The `app` image (built from the repo `Dockerfile`) serves UI + `/api` on `:8080`.
- `postgres` (pinned `postgres:18`) starts first; the app waits for it healthy,
  auto-migrates, then listens. Data persists in the `apotech-pgdata` named volume.
- Secrets come from `.env` (env overrides in `internal/config`): `APOTECH_JWT_SECRET`,
  `APOTECH_DB_PASSWORD`, `APOTECH_OWNER_EMAIL`, `APOTECH_OWNER_PASSWORD`, `APOTECH_TZ`.

Browse `http://<host>:8080`, log in as the bootstrap owner, create real OWNER
users, then change/disable the bootstrap account.

### Update
```sh
git pull
docker compose -f docker-compose.prod.yml up -d --build   # rebuilds image, re-migrates on boot
```

### Make shortcuts
`make docker-build`, `make docker-up`, `make docker-down`.

---

## Flavor 2 — Windows installer

A self-contained `.exe` that installs the app, a **bundled PostgreSQL**, both as
auto-start Windows Services, plus a browser shortcut. Zero prerequisites for the
pharmacist. Full build + install + verification details: [packaging/windows/README.md](packaging/windows/README.md).

### Build (developer machine, Windows x64)
Needs Go 1.25+, Node 20+, and Inno Setup 6.
```powershell
make installer          # = dist-windows + packaging\windows\build-windows.ps1
# -> dist\ApotechSetup-<version>.exe
```

### Install (target PC)
Run the `.exe`. The wizard asks for the owner email/password, **network access**
(single-PC `127.0.0.1` vs LAN `0.0.0.0` + firewall rule), and ports. Post-install
initializes the DB, writes `C:\ProgramData\Apotech\config.yaml` (random
`jwt_secret` + DB password), registers + starts `apotech-postgres` and
`apotech-server`, and opens the browser.

### Topology
Editable later in `C:\ProgramData\Apotech\config.yaml` (`server.host`); run
`Restart-Service apotech-server` to apply. LAN mode adds a firewall rule for the
app port; PostgreSQL always stays bound to `127.0.0.1` (clients hit the app).

---

## Flavor 3 — Windows installer (portable, SQLite)

A second Windows `.exe` for **single-PC, single-user** deployments: no admin,
no services, no firewall, no database server. The whole runtime is one binary
+ one SQLite file. Same UI, same RPCs, same migrations semantically. Full
details: [packaging/windows/README-portable.md](packaging/windows/README-portable.md).

### Build (developer machine)
Needs Go 1.25+, Node 20+, and Inno Setup 6. Cross-compiles from Linux/macOS
hosts too (modernc.org/sqlite is pure-Go, no CGO).
```powershell
make installer-portable   # = dist-windows-portable + build-windows-portable.ps1
# -> dist\ApotechPortableSetup-<version>.exe
```

### Install (target PC)
Run the `.exe`. The wizard asks for owner email/password + the app port
(default `8080`). No admin needed — installs per-user under `%LOCALAPPDATA%`
by default. Post-install writes a fresh `config.yaml` (random `jwt_secret`,
`driver: sqlite`, `path: <install>\data\apotech.db`) and launches the EXE,
which auto-opens the browser to `http://127.0.0.1:8080`.

### Daily ops
- **Launch**: Start Menu → **Apotech Portable**. Or tick "Run on Windows
  startup" during install (adds an HKCU Run key — not a service).
- **Backup**: built-in `Settings → Backups` page, or run
  `apotech-portable-backup.bat` from Task Scheduler. Each backup is a
  per-timestamp folder containing `database.db` (SQLite `VACUUM INTO` snapshot)
  + `manifest.txt`.
- **Restore**: close the EXE → copy `backups\backup_<TS>\database.db` over
  `data\apotech.db` → relaunch.
- **No NSFP cross-warehouse concurrency**: SQLite serializes writes at the DB
  level, so the e-Faktur allocator works without `FOR UPDATE SKIP LOCKED`.
  Single-shop pharmacies don't notice; multi-cashier scale needs the full
  Postgres flavor.

### ZIP variant (drop-anywhere, USB-friendly)
The same `apotech-portable.exe` ships in a second shape — a plain ZIP, no
installer, no Start Menu entry, no registry footprint. Use when the owner
wants the whole pharmacy in a folder they can put on a USB stick or sync via
Dropbox.

```powershell
make zip-portable   # = dist-windows-portable + build-zip-portable.ps1
# -> dist\ApotechPortable-<version>.zip  (~8 MB)
```

The ZIP unpacks into `ApotechPortable\` with exactly three files:
- `apotech-portable.exe`
- `first-run.bat` — interactive one-time setup. Prompts for owner email +
  password, generates a 256-bit JWT secret, writes `config.yaml`, creates
  `data\` and `backups\`. Refuses to overwrite an existing `config.yaml`.
- `README.txt` — bilingual EN + ID tutorial (quick start, daily ops, backup,
  restore, FAQ, troubleshooting).

**On the target PC:** unzip anywhere → double-click `first-run.bat` once →
enter credentials → double-click `apotech-portable.exe` → browser opens at
`http://127.0.0.1:8080`. Data lives in `data\apotech.db` next to the EXE;
backups in `backups\backup_<ts>\database.db`. The whole folder is self-
contained — `xcopy` it (or zip + transfer) to another PC and run the EXE
there with no further setup.

---

## Local dev (unchanged)
`make up` (Postgres) · `make run` (server on `:8080`) · `make web` (Vite on `:5173`,
proxies `/api` → `:8080`). Dev uses the Vite server, not the embedded SPA.

`make build` produces the native self-contained binary at `dist/apotech` for a
local production smoke test.

---

## Daily operations
| Action | Docker | Windows (full) | Windows (portable) |
|---|---|---|---|
| Logs | `docker logs -f apotech-app` (JSON via `slog`) | `C:\ProgramData\Apotech\logs\` (WinSW) + Event Viewer | stdout — pipe to a log file or run from a terminal |
| Migrations | automatic on boot | automatic on boot | automatic on boot |
| Backup (one-off) | OWNER → **Settings → Backups → Create**, OR `make backup`, OR `docker compose exec ... pg_dump` | OWNER → **Settings → Backups → Create**, OR `scripts\apotech-backup.bat` | OWNER → **Settings → Backups → Create**, OR `apotech-portable-backup.bat` (= `apotech-portable.exe --backup`) |
| Backup (nightly) | cron `make backup` | Task Scheduler → `apotech-backup.bat` | Task Scheduler → `apotech-portable-backup.bat` |
| Rotate JWT secret | edit `.env`, `up -d` | edit `config.yaml`, `Restart-Service apotech-server` | edit `config.yaml`, close + relaunch the EXE |
| Reset a password | OWNER → Users → "Issue reset token" → hand token OOB → user redeems at `/reset?token=...` | same | same |

## Backups
- **Layout (one folder per backup)** — every backup is its own per-timestamp directory under `backup.directory` (Docker: `/var/lib/apotech/backups` mounted as the `apotech-backups` named volume; Windows full: `C:\ProgramData\Apotech\backups\`; Windows portable: `<install dir>\backups\`; dev: `./backups`):
  ```
  backup_2026-05-26_152400/
    database.sql.gz   (Postgres flavors: pg_dump compressed)
    database.db       (SQLite portable: VACUUM INTO snapshot — open in DB Browser for SQLite)
    manifest.txt      (created_at, schema_version, size_bytes, app/db version)
  ```
  Override the root with the `APOTECH_BACKUP_DIR` env var or the `backup.directory` config key.
- **Four entry points produce identical directory layouts** (the dump filename differs per flavor) so the OWNER's in-app list and the cron job's output are the same folders:
  - **In-app** (OWNER → Settings → Backups → Create) — `BackupService.CreateBackup`; same screen lists past backups + a Delete with confirm. Refreshes every 60s.
  - **Docker / dev CLI** — `make backup` (uses the compose Postgres' bundled `pg_dump`).
  - **Windows CLI (full)** — `C:\Program Files\Apotech\scripts\apotech-backup.bat` (uses the bundled `pg_dump.exe`).
  - **Windows CLI (portable)** — `<install dir>\apotech-portable-backup.bat` → `apotech-portable.exe --backup` (no external binary, single SQLite `VACUUM INTO` snapshot).
- **Restore is manual.** A maintenance-mode UX is deferred, so there's no in-app restore button. Choose by flavor:
  - **Docker (compressed)**: stop the app, restore, restart.
    ```sh
    docker compose -f docker-compose.prod.yml stop app
    gunzip < /var/lib/docker/volumes/apotech_apotech-backups/_data/backup_<TS>/database.sql.gz \
      | docker compose -f docker-compose.prod.yml exec -T postgres psql -U apotech -d apotech
    docker compose -f docker-compose.prod.yml start app
    ```
  - **Windows (full, uncompressed)**: stop the service, restore, restart.
    ```powershell
    Stop-Service apotech-server
    & "$env:ProgramFiles\Apotech\pgsql\bin\psql.exe" -h 127.0.0.1 -p <port> -U apotech -d apotech `
      -f "$env:ProgramData\Apotech\backups\backup_<TS>\database.sql"
    Start-Service apotech-server
    ```
  - **Windows (portable, SQLite)**: close the EXE, copy the snapshot file in place, relaunch.
    ```powershell
    # Close apotech-portable.exe first (no service to stop — it's user-launched).
    Copy-Item -Force "<install>\backups\backup_<TS>\database.db" "<install>\data\apotech.db"
    & "<install>\apotech-portable.exe"
    ```
  The Postgres dumps are `pg_dump --clean --if-exists`, so they drop existing tables before reloading — the live DB is safe to restore over, but make sure no other clients are writing while it runs. The SQLite snapshot is a complete consistent file, so the "copy in place" pattern is enough.
- **Operational discipline**: keep at least 7 nightlies; ship the latest off-host weekly (rsync / S3 / external drive). Test restore against a fresh DB before you rely on a backup.
- **Why the Docker image is no longer distroless**: `BackupService` subprocesses `pg_dump`, which doesn't ship in `distroless/static`. The runtime base switched to `debian:bookworm-slim` + `postgresql-client` (~80 MB heavier). The container still runs as a non-root UID (65532).
- **pg_dump auto-resolution**: every in-app `Create backup` looks for pg_dump in this order — system PATH → bundled next to the apotech binary (`<install>/pgsql/bin/pg_dump.exe`, the Windows installer layout — works even though the installer doesn't put that dir on PATH) → cache at `backup.pg_tools_dir` (default `%LOCALAPPDATA%\apotech\pgtools` on Windows / `~/.cache/apotech/pgtools` on Linux) → on **Windows only**, auto-download the EDB binaries zip (~75 MB) into the cache and reuse it. Linux without `postgresql-client` (and outside Docker) gets a clear "install postgresql-client" error instead of a download — apt is the right answer there. Override the cache root with `APOTECH_PG_TOOLS_DIR`.
- **First-time Create cost on Windows dev**: the EDB zip download is **~75 MB** and has been observed taking **30+ min on slow foreign network links** before the remote occasionally drops the connection. The resolver supports **HTTP Range resume** (partial `.tmp` files survive failures) and retries up to 3 times in one Create call, so a click eventually succeeds even on flaky links. Subsequent backups use the cached binary and complete in under a second.
- **Manual escape hatch** (recommended when the auto-download is too slow): drop a `pg_dump.exe` (any recent version from a PostgreSQL Windows install) at `%LOCALAPPDATA%\apotech\pgtools\pgsql\bin\pg_dump.exe`. The resolver's step 3 (cache lookup) finds it and skips the download entirely.

## Observability
- Structured JSON logs via `log/slog`.
- Every write RPC is recorded in the `audit_log` table (`internal/auth/audit.go`):
  ```sql
  SELECT created_at, user_id, role, procedure, ok, code, message
  FROM audit_log WHERE created_at > now() - interval '24 hours' ORDER BY created_at DESC;
  ```
- No external metrics endpoint today; add a Prometheus `/metrics` in a follow-up.

## Security checklist
- Set a strong `auth.jwt_secret` (Docker: `APOTECH_JWT_SECRET`; Windows: generated automatically).
- Change the bootstrap owner password after first login; disable the bootstrap account.
- Put TLS in front for any non-localhost exposure (the server speaks plaintext HTTP).
- PostgreSQL is reachable only from the app (no published host port in prod compose; localhost-only on Windows).
- Login rate limit: 5 attempts then ~1/min refill per email (in-process, single-node).
- Refresh tokens rotate on every Refresh; replaying a revoked token revokes the whole family.

## Multi-branch
- Supported at the data layer (Phase 8); `branches` is seeded with `MAIN`. Create more via `/branches` (OWNER).
- Frontend sends `X-Branch-Id` per RPC; switching branches reloads. Per-list branch filtering is opt-in per RPC (see CLAUDE.md "Known gaps").

## Known limitations
- ESC/POS dispatch assumes backend + printer share a LAN (raw TCP :9100).
- BPJS / e-Faktur are local stubs pending real credentials.
- No SMTP password-reset (OWNER-issued token, handed OOB).
- No HA: single-process, in-memory rate limiter; async audit-write loses queued rows on crash.
- Windows + Docker images are **unsigned**; expect SmartScreen on the installer until code-signed.

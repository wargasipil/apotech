# CLAUDE.md — Apotech Project Memory

Living doc for Claude. Update this file whenever the stack, layout, or conventions change (new service, new dep, new top-level dir).

## Project overview
- **What**: Management app for an apotek (pharmacy) store.
- **Current phase**: Phases 7–9 shipped. The app is feature-complete vs the original roadmap: Indonesia integrations (ESC/POS printer ✓, e-Faktur tax invoice ✓, BPJS claim tracking ✓ stub), multi-branch ✓, production hardening ✓ (Login rate limit, audit log, owner-issued password reset, slog structured logs, `make backup`, `DEPLOYMENT.md`).
- **Packaging**: shipped three turnkey distribution flavors — a **Docker image** + **server Windows installer** (both Postgres-backed, full multi-PC shop deployment), and a **portable Windows installer** (`packaging/windows/apotech-portable.iss`, SQLite-backed, single-PC, no admin/services/firewall). All three share the same source tree; the two Windows flavors produce **two separate binaries** via Go build tags (`apotech.exe` = default, `apotech-portable.exe` = `-tags sqlite`). See "Packaging & distribution" + "SQLite portable flavor" below, and `DEPLOYMENT.md`.
- **Next phase**: open to user direction — the roadmap is closed. Live-API integrations (real BPJS HTTP client, real DJP e-Faktur web service) are the most obvious unfinished items; both require external credentials.

## Roadmap

| # | Phase | Status |
|---|---|---|
| 0 | Skeleton (Go + ConnectRPC + Postgres + React/Chakra + Buf + goose + Makefile) | ✓ shipped |
| 1 | Auth + user/role management (JWT, proto-declared role guards) | ✓ shipped |
| 2 | Inventory foundation (suppliers, medicines + price-version table, batches, stock ledger) | ✓ shipped |
| — | Frontend foundation refactor (TanStack Query + RHF + Zod + i18n + Chakra `defaultSystem`) | ✓ shipped |
| — | Refresh tokens (rotation + reuse detection; access 1h, refresh 30d) | ✓ shipped |
| 3 | POS / Sales (customers, sales, FEFO consumption, split-view cashier UI, on-screen receipt) | ✓ shipped |
| 4 | Analytics (sales / inventory / margin via Recharts; live Postgres queries) | ✓ shipped |
| 5 | Purchasing (purchase orders, supplier ledger, formal receive flow) | ✓ shipped |
| 6 | Prescriptions (Rx validation tied to a Sale, customer-linked) | ✓ shipped |
| 7 | Indonesia integrations (ESC/POS thermal ✓, e-Faktur ✓, BPJS Kesehatan ✓ stub) | ✓ shipped |
| 8 | Multi-branch (promote `branch_id` from placeholder to first-class) | ✓ shipped |
| 9 | Production hardening (rate limit, audit log, password reset, slog, backups, runbook) | ✓ shipped |

**Recommended sequence**: 3 → 7-printer → 4 → 5 → 6 → 7-eFaktur → 7-BPJS → 8 → 9.

Each phase's full scope, schema, RPC list, and verification plan lives in the per-user plan file (`~/.claude/plans/…`) when it becomes the active task; only this high-level table lives here. **Update this table at the start of every phase (move 🚧 pointer) and at the end (flip status to ✓ shipped).**

## Stack
| Area | Choice |
|------|--------|
| Backend language | Go (1.25+) |
| ORM | GORM (`gorm.io/gorm`, `gorm.io/driver/postgres`) |
| DB | PostgreSQL (`postgres:latest` via docker-compose) |
| API | ConnectRPC (`connectrpc.com/connect`) over HTTP/1.1 + h2c |
| Codegen | Buf (`buf.yaml` v2, `buf.gen.yaml` v2) |
| Migrations | goose (`github.com/pressly/goose/v3`) |
| Config | YAML (`gopkg.in/yaml.v3`), file path overridable via `APOTECH_CONFIG` |
| Frontend | React 18 + TypeScript + Chakra UI v3 (`defaultSystem`) + Vite |
| Frontend RPC client | `@connectrpc/connect`, `@connectrpc/connect-web`, `@bufbuild/protobuf` |
| Frontend routing | `react-router-dom` v6 |
| Frontend server state | TanStack Query v5 (`@tanstack/react-query`) |
| Frontend client state | Zustand v4 (cross-cutting only) + React Context (auth) |
| Frontend forms | React Hook Form + Zod (resolvers via `@hookform/resolvers/zod`) |
| Frontend i18n | `react-i18next` (id, en) — no hard-coded user strings |
| Frontend icons | Lucide (`lucide-react`) |
| Frontend toaster | Chakra v3 `createToaster` (global instance + `AppToaster` mount) |
| Charts | Recharts v2 (`recharts`) |
| Auth | JWT HS256 (`github.com/golang-jwt/jwt/v5`); bcrypt password hashing (`golang.org/x/crypto/bcrypt`) |

## Repo layout
```
apotech/
├── Makefile                # unified entrypoint (docker, buf, go, npm targets)
├── proto/
│   ├── auth_iface/v1/      # Role enum + MethodOptions extensions (no services)
│   ├── health_iface/v1/    # HealthService
│   ├── user_iface/v1/      # User + UserService + AuthService (login/refresh/logout/me)
│   ├── inventory_iface/v1/ # Supplier, Medicine (+ MedicinePrice), Batch, StockMovement
│   ├── customer_iface/v1/  # Customer + CustomerService
│   ├── pos_iface/v1/       # Sale, SaleItem, PaymentSource + SaleService (FEFO consumption)
│   ├── analytics_iface/v1/ # SalesAnalytics + InventoryAnalytics + MarginAnalytics
│   ├── purchasing_iface/v1/ # PurchaseOrder + PurchaseReceipt + PurchasePayment services
│   ├── prescription_iface/v1/ # Prescription + PrescriptionItem + PrescriptionService
│   ├── tax_iface/v1/       # NSFP pool + TaxInvoice (e-Faktur bookkeeping; no DJP API)
│   ├── bpjs_iface/v1/      # BpjsClaim (stub: local claim tracking, no live BPJS API)
│   ├── branch_iface/v1/    # Branch + UserBranchMembership + BranchService (DEPRECATED — superseded by warehouse_iface)
│   ├── stocktake_iface/v1/ # StocktakeSession + StocktakeLine + StocktakeService
│   ├── settings_iface/v1/  # App-wide Settings (low_stock_threshold) + SettingsService
│   ├── backup_iface/v1/    # BackupService (OWNER-only Create / List / Delete; per-timestamp dirs)
│   └── warehouse_iface/v1/ # Warehouse + UserWarehouseMembership + WarehouseService + StockTransferService
├── buf.yaml                # buf v2 module config (root)
├── buf.gen.yaml            # buf v2 codegen config (root)
├── backend/                # Go service
│   ├── cmd/server/         # main entrypoint (serves UI + /api, auto-migrates on boot)
│   ├── cmd/migrate/        # goose wrapper (uses embedded migrations; `create` writes to disk)
│   ├── internal/
│   │   ├── auth/           # JWT issuer, password hash, ctx Principal, interceptor
│   │   ├── config/         # YAML loader (Server{Host,Port}, Database{…,AutoMigrate}, Auth, Bootstrap, Printer, Backup) + env overrides
│   │   ├── db/             # GORM open + ping
│   │   ├── dbmigrate/      # runs embedded goose migrations on boot
│   │   ├── model/          # GORM models (User, …)
│   │   ├── printer/        # ESC/POS receipt rendering + raw-TCP dispatch
│   │   ├── web/            # embeds frontend dist (//go:embed) + SPA handler; dist/.gitkeep stub
│   │   └── service/        # ConnectRPC service implementations (Auth, Users, Health)
│   ├── gen/                # GENERATED — do not hand-edit
│   ├── migrations/         # goose .sql files + embed.go (//go:embed *.sql)
│   └── e2e/                # integration tests (in-process httptest + real dev DB)
├── frontend/               # React app
│   └── src/
│       ├── gen/            # GENERATED — do not hand-edit
│       ├── lib/
│       │   ├── transport.ts  # Connect transport + Bearer interceptor
│       │   ├── auth.tsx      # AuthProvider + useAuth hook
│       │   ├── clients.ts    # createPromiseClient for each service
│       │   ├── queryClient.ts # TanStack Query client + global error -> toast
│       │   ├── toaster.tsx   # AppToaster + toast.success/error/fromError
│       │   ├── i18n.ts       # react-i18next init (id + en)
│       │   ├── format.ts     # locale-aware money/date helpers
│       │   └── pagination.ts # DEFAULT_PAGE_SIZE/ALL_LIMIT + usePageState helper
│       ├── stores/
│       │   └── preferences.ts # theme + locale + sidebar (Zustand, persisted)
│       ├── queries/        # one file per domain — TanStack hooks wrapping Connect clients
│       │   ├── users.ts
│       │   ├── suppliers.ts
│       │   ├── medicines.ts
│       │   ├── batches.ts
│       │   ├── stock.ts
│       │   ├── refs.ts      # resolve-by-IDs name hooks (useMedicineRefs/useSupplierRefs/useCustomerRefs/useBatchRefs)
│       │   └── backup.ts    # useBackupsQuery / useCreateBackupMutation / useDeleteBackupMutation
│       ├── locales/{en,id}.json
│       ├── components/
│       │   ├── AppShell.tsx          # sidebar + topbar wrapper
│       │   ├── Sidebar.tsx           # 3-state responsive sidebar
│       │   ├── TopBar.tsx            # lang/theme/user menu chrome
│       │   ├── PageHeader.tsx        # title + breadcrumbs + actions
│       │   ├── EntityDrawer.tsx      # slide-over for create/edit
│       │   ├── EntityDialog.tsx      # centered-modal counterpart (same API; used by Obat form)
│       │   ├── FormField.tsx         # RHF Controller + Chakra Field (money prop -> MoneyInput)
│       │   ├── MoneyInput.tsx        # thousands-grouped integer money input (empty-at-0)
│       │   ├── SearchableSelect.tsx  # Combobox wrapper (long lists, type-to-filter)
│       │   ├── EnumSelect.tsx        # Select wrapper (short fixed enums)
│       │   ├── WarehouseSelect.tsx   # reusable warehouse picker: button -> searchable modal dialog
│       │   ├── ExportButton.tsx      # shared "Export CSV" button (async fetch-all -> downloadCsv)
│       │   ├── RouteTabs.tsx         # Chakra Tabs + react-router NavLink triggers (URL-driven tabs)
│       │   ├── Pagination.tsx        # shared list pager (Showing X–Y of N + Prev/Next + page size)
│       │   ├── BackButton.tsx        # reusable detail-page back nav (HARD RULE)
│       │   ├── ExpiryBadge.tsx       # shared days-to-expiry badge (Batches + MedicineDetail)
│       │   ├── ErrorBoundary.tsx
│       │   └── ProtectedRoute.tsx
│       ├── routes/{Login,Dashboard,Users,Inventory,Orders}.tsx   # Orders = sales/order history
│       ├── routes/inventory/{Medicines,MedicineDetail,Suppliers,Batches,Movements,Stocktake,StocktakeDetail,Transfers}.tsx
│       ├── routes/inventory/medicineDrawers.tsx  # shared Create/Edit medicine form drawers
│       ├── routes/Warehouses.tsx     # gudang admin (OWNER); replaces Branches.tsx (dormant)
│       ├── routes/purchasing/{Purchasing,PurchaseOrdersList,SuppliersLedger,NewPurchaseOrder,PurchaseOrderDetail}.tsx
│       ├── routes/Prescriptions.tsx
│       ├── App.tsx           # picks AppShell vs bare layout (login)
│       └── main.tsx          # ErrorBoundary > QueryClient > Chakra > Auth > Router + AppToaster
│   ├── playwright.config.ts  # Playwright config (headless Chromium, single worker)
│   └── tests/e2e/            # Browser E2E specs (auth, analytics, popups)
├── Dockerfile              # multi-stage: build SPA -> embed -> single binary -> debian:bookworm-slim + postgresql-client
├── .dockerignore
├── docker-compose.yml      # DEV: postgres:latest only (bind-mounted data/)
├── docker-compose.prod.yml # PROD: app image + pinned postgres:18 (named volume, healthcheck)
├── config.example.yaml     # template; copy to config.yaml (gitignored)
├── config.docker.yaml      # baked into the image; secrets via env overrides
├── config.yaml             # local runtime config (gitignored, lives at root)
├── .env.example            # secrets for docker-compose.prod.yml (copy to .env, gitignored)
├── packaging/windows/      # Windows installer: apotech.iss (Inno Setup), setup/uninstall.ps1,
│                           #   WinSW service template, build-windows.ps1, backup .bat, README
├── dist/                   # gitignored; build outputs (apotech, apotech.exe, ApotechSetup-*.exe)
├── backups/                # gitignored; populated by `make backup`
├── DEPLOYMENT.md           # deployment runbook (Docker + Windows flavors)
└── README.md
```

## Code generation
- `buf generate` (run at repo root) produces:
  - Go: `backend/gen/<pkg>/v1/*.pb.go` and `<pkg>v1connect/*.connect.go`
  - TS: `frontend/src/gen/<pkg>/v1/*_{pb,connect}.ts`
- Generated code is **committed** to the repo (simpler for review). Do not edit by hand.
- **Proto package naming pattern (HARD RULE)**: every proto package is named `<domain>_iface.v1`. No exceptions. Examples: `health_iface.v1`, `auth_iface.v1`, `user_iface.v1`. **Do NOT create packages prefixed with `apotech.*`** — `apotech` is the project name and never appears in a proto package path. New domains get a new folder under `proto/` (e.g. `proto/inventory_iface/v1/`).
- `auth_iface.v1` is a types-only package (Role enum + MethodOptions extensions). Services that need the Role import this package.
- **TS plugins are pinned to v1** in `buf.gen.yaml` (`bufbuild/es:v1.10.0`, `connectrpc/es:v1.6.1`). Reason: the v2 plugins for Connect-ES are not published as remote buf plugins yet; mixing v1+v2 plugins causes type mismatches. Frontend deps (`@bufbuild/protobuf`, `@connectrpc/connect*`) must stay on `^1.x` until the v2 remote plugin lands.

## Database
- **goose owns the schema.** Migration files live in `backend/migrations/` and are **embedded** into the binary via `backend/migrations/embed.go` (`//go:embed *.sql`).
- **GORM is query-only.** Do NOT use `AutoMigrate` — it causes drift with goose.
- DSN is built from `config.yaml` (`database.{host,port,user,password,name,sslmode}`).
- **Auto-migrate on boot**: the server runs the embedded migrations on startup via `internal/dbmigrate` unless `database.auto_migrate: false`. This is what makes the packaged binary turnkey (no separate migrate step in Docker / Windows). `internal/db` must NOT call `AutoMigrate`; that's GORM's, not goose's.
- Make targets `migrate-up` / `migrate-down` still drive goose explicitly; `cmd/migrate` reads the embedded FS for all commands except `create` (which writes a new `.sql` to disk).

## Config
- Default file: `./config.yaml` (relative to the binary's CWD). Repo-canonical location: `config.yaml` at the repo root (copy from `config.example.yaml`).
- Override path: `APOTECH_CONFIG=/path/to/config.yaml`.
- `config.yaml` is gitignored; `config.example.yaml` is committed.
- `go -C backend run ...` puts the binary's CWD at `backend/`, NOT the repo root. The Makefile sets `APOTECH_CONFIG=../config.yaml` for every backend recipe so the binary finds the root-level config from there. If you run the binary directly, set `APOTECH_CONFIG` yourself or run it from a directory that has a `config.yaml` next to it.

## Common commands
All commands run from the repo root (the Makefile lives there).
```sh
make up              # docker compose up -d (Postgres)
make down            # docker compose down
make reset-devel-data # wipe + reinit dev Postgres (use when fixtures get polluted)
make generate        # buf generate -> backend/gen + frontend/src/gen
make tidy            # go -C backend mod tidy
make migrate-up      # apply goose migrations
make migrate-down    # rollback one migration
make migrate-status  # show migration state
make migrate-create name=add_medicines_table   # new migration file
make run             # API server on :8080
make test-e2e        # run Go integration tests against the dev DB
make web-install     # npm install (frontend)
make web             # Vite dev server on :5173
make build           # single self-contained binary -> dist/apotech (SPA + migrations embedded)
make dist-windows    # cross-compile dist/apotech.exe (input to the Windows installer)
make docker-build    # docker build -t apotech:latest .
make docker-up       # docker compose -f docker-compose.prod.yml up -d --build
make docker-down     # tear down the prod stack
make installer       # build dist-windows + assemble the Windows installer (needs Inno Setup)
```

### Development workflow (HARD RULE)
**Clean up dev processes after verifying.** If you (the assistant) start `make run` (Go backend, binds `:8080`) or `make web` (Vite dev server, binds `:5173`) — typically with `run_in_background: true` to drive a Playwright verification — you MUST kill that process before ending the turn / reporting the task complete. Find the PID via `netstat -ano | grep ":8080.*LISTENING"` (or `:5173`) and call `taskkill //F //PID <pid>` (Windows shell) / `kill <pid>` (POSIX). Dev servers the **user** started in their own terminal are NOT in scope — only processes the assistant launched itself. Leftover processes block the user's own dev workflow; the user has had to ask "stop it" / "clean up" multiple times. Killing on exit is cheap and the user can always restart.

## Conventions
- **New RPC method**: add it to a `.proto` file under `proto/<pkg>/v1/`, run `make generate`, implement the handler in `backend/internal/service/`, register it in `backend/cmd/server/main.go`.
- **List RPC pagination (convention)**: every `List*` RPC is server-paginated. The request carries `int32 limit` + `int32 offset`; the response carries `int32 total` (the unfiltered-by-page count for the same filters). Handlers use `normPage(limit, offset)` ([service/pagination.go](backend/internal/service/pagination.go)) — default page size **25**, cap **1000** (large limits are **clamped** to 1000, not reset to the default — so `useAll*` / `ALL_LIMIT` preloads receive the full list up to 1000), `offset` floored at 0 — then a two-chain GORM pattern: an `applyFilters(q)` closure feeds both a `Count(&total)` and the paginated `Offset/Limit/Find`, so filters can't contaminate the count. Denormalized per-page data (names, stock aggregates) is batch-loaded after the page loads (an `enrich*` method), never N+1. `ListBatches` computes per-warehouse stock in SQL (`GROUP BY` + `HAVING`) so `only_in_stock` + paging + `total` stay consistent; `ListPrescriptions` paginates the base (customer-scoped) rows and applies the computed-status filter client-side over the page (`total` reflects the base rows — documented v1 limitation).
- **New domain**: create a new `proto/<domain>_iface/v1/` folder and proto files; generated Go lands in `backend/gen/<domain>_iface/v1/` and TS in `frontend/src/gen/<domain>_iface/v1/`.
- **Frontend dev API calls**: hit `/api/...`. The backend serves Connect handlers **under `/api`** (it strips the prefix internally via `http.StripPrefix`), and the embedded SPA is served from `/`. In dev, Vite proxies `/api` → `:8080` **without rewriting** (matching prod). In production the single binary serves both UI and `/api` on one origin — no proxy, no CORS.
- **Chakra UI v3**: use **`defaultSystem`** (no custom system, no custom palette, no custom semantic tokens). Compose with other components via `asChild` (e.g. `<ChakraLink asChild><RouterLink to="/">...</RouterLink></ChakraLink>`). Brand accent is expressed via `colorPalette="blue"` on interactive components; surface and text tokens (`bg`, `bg.subtle`, `bg.muted`, `fg`, `fg.muted`, `border`, `border.muted`) come straight from `defaultSystem`.
- **Module path**: `github.com/apotech/backend` (placeholder — rename after pushing to a real Git host).
- **Migrations**: embedded via `backend/migrations/embed.go`. `cmd/migrate` calls `goose.SetBaseFS(migrations.FS)` for all commands except `create`, which still writes a new `.sql` to the on-disk `backend/migrations/` (run via `make migrate-create`, CWD = `backend/`). The server auto-applies them on boot (`internal/dbmigrate`).
- **Postgres volume mount**: dev `docker-compose.yml` uses a **named volume** (`apotech_devel_data` mounted at `/var/lib/postgresql`, NOT `/var/lib/postgresql/data`). The named volume enables `make reset-devel-data` to wipe + recreate the dev cluster portably (`docker compose down -v` → `up -d --wait`) from cmd.exe, PowerShell, or bash. The unusual mount target (`/var/lib/postgresql` rather than `/data`) is still required for `postgres:18+`, which stores data in a version-suffixed subdirectory and refuses to start if you mount directly at `/data`. The compose's `healthcheck` (`pg_isready`) is what `--wait` polls. Prod `docker-compose.prod.yml` uses a separate named volume.

## Packaging & distribution
The app ships as **one self-contained binary** that serves the SPA + `/api` on a single port and auto-migrates on boot. Both flavors build from this binary. Full ops in [DEPLOYMENT.md](DEPLOYMENT.md); Windows specifics in [packaging/windows/README.md](packaging/windows/README.md).
- **Single binary anatomy**: `internal/web` embeds `frontend/dist` (`//go:embed all:dist`) and serves it with SPA fallback; Connect handlers mount under `/api`; migrations embed via `backend/migrations/embed.go` and run on boot; `server.host` (config) controls the bind interface (`0.0.0.0` default; `127.0.0.1` single-PC). A `/healthz` route answers liveness probes outside `/api`. `time/tzdata` is imported so `TZ` works on minimal base images (the "today" boundary uses `time.Local`).
- **Embed compile dependency (gotcha)**: `//go:embed all:dist` needs at least one file present, so `backend/internal/web/dist/.gitkeep` is committed and the rest is gitignored. A plain `go build` on a fresh checkout works but serves a "frontend not built" stub; `make build` copies the real `frontend/dist` in first. Never delete the stub.
- **Config for packaging**: secrets can come from env (`APOTECH_JWT_SECRET`, `APOTECH_DB_PASSWORD`, `APOTECH_DB_HOST`, `APOTECH_OWNER_EMAIL`, `APOTECH_OWNER_PASSWORD`, `APOTECH_BACKUP_DIR`) overriding the YAML — so the Docker image bakes only non-secret defaults (`config.docker.yaml`).
- **Docker**: `Dockerfile` (node build → go build with embed → **`debian:bookworm-slim` + `postgresql-client`** — switched from distroless so `BackupService` can subprocess `pg_dump`; ~80 MB heavier, runs as a non-root `apotech` UID/GID) + `docker-compose.prod.yml` (app + pinned `postgres:18`, named volumes for pgdata + `/var/lib/apotech/backups`, healthcheck, `.env` secrets). Dev `docker-compose.yml` (Postgres-only) is unchanged.
- **Windows**: `packaging/windows/` — Inno Setup (`apotech.iss`) bundles `apotech.exe` + PostgreSQL Windows binaries + WinSW; `setup.ps1` inits the DB, writes `config.yaml`, registers `apotech-postgres` (native `pg_ctl register`, NetworkService) + `apotech-server` (WinSW) services, optional firewall rule, shortcut. Built via `build-windows.ps1` (`make installer`). **Scripts must stay ASCII** — Windows PowerShell 5.1 reads `-File` as Windows-1252, so non-ASCII (em dashes, curly quotes) corrupts parsing.

## SQLite portable flavor (single-PC Windows)
A second Windows binary, **`apotech-portable.exe`**, ships the same UI + RPCs over a **single SQLite file** — no Postgres, no service, no admin, no firewall. Same source tree; the SQLite driver + parallel migrations + VACUUM-INTO backup are selected via Go **build tags** (`-tags sqlite`), so each binary only links the code it actually uses. ~23 MB.

- **Build target = two separate binaries**: `make build` / `make dist-windows` (default, Postgres) and `make build-portable` / `make dist-windows-portable` (`-tags sqlite`). modernc.org/sqlite is pure-Go, so cross-compile from any host needs no CGO.
- **Config switch**: `database.driver: sqlite` + `database.path: ./data/apotech.db` (env overrides `APOTECH_DB_DRIVER`, `APOTECH_DB_PATH`). The Postgres build refuses `driver: sqlite` and vice-versa — config mismatch is a clear boot-time error.
- **Driver branch points** (build-tag-split files):
  - DB open: [backend/internal/db/db_postgres.go](backend/internal/db/db_postgres.go) (gorm pg dialect) vs [db_sqlite.go](backend/internal/db/db_sqlite.go) (gorm sqlite dialect + DSN pragmas `journal_mode=WAL&foreign_keys=ON&busy_timeout=30000`).
  - Migrations embed: [backend/migrations/embed_postgres.go](backend/migrations/embed_postgres.go) (`//go:embed *.sql` in `migrations/`) vs [embed_sqlite.go](backend/migrations/embed_sqlite.go) (`//go:embed sqlite/*.sql` rerooted via `fs.Sub` so goose sees a flat layout). Both expose `migrations.FS` + `migrations.Dialect`.
  - Backup: [backend/internal/service/backup_postgres.go](backend/internal/service/backup_postgres.go) (shells `pg_dump` → `database.sql.gz`) vs [backup_sqlite.go](backend/internal/service/backup_sqlite.go) (`VACUUM INTO database.db`). Per-timestamp directory + manifest stay identical so listing + delete + UI work unchanged.
  - Analytics daily bucket: [backend/internal/service/analytics_postgres.go](backend/internal/service/analytics_postgres.go) (`DATE_TRUNC`) vs [analytics_sqlite.go](backend/internal/service/analytics_sqlite.go) (`strftime('%Y-%m-%d', ...)` + Go-side parse back to `time.Time`).
  - pgdump.go + pgdump_test.go + backup_test.go are tagged `//go:build !sqlite` so they vanish from the portable binary.
- **Dialect helpers (runtime)**: [backend/internal/sqldialect/](backend/internal/sqldialect/) — same exported API in `dialect_postgres.go` / `dialect_sqlite.go`. `ILike(col)` / `ILikeAny(cols...)` → `ILIKE`/`LIKE` (SQLite LIKE is ASCII case-insensitive by default), `UnixEpoch(col)` → `EXTRACT(EPOCH)::bigint`/`CAST(strftime('%s', ...) AS INTEGER)`, `DatePlusDays(n)` → `(CURRENT_DATE + INTERVAL 'n days')`/`date('now', '+n days')`, `LockForUpdate()` / `LockForUpdateSkipLocked()` → `clause.Locking{...}`/no-op (SQLite serializes writes at the DB level; existing lock-then-mutate logic stays correct). **HARD RULE**: the no-op `clause.Expression` on SQLite must implement `Name() string { return "FOR" }` and `MergeClause(*clause.Clause)` — without those, GORM falls back to treating the expression as a WHERE condition and appends a trailing `AND`, which breaks UPDATE statements with `incomplete input` errors. Sites updated: every search RPC's ILIKE, every `clause.Locking` site (~14), DATE_TRUNC/EXTRACT(EPOCH) in `sales.go` + `analytics.go`, the `::date` cast in `medicines.go`, the `INTERVAL '30 days'` carve-out in `analytics.go`.
- **UUID generation**: dropped `default:gen_random_uuid()` from every model tag (SQLite has no equivalent). Replaced with a single `BeforeCreate` GORM callback registered in [backend/internal/db/db.go](backend/internal/db/db.go) `registerUUIDPKCallback`: walks the schema's primary fields, fills any empty string PK whose column type is `uuid` with `uuid.NewString()`. Driver-agnostic; Postgres still has the column default as a safety net for any code path that leaves `ID` empty (the callback's net effect is to make UUIDs Go-side instead of DB-side).
- **Parallel SQLite migration set**: [backend/migrations/sqlite/](backend/migrations/sqlite/) — 31 hand-translated `.sql` files matching the Postgres version IDs 1:1. Translation rules (applied once per file): `UUID` → `TEXT`, `TIMESTAMPTZ` → `DATETIME`, `BIGINT` → `INTEGER`, `BIGSERIAL` → `INTEGER PRIMARY KEY AUTOINCREMENT`, `CITEXT` → `TEXT COLLATE NOCASE`, drop `gen_random_uuid()` defaults + `::date`/`::bigint` casts, drop `CREATE EXTENSION` (file 00001 is a no-op `SELECT 1`). LPAD + window-function backfills (00020 supplier_code) drop because the portable build is fresh-DB only; the service layer guarantees uniqueness on insert. Partial unique indexes (`WHERE ...`) stay as-is (SQLite 3.8+ supports them). `ALTER TABLE ... ADD CONSTRAINT` / `SET NOT NULL` aren't supported by SQLite — for 00019_warehouses the TRANSFER enum values are baked into 00007 upfront and `stock_movements.warehouse_id` stays nullable in schema (service layer's `resolveWarehouse` enforces population on every write).
- **Tests under `-tags sqlite`**: `make test-e2e-sqlite`. [backend/e2e/helpers_sqlite.go](backend/e2e/helpers_sqlite.go) injects a fresh per-test SQLite file under `t.TempDir()`; `SetupEnv` runs `dbmigrate.Run` from an empty file before `EnsureBootstrapOwner`. Pg-only tests (`backup_test.go`, `pgdump.go`/`pgdump_test.go`) are tagged `!sqlite` so they vanish from the SQLite suite. Most of the suite passes against SQLite; documented known gaps below.
- **Portable installer**: [packaging/windows/apotech-portable.iss](packaging/windows/apotech-portable.iss) (Inno Setup) ships `apotech-portable.exe` + a baked `config.yaml` (driver=sqlite, owner email/password from the wizard, fresh JWT secret) + `apotech-portable-backup.bat` + a README. `PrivilegesRequired=lowest` (no admin). Optional **Run on Windows startup** writes an HKCU Run key (not a service). Built via [packaging/windows/build-windows-portable.ps1](packaging/windows/build-windows-portable.ps1) (`make installer-portable`); the setup script [setup-portable.ps1](packaging/windows/setup-portable.ps1) writes the config + creates `data/` and `backups/` dirs.
- **Two portable distribution shapes**: `make installer-portable` produces the Inno Setup `.exe` (Start-Menu shortcuts, uninstaller); `make zip-portable` produces `dist/ApotechPortable-<version>.zip` (drop-anywhere, USB-friendly, ~8 MB). The ZIP contains the same `apotech-portable.exe` + a credentials-prompt `first-run.bat` + a bilingual EN/ID `README.txt`. ZIP staging lives in [packaging/windows/zip-portable/](packaging/windows/zip-portable/); built via [packaging/windows/build-zip-portable.ps1](packaging/windows/build-zip-portable.ps1) (uses `Compress-Archive` — no external 7zip dependency).
- **`--backup` CLI flag**: `apotech-portable.exe --backup` runs `dbmigrate.Run` + one `CreateBackup` in-process and exits. Used by the portable backup `.bat` (wire to Task Scheduler for nightly snapshots).
- **Auto-browser-open**: new `server.open_browser_on_start: true` in config triggers a non-blocking `rundll32 url.dll,FileProtocolHandler http://host:port` (Windows) / `open` (macOS) / `xdg-open` (Linux) right after the listener is ready. Portable config sets it true; Postgres/Docker leave it false.
- **Restore (SQLite)**: stop `apotech-portable.exe`, copy `backups/backup_<ts>/database.db` over `data/apotech.db`, start again. No CLI dance, no admin.
- **Known gaps under the `sqlite` tag**:
  - `make test-e2e-sqlite` has a handful of stocktake/PO/warehouse-scoping tests that fail vs Postgres — they exercise paths that depend on the FOR UPDATE-driven serialization the Postgres flavor uses, which SQLite no-ops. Documented; non-blocking for the portable runtime.
  - `TestAnalytics_DimensionFanout` + `TestAnalytics_OrderIsWarehouseScoped` are TZ-sensitive at runtime (DATE_TRUNC vs Go local time near UTC midnight) and can fail on either flavor depending on host TZ. Pre-existing, not caused by this work.
  - No CI matrix yet (`.github/workflows/test.yml` runs only the default Postgres suite); follow-up.

## Testing

### Backend integration / E2E (Go)
- Tests live in `backend/e2e/`. Run with `make test-e2e`. Each `*_test.go` calls `e2e.SetupEnv(t)` to spin up an in-process `httptest.Server` wrapping the same handler stack `cmd/server/main.go` registers, backed by the **real dev Postgres** (`config.yaml`). Tests exercise the full Connect → service → DB stack over the wire.
- **Assertion library**: `github.com/stretchr/testify/require`. One-liner asserts, fatal on failure.
- **Side effect of `make test-e2e`**: `SetupEnv` calls `Users.EnsureBootstrapOwner` so the owner password gets reset to `cfg.Bootstrap.owner_password` on every run. Only run against the dev DB.
- **Adding a new test**: drop a `<feature>_test.go` file in `backend/e2e/`, call `e2e.SetupEnv(t)`, use the typed Connect clients on `env.Auth` / `env.Users` (extend `Env` for new services as they're tested).
- **What's covered today**: login (happy path, invalid credentials, public-when-authenticated). Refresh, Logout, role-guard enforcement, POS, inventory — **not yet tested**.

### Browser E2E (Playwright)
- Tests live in `frontend/tests/e2e/`. Run with `make test-browser` from the repo root (requires `make run` + `make web` already running in other terminals). Headless Chromium against the dev backend + dev DB.
- **Config**: [frontend/playwright.config.ts](frontend/playwright.config.ts). Single worker, retains traces/screenshots/video on failure, HTML reporter at `frontend/playwright-report/`.
- **Shared helpers** ([frontend/tests/e2e/_helpers.ts](frontend/tests/e2e/_helpers.ts)): `loginAs(page)`, `clearAuth(page)`, `OWNER` test-user constant, and an extended `test` fixture that **fails any test producing an unexpected browser console error** — this alone catches large classes of bugs (the BigInt JSON.stringify crash from the analytics page would fail every analytics test instantly). The `ALLOWED_CONSOLE_NOISE` allowlist mutes a few known-benign upstream warnings.
- **What's covered today** (Phase A — 12 tests):
  - `auth.spec.ts` — login success, wrong password stays on /login, logout clears tokens.
  - `analytics.spec.ts` — all three tabs render; date-range change refetches without console errors (BigInt regression).
  - `popups.spec.ts` — EntityDrawer Add/Cancel/Save; Branches required-field validation; Dialog F4-open + auto-focus; dismissal via Esc / Close [×] / outside-positioner click. **One `.fixme`**: drawer form-reset on Cancel (known bug — flip `.fixme` to `()` after fix).
- **Adding a new test**: drop a `<feature>.spec.ts` in `frontend/tests/e2e/`, import `{ test, expect, loginAs }` from `./_helpers`, use Playwright's `page` API. Prefer role-based selectors (`getByRole`, `getByText`) over CSS — they're resilient to Chakra class-name churn.
- **Test data**: same shared dev DB as the Go suite. Tests should use timestamped names (`e2e-customer-${Date.now()}`) and clean up after themselves (archive rows) until proper test-DB isolation lands.
- **Interactive debugging**: `cd frontend && npx playwright test --ui` opens the time-travel debugger. `npx playwright show-report` opens the last HTML report.

### Convenience
- `make test-all` runs both suites back-to-back.

## Frontend conventions

### ChakraUI-first (HARD RULE)
**Before writing any custom JSX or hand-rolled UI primitive, search the Chakra v3 component catalog (https://chakra-ui.com) and use the built-in equivalent.** Chakra ships rich components for nearly every common need: `Field` (form fields), `Input`, `Select`/`Combobox`/`NativeSelect`, `Switch`, `Checkbox`, `RadioGroup`, `Slider`, `Editable`, `Tabs`, `Table`, `Card`, `Tag`, `Badge`, `Avatar`, `Accordion`, `Tooltip`, `Popover`, `Menu`, `Drawer`, `Dialog`, `Toast`, `Steps`, `Progress`, `Stat`, `Skeleton`, `Spinner`, `Pagination`, `Pin Input`, `File Upload`, `Number Input`, `Date Picker`, `Tree View`, etc. **Compose Chakra components rather than rebuilding them.** Custom JSX is only acceptable when the requirement is genuinely outside Chakra's catalog. This rule supersedes any urge to hand-roll a UI primitive.

### No native confirm / alert / prompt (HARD RULE)
**Never use `window.confirm`, `window.alert`, or `window.prompt`.** For destructive-action confirmations (archive, delete, void, cancel), use the shared [`<ConfirmDialog>`](frontend/src/components/ConfirmDialog.tsx) component (or compose a Chakra `Dialog.Root` inline for richer flows like `SettingsBackups.tsx`'s delete-confirm). Native browser dialogs ignore the app's locale, theme, and accessibility — they look out of place against the rest of the Chakra UI.

### Selects (HARD RULE)
**Prefer richer Chakra select widgets over `NativeSelect`.** The native `<select>` element renders with OS-default chrome which looks inconsistent against the rest of the Chakra UI. Two reusable wrappers live in `frontend/src/components/`:

- **`<SearchableSelect>`** (wraps Chakra `Combobox`) — use for dynamic or long lists: medicines, customers, suppliers, batches, anything that can grow past ~10 items or benefits from type-to-filter.
- **`<EnumSelect>`** (wraps Chakra `Select`) — use for short fixed-enum option sets: status filters, role pickers, payment source, date-range presets, branch switcher.

Both expose the same prop API: `value`, `onChange(value)`, `itemToString`, `itemToValue`, `placeholder`, `disabled`, `size`, `width`. Both wrap in `<Portal>` so popovers escape drawer/dialog stacking contexts.

**`NativeSelect` is allowed only as a documented exception** — when the option set is ≤3 hardcoded items AND a popover feels heavyweight (rare). Add an inline comment explaining the choice. Today the codebase has zero `NativeSelect` usages — keep it that way unless you have a strong, justified reason.

#### Dynamic data MUST search server-side (HARD RULE)
When the options for a `<SearchableSelect>` come from a queryable domain that can grow with user activity — medicines, customers, suppliers, batches, prescriptions, sales, POs, anything similar — the select **MUST use the `loadOptions` (async) mode**, backed by a `Search<Domain>` RPC on the backend. **Do NOT** preload the full list via `useXxxQuery()` and pass it as `items`. The wrapper debounces input by 250ms, fires `loadOptions(query)`, and renders the result with a stale-result guard. No keystroke-per-RPC fan-out, no client-side filter pass over a giant array, no page-mount payload bloat.

The only carve-out for `items` (preload) mode: **≤20 hardcoded options** — status enums, payment sources, role pickers, date-range presets, branch switcher. Those use `items` or `<EnumSelect>`.

**To add a new dynamic select**:
1. Confirm or add a `Search<Domain>(query: string, limit: int32) → []<Domain>` RPC on the backend (model after `Customers.SearchCustomers` in [service/customers.go](backend/internal/service/customers.go)).
2. Add a `search<Domain>(query: string)` adapter in `frontend/src/queries/<domain>.ts` (model after `searchCustomers` / `searchSuppliers` / `searchMedicines` / `searchBatches`).
3. Pass it as `loadOptions={search<Domain>}` to `<SearchableSelect>`.
4. In edit-mode drawers (drawer mounts with `value` pre-set), pass `selectedLabel={record.name}` so the trigger immediately shows the right label instead of the raw UUID. The wrapper internally caches labels of picked items so subsequent renders stay correct.

**Note on display maps**: list/table pages that currently use `useXxxQuery()` for a `customerNameById` / `medById` display map can keep that preload as a separate concern (it's for the **table cells**, not the select). Future work: denormalize the name into the parent record's response so the preload can go away. The hard rule is about the SELECT's option source, not about every page-level data fetch.

### Tabs (HARD RULE)
**Never hand-roll a tab UI.** All tab strips in the app use Chakra v3's `Tabs` primitive (default `variant="line"`). Two choices depending on whether each tab is a route:

- **`<RouteTabs items={...}>`** ([components/RouteTabs.tsx](frontend/src/components/RouteTabs.tsx)) for **URL-driven** tabs — each tab is a sub-route under `<Outlet/>`. A **controlled** `Tabs.Root` (`value` from `useLocation()` via longest-prefix match) whose `onValueChange` calls react-router's `navigate(to)` — a client-side SPA route change, no full reload. Triggers are **plain `Tabs.Trigger` buttons, NOT `asChild` over `<NavLink>`**: the anchor form rendered an `<a href>` whose native navigation beat NavLink's SPA handler and reloaded the whole page on every tab click. The caller renders `<Outlet/>` separately below. Used by `/analytics`, `/purchasing`. (Tradeoff: tabs are buttons, so no middle-click/open-in-new-tab; deep-linking still works since each tab has its own URL.) `/inventory` no longer uses a tab strip — its sub-navigation moved to the sidebar's expandable **Inventaris** group (see Layout).
- **`Tabs.Root` directly** for **state-driven** tabs — one route, panels switch via internal Chakra state. Used by `/tax` (Issued invoices / NSFP pool).

Do NOT reach for `NavLink` + manual `borderBottomWidth` styling for a tab strip. If a new tabbed surface lands and the tabs are routes → `<RouteTabs>`. If they're not routes → `Tabs.Root`. Sidebar's left-rail nav is not a tab and is exempt (it uses `borderLeftWidth` accent, different semantics).

### Pagination (HARD RULE)
**Every list page is server-paginated** — never load a whole table client-side. The pieces:
- **`<Pagination>`** ([components/Pagination.tsx](frontend/src/components/Pagination.tsx)) — "Showing X–Y of N" + Prev/Next + a page-size `<EnumSelect>`. Render it under any paginated table. Page is **0-based**.
- **`usePageState(resetKey)`** ([lib/pagination.ts](frontend/src/lib/pagination.ts)) — owns `{ page, setPage, pageSize, setPageSize }`; pass a string built from the active filters as `resetKey` so the page snaps back to 0 whenever a filter changes. `DEFAULT_PAGE_SIZE = 25`.
- **List query hooks return `{ rows, total }`** (plus the React Query state), not a bare array. The hook sends `limit: pageSize, offset: page * pageSize`. Pages read `q.rows` / `q.total`, never `q.data`.
- **Referenced-name display maps use resolve-by-IDs, NOT a whole-list preload** — see the "Referenced names" HARD RULE below. `useAll<Domain>Query()` / `ALL_LIMIT` (= 1000) is reserved for the few surfaces that genuinely need the whole list in memory: the **POS medicine search/scan + stock aggregation**, the **Rx picker**, the **Transfers From/To selectors** (via `useAllWarehousesQuery` — warehouses are a small bounded set, so a single ALL_LIMIT call is cheaper than driving a server-side search for a 3-row picker), and the handful of `items`-mode preload selects. It does NOT apply to the dynamic `<SearchableSelect>` option source (that uses `loadOptions` + a `Search<Domain>` RPC), and it no longer applies to table/detail/filter name-maps (those resolve by IDs). `normPage` clamps any limit to 1000 so a genuine `ALL_LIMIT` preload returns the full list (it previously reset >200 to 25, silently truncating to the first page). Beyond 1000 rows even those surfaces need denormalization.

### Referenced names = resolve by IDs (HARD RULE)
To display a referenced entity's **name** from an ID (supplier/medicine/customer/batch on a list, detail, or filter), resolve the IDs the current page is showing via a batch **`Resolve<Domain>(ids)`** RPC and render `map.get(id)?.name ?? "—"`. The four resolve hooks live in [queries/refs.ts](frontend/src/queries/refs.ts): `useMedicineRefs` · `useSupplierRefs` · `useCustomerRefs` · `useBatchRefs`, each returning a `Map<id, Ref>` (deduped + sorted ids, `enabled` only when non-empty, `staleTime: 60_000`). Backend: one `Resolve<Domain>(ids) → []<Domain>Ref` RPC per domain (`WHERE id IN (ids)`, capped at 500 ids via `dedupeIDs` in [service/pagination.go](backend/internal/service/pagination.go), no enrich, role guards mirroring that domain's `List*`/`Search*`) — model after `ResolveCustomers` in [service/customers.go](backend/internal/service/customers.go). Refs are minimal: `MedicineRef{id,name,sku}`, `SupplierRef{id,code,name}`, `CustomerRef{id,name}`, `BatchRef{id,batch_number,medicine_id,medicine_name}`.
- **Never preload the whole catalog (`useAll*Query`) just to build a name map**, and **never render a raw ID** as a fallback — use `"—"` (a neutral placeholder) while resolving or when a name is genuinely missing.
- **Carve-outs (not gaps):** per-response denormalization the server already does stays (Transfers' warehouse names, Order/Sale-list `customer_name` + `medicine_name`, the PO list's item `medicine_name`/`medicine_sku`). And **POS keeps `useAllMedicinesQuery`** — it's a search/scan surface + client-side stock aggregation, not a name map. When a surface needs a domain *flag* the minimal ref doesn't carry (e.g. the Rx drawer's `prescription_required`), it may keep its preload; promote the flag onto the ref only if it becomes a genuine name-map need.
- **To wire a new referenced name:** (1) confirm/add `Resolve<Domain>` on the backend; (2) add a `use<Domain>Refs` hook in [queries/refs.ts](frontend/src/queries/refs.ts); (3) collect the page's IDs (`rows.map(r => r.fooId)`, plus the active filter id for a `<SearchableSelect>` `selectedLabel`) and render `refs.get(id)?.name ?? "—"`.

### Detail pages (HARD RULE)
**Every detail / single-record page (a `:id` route showing one entity) MUST render `<BackButton>`** ([components/BackButton.tsx](frontend/src/components/BackButton.tsx)) at the top of its content, with `to` pointing at its parent list (e.g. `/medicines`, `/purchasing`, `/inventory/stocktake`). It's a ghost "← Back" button that navigates client-side (`navigate(to)`, or `navigate(-1)` when `to` is omitted) — no full reload. Don't hand-roll back links; breadcrumbs in `<PageHeader>` are complementary, not a substitute. Today's detail pages: `MedicineDetail`, `PurchaseOrderDetail`, `StocktakeDetail`, `OrderDetail`.

### Visual identity & theme
- **Chakra `defaultSystem` only.** No custom system, no custom palette, no custom semantic tokens, no custom font. Brand accent uses the default `blue` palette via `colorPalette="blue"` on interactive components. Surface/text tokens (`bg`, `bg.subtle`, `bg.muted`, `fg`, `fg.muted`, `border`, `border.muted`) come straight from `defaultSystem` and flip light/dark automatically.
- **Typography**: default system font stack from Chakra `defaultSystem`. No Google Fonts.
- **Icons**: Lucide (`lucide-react`). Tree-shakable. Domain mapping (Pill = Medicines, Truck = Suppliers, Boxes = Batches, ArrowLeftRight = Movements, BarChart3 = Analytics, ShoppingCart = POS, Users = Users, LayoutDashboard = Dashboard).
- **Layout**: `AppShell` (sidebar + sticky top bar) wraps every authenticated route. Sidebar is 3-state responsive (expanded 240px on desktop, icon-only 64px when collapsed, slide-over drawer on mobile via the hamburger in `TopBar`). The rail is flat except for **one expandable group, "Inventaris"** (`<NavGroupItem>` in [Sidebar.tsx](frontend/src/components/Sidebar.tsx)): it expands to **Restock (`/purchasing`) · Pemasok · Batch · Mutasi · Stok opname · Transfer**, auto-expands when a child route is active, and (when the rail is icon-collapsed) renders its children as flat icon links. **Obat** (`/medicines`, the Medicines catalog) is its own top-level item — it is no longer an Inventory tab. Sidebar items filtered by role: CASHIER sees Dashboard+POS+Customers+Order history, PHARMACIST adds Obat+Inventaris(group)+Analytics+Prescriptions, OWNER adds Tax+Warehouses+Users. Login route uses a bare layout (no shell).
- **Pages start with `<PageHeader>`**: breadcrumbs → title → optional description → right-aligned actions → hairline divider. Non-sticky.
- **Create/Edit uses `<EntityDrawer>`** (slide-over from right). Modal-confirm reserved for destructive actions (future).
- **Data fetching = TanStack Query**: each domain has `src/queries/<domain>.ts` exporting `useXxxQuery` and `useXxxMutation` hooks that wrap the Connect client from [lib/clients.ts](frontend/src/lib/clients.ts). Query keys are tuples (`["medicines", "list", { includeInactive }]`). Mutations call `invalidateQueries` on the relevant list keys. Defaults: `staleTime: 30_000`, `refetchOnWindowFocus: false`, `retry: 1`. Don't fetch with raw `useEffect` + `setState`.
- **Forms = React Hook Form + Zod**: schema-first. Each form file declares a `z.object({...})`, derives `type FormValues = z.infer<typeof Schema>`, uses `useForm({ resolver: zodResolver(Schema) })`, and renders fields via the shared `<FormField>` component (wraps Chakra `Field.Root` + RHF `Controller`). No manual `setError`/`setBusy` boilerplate.
- **Money inputs = `<MoneyInput>`** ([components/MoneyInput.tsx](frontend/src/components/MoneyInput.tsx)), NOT a raw `<Input type="number">`. It's a controlled integer-money input: shows the value with the active locale's thousands separator (id → `200.000`), renders **empty with a `"0"` placeholder when the value is 0** (so typing never produces a leading zero like `09000`), and emits the **unformatted digit string** via `onChange` (`""` for empty/zero) so `z.coerce.bigint()`/`Number(raw || 0)` consumers parse cleanly. In a `<FormField>`, pass the **`money`** prop (drop `type="number"`); for a standalone field use `<MoneyInput value onChange>` directly. Grouping/parsing live in `formatThousands`/`parseThousands` ([lib/format.ts](frontend/src/lib/format.ts)). Quantity inputs stay plain `type="number"`.
- **Toasts**: import `toast` from [lib/toaster.tsx](frontend/src/lib/toaster.tsx). Methods: `toast.success`, `toast.info`, `toast.error`, `toast.fromError(err)`. **The global `QueryCache` + `MutationCache` route every unhandled error to a toast automatically.** Use `meta: { silentError: true }` on a query/mutation to opt out (e.g., Login surfaces a specific error message instead).
- **i18n hard rule**: **no hard-coded user-visible strings in components.** All UI copy lives in `src/locales/{en,id}.json`. Use `const { t } = useTranslation(); t("key.path")`. SKUs, IDs, and backend error codes are exempt. Date/money formatting goes through `src/lib/format.ts` (`formatMoney`, `formatDate`, `formatDateTime`, `formatUnix`) so it follows the active locale.
- **Dark mode**: `usePreferencesStore.theme` (`"light" | "dark"`) is the source of truth. The store's `setTheme` setter toggles a `data-theme` attribute + `dark` class on `<html>`, which is the platform mechanism Chakra v3 uses to flip its built-in `_dark` semantic-token values. No custom system, no FOUC script. On app boot, [stores/preferences.ts](frontend/src/stores/preferences.ts) re-applies the persisted theme; brief light-mode flash on cold load is acceptable.
- **Client state (Zustand)**: only for cross-cutting state. Today: `usePreferencesStore` (theme + locale + sidebar collapsed). Coming Phase 3: `useCartStore` (POS cart). Persisted to `localStorage` via `zustand/middleware`'s `persist`.
- **Route protection**: `<ProtectedRoute>` accepts `requiredRole?: Role` or `requiredRoles?: Role[]`. UI gating only; backend `auth.NewInterceptor` is the real enforcement.

## Auth model
- **Roles**: protobuf enum `auth_iface.v1.Role` — `ROLE_OWNER`, `ROLE_PHARMACIST`, `ROLE_CASHIER`. The DB stores the stripped string `"OWNER"|"PHARMACIST"|"CASHIER"` in `users.role` (see `roleEnumToString` in [backend/internal/auth/policy.go](backend/internal/auth/policy.go) and `roleFromProto`/`roleToProto` in [backend/internal/service/users.go](backend/internal/service/users.go)). Keeps rows human-readable in psql.
- **Login**: `AuthService.Login` returns an **access JWT** (HS256, signed with `cfg.Auth.JWTSecret`, default TTL `1h`) and an **opaque refresh token** (random 256-bit hex string). The client stores access in `localStorage["apotech_access_token"]` and refresh in `localStorage["apotech_refresh_token"]`. The access token is sent in `Authorization: Bearer <jwt>` on every request.
- **Refresh tokens**: random 256-bit opaque strings, stored **hashed** (SHA-256) in the `refresh_tokens` table. Every `AuthService.Refresh` call rotates: the presented token is marked `revoked_at = now()` and a new child token is issued with the same `family_id` and `parent_id` linking back. If a **revoked-but-not-expired** token is ever replayed, **the entire family is revoked** — visible signal of theft; the legitimate user gets logged out and re-prompts. `AuthService.Logout` revokes the family proactively. Default refresh TTL: `30d` (`auth.refresh_token_ttl` in `config.yaml`).
- **Frontend silent refresh**: `lib/transport.ts` has an interceptor that, on `Unauthenticated` from any RPC, calls `AuthService.Refresh` once (singleton in-flight promise so concurrent requests don't double-refresh), swaps both tokens in localStorage, and retries the original request. `AuthService.Refresh` itself is exempt to avoid infinite recursion.
- **Policy is declared in proto, not Go.** Each rpc carries one of:
  - `option (auth_iface.v1.public) = true;` — no auth (Login, Ping)
  - `option (auth_iface.v1.allowed_roles) = ROLE_xxx;` (repeated for multi-role) — authn + role-in-set
  - *no option* — authn only (e.g. `Me`, `ChangePassword`)
- **`auth.BuildPolicy()`** ([policy.go](backend/internal/auth/policy.go)) walks `protoregistry.GlobalFiles` at boot and produces `map[procedure]Policy`. Called once in `main.go` and passed to the interceptor.
- **`auth.NewInterceptor`** ([interceptor.go](backend/internal/auth/interceptor.go)) is the **single auth gate**. It (a) skips public procedures, (b) parses the JWT, (c) enforces `AllowedRoles`, (d) injects `auth.Principal{UserID, Role}` into the request context.
- **Handlers must NOT call role-check helpers.** They use `auth.MustPrincipal(ctx)` only for row-level decisions (e.g. `ChangePassword` self-vs-other). Adding a role check inside a handler is a code smell — the policy belongs in the proto.
- **Default policy**: a new RPC with no annotation is authenticated-only. To open it up you must explicitly mark `public = true`. Safe failure mode for forgotten annotations.
- **Bootstrap owner**: on every boot, `Users.EnsureBootstrapOwner` upserts the user described by `cfg.Bootstrap.owner_email/owner_password`. Empty email → skip. Changing the password in `config.yaml` rotates it on next restart.
- **Default-warehouse auto-grant**: both `EnsureBootstrapOwner` and `CreateUser` call `grantDefaultWarehouse(tx, userID)` ([service/users.go](backend/internal/service/users.go)) after the user lands — idempotent insert of a `user_warehouses` membership for the global default warehouse, with `is_default = true` only when the user has no other default. Closes the gap where migration 00019's at-migration-time grant produced **zero rows** on a fresh DB (no users to grant to yet), leaving the bootstrap owner with no access to MAIN.
- **Frontend tokens**: stored in `localStorage["apotech_access_token"]` and `localStorage["apotech_refresh_token"]`. Only `AuthProvider` ([lib/auth.tsx](frontend/src/lib/auth.tsx)) and the transport interceptor ([lib/transport.ts](frontend/src/lib/transport.ts)) touch them; everything else uses `useAuth()`. `logout()` is async — calls `AuthService.Logout` first (best-effort), then clears both keys.
- **Route protection**: `<ProtectedRoute>` (with optional `requiredRole`) wraps router children. UI gating only — the backend `auth.NewInterceptor` is the real enforcement.

## Analytics (Phase 4 — rewritten)
- **Aggregation strategy**: live Postgres queries only. No materialized views, no external OLAP. Existing indexes (`sales_completed_idx`, `sale_items_medicine_idx`, `stock_movements_created_idx`) carry the current dataset comfortably. If any query exceeds ~500ms, promote to a materialized view; not pre-built.
- **Proto package**: `analytics_iface.v1` — single `AnalyticsService` with **three dimension-scoped RPCs**:
  - `DailyMetric(metric_types, filter, sort, granularity)` → `{ days, order, stock }`. Days are bucket keys (`YYYY-MM-DD` / `YYYY-Www` / `YYYY-MM`) in the order the UI should render. **Not paginated** — range bounds the row count (worst case ~365 days for YTD).
  - `ProductMetric(metric_types, filter, sort, limit, offset)` → `{ medicine_ids, order, stock, total }`. Paginated.
  - `UserMetric(metric_types, filter, sort, limit, offset)` → `{ user_ids, order, total }`. Paginated. **STOCK in `metric_types` (or `Sort.field=stock`) returns `InvalidArgument`** — stock has no meaning per user.
- **Response shape (map + sorted ids list)**: each response carries a `repeated <id>` list (the sorted order) + `MetricOrder { map<id, OrderItem> }` and/or `MetricStock { map<id, StockItem> }` blocks. The frontend looks up display names via the existing **Resolve<Domain>(ids)** hooks ([queries/refs.ts](frontend/src/queries/refs.ts)) — the analytics handler returns IDs + numbers only, staying N+1-free. Same "Referenced names = resolve by IDs" HARD RULE.
- **Metrics**:
  - `OrderItem { terjual, hpp, profit }` — `terjual` = gross revenue, `hpp` = COGS (computed from SALE `stock_movements` × `batches.cost_price` joined by `sale_item_id`, identical to the deleted MarginAnalytics pattern), `profit` = `terjual - hpp`.
  - `StockItem { ready, ongoing }` — Daily: `ready` reconstructed AS-OF-END-OF-DAY from `stock_movements WHERE created_at < bucket_end`. `ongoing` is a current snapshot of open-PO outstanding (`SUM(po_items.ordered_qty - po_items.received_qty)` over non-VOIDED/CLOSED/RECEIVED POs) — historical reconstruction of in-flight POs is intentionally out of scope. Product: current per-medicine `ready` + `on_order`. Stock is **warehouse-scoped via `resolveWarehouse`**.
- **All ORDER + STOCK aggregations are warehouse-scoped** via `s.warehouse_id = resolveWarehouse(ctx, db, caller)` (sales) and `sm.warehouse_id = ?` (stock_movements). Sales carry `warehouse_id` from `StartSale` — the filter brings analytics in line with the rest of the app's active-warehouse convention. Switching the TopBar warehouse updates ALL columns (was: stock only). The on-PO `ongoing` carve-out stays company-wide (`purchase_order_items` has no warehouse column — same pre-existing carve-out).
- **Backend pattern**: 2-phase per RPC — Phase 1 builds the universe + sorts + paginates (returns ordered IDs); Phase 2 fetches metric blocks scoped `WHERE id IN (page_ids)`. Validation rejects duplicate / `_UNSPECIFIED` metric types and `Sort.field` referencing a metric not in `metric_types`. Single handler file ([backend/internal/service/analytics.go](backend/internal/service/analytics.go)) — replaces the 3 old `analytics_*.go` files.
- **Day/Week/Month granularity**: enumerated in Go (`enumerateBuckets`/`bucketStart`/`bucketNext` helpers) — portable and matches the per-bucket subquery style; avoids Postgres `generate_series` overload mismatches.
- **All RPCs are OWNER + PHARMACIST only** via proto-declared `allowed_roles`. CASHIER doesn't see analytics.
- **Frontend**:
  - Routes: `/analytics/daily` ([routes/analytics/Daily.tsx](frontend/src/routes/analytics/Daily.tsx)) with Table + Graph tabs; `/analytics/product` ([routes/analytics/Product.tsx](frontend/src/routes/analytics/Product.tsx)); `/analytics/user` ([routes/analytics/User.tsx](frontend/src/routes/analytics/User.tsx)). All three reuse the shared `<MetricTable>` ([components/MetricTable.tsx](frontend/src/components/MetricTable.tsx)) — sortable column headers, money + integer cells, label lookup via the `labelById` map. Daily additionally uses `<MetricGraphs>` ([components/MetricGraphs.tsx](frontend/src/components/MetricGraphs.tsx)) — one Recharts `<LineChart>` per active metric field.
  - **Column / metric visibility** is driven by a single **`<ColumnsPopover>`** ([components/ColumnsPopover.tsx](frontend/src/components/ColumnsPopover.tsx)) on each page's toolbar — a popover button (with a `visible/total` counter badge + Reset action) listing per-field checkboxes sectioned by metric group. Per-field granularity (`order.terjual` / `order.hpp` / `order.profit` / `stock.ready` / `stock.ongoing`); each page owns its own `Set<string>` of visible fields. The frontend derives the backend's `MetricType[]` from the selection via `fieldsToMetricTypes` ([lib/analyticsFields.ts](frontend/src/lib/analyticsFields.ts)) — any field in a group → group included. Backend stays unchanged: it ships the whole group's block; the table/charts hide unchecked columns client-side. On the User page the whole Stock section in the popover is disabled inline (tooltip `analytics.errors.userStockUnsupported`). Scales to any number of metric groups added later — same single-button affordance regardless of how many fields exist. Un-checking the sort column auto-clears the sort (`clearSortIfHidden` helper in Daily.tsx).
  - Resolve hooks: `useMedicineRefs` (Product), `useUserRefs` (User — new RPC `UserService.ResolveUsers` mirrors `ResolveCustomers`). Daily needs no resolve (day strings are the labels).
  - Query hooks: `useDailyMetricQuery` / `useProductMetricQuery` / `useUserMetricQuery` in [queries/analytics.ts](frontend/src/queries/analytics.ts).
- **Legacy URL shims** ([main.tsx](frontend/src/main.tsx)): `/analytics/{sales,margins,operations,profitability,inventory}` all `<Navigate>` to `/analytics/daily`. The 13 old RPCs (`GetRevenueTrend`, `GetTopSellers`, `GetPaymentMix`, `GetSalesByCashier`, `GetHourOfDayHeatmap`, `GetTurnover`, `GetDeadStock`, `GetDaysOfStockRemaining`, `GetExpiryRiskForecast`, `GetMarginPerMedicine`, `GetTopMargin`, `GetSupplierCostTrend`) and the `HourHeatmap` component are deleted.
- **CSV export**: deferred this round (was attached to old per-RPC tables; new MetricTable is generic and will get a column-aware export in a follow-up if asked). The shared `<ExportButton>` + `downloadCsv` infrastructure stays for the list pages.
- **Date range filter**: shared [components/DateRangeFilter.tsx](frontend/src/components/DateRangeFilter.tsx) with presets (Today / 7d / 30d / 90d / YTD / custom).

## Sales model (Phase 3)
- **Customers**: light table (`name`, `phone`, `bpjs_no`, `notes`, `active`). Sales may attach a customer or stay anonymous. CASHIER+PHARMACIST+OWNER can list/get/search/create; only OWNER+PHARMACIST can update/archive. `bpjs_no` column reserved for the BPJS integration phase.
- **Sale state machine**: `DRAFT → COMPLETED` or `DRAFT → VOIDED`. `ON_HOLD` is intentionally not modeled. Only DRAFT sales accept item/customer mutations.
- **Sale numbering**: human-friendly per-year format `INV-2026-0001`. A `sale_no_counters(year, last_seq)` row is incremented inside `CompleteSale`'s tx; the resulting `sale_no` is unique. UUID `id` is the primary key; `sale_no` is for human reference.
- **Money snapshots**: `sale_items.unit_price_snapshot` captures the price at the moment the line was added (via `AddItem`). Historical reporting therefore survives `medicines.unit_price` edits made after the sale. `line_discount` and `cart_discount` columns are placeholders for the upcoming discount-service slice; currently always 0.
- **FEFO consumption rule** lives inside `CompleteSale` (see [service/sales.go](backend/internal/service/sales.go)). For each cart line: load batches for that medicine ordered by `expiry_date ASC`, compute current per-batch stock from `stock_movements`, allocate greedily. If a line spans multiple batches, the placeholder `sale_items` row is deleted and one new row per consumed batch is inserted — each with its `batch_id` pinned and `stock_movements(type=SALE, qty=-N, sale_item_id=<that row>)` linked. This keeps the audit chain bidirectional: every SALE movement has a sale_item, every sale_item has a batch.
- **Insufficient stock**: if any line can't be fully allocated, the whole `CompleteSale` tx rolls back with `FailedPrecondition`. The user retries after adjusting qty or restocking.
- **`branch_id` placeholders**: added (nullable) to `sales`, `sale_items`, and `stock_movements` — all reserved for the multi-branch phase.
- **Today's snapshot**: `SaleService.GetTodaySnapshot` aggregates revenue / sale count / items sold / top medicine / **last-sale unix** for COMPLETED sales since `00:00 server-local`. **Warehouse-scoped via `resolveWarehouse`** + accepts an optional **`cashier_user_id`** filter (a non-OWNER caller may only pass their own id; enforced with `InvalidArgument`). Drives the role-aware Dashboard tiles.
- **Role-aware Dashboard** ([routes/Dashboard.tsx](frontend/src/routes/Dashboard.tsx)): branches on `user.role`. **OWNER** sees a "Business health · today" section with 6 tiles (revenue / profit / sales / items / avg basket / low stock / expiring-30d) + a 7-day revenue trend (Recharts `<LineChart>`); profit derives from `DailyMetric{ORDER, today, DAY}`; trend from `DailyMetric{ORDER, last 7d, DAY}`. **PHARMACIST** sees an "Inventory health" section: low stock + expiring-30d + active Rx. **CASHIER** sees "My shift · today": my revenue / my sales / my items / my last sale (`useTodaySnapshotQuery({ cashierUserId: user.id })`). Every tile is clickable (`<DashboardTile to=...>` → react-router Link). All tiles are warehouse-scoped (matches the analytics convention). Hooks: `useExpiringSoonCountQuery(days)` ([queries/batches.ts](frontend/src/queries/batches.ts)) wraps `ListBatches{onlyInStock, dateField=expiry, fromUnix=now, toUnix=now+Nd}.total`; `useActiveRxCountQuery` ([queries/prescriptions.ts](frontend/src/queries/prescriptions.ts)) wraps `ListPrescriptions{status: ACTIVE, limit: ALL_LIMIT}` and reads `.rows.length` (the handler applies the computed-status filter client-side — `total` is base-rows, so we count the post-filtered rows directly; cap 1000 is plenty for a single pharmacy).
- **POS UI**: route `/pos`, available to CASHIER + PHARMACIST + OWNER. Opts out of `AppShell` — full-screen via the bare layout branch in [App.tsx](frontend/src/App.tsx). Split view: medicine search (~60%, auto-focused, barcode-scanner friendly — SKU-exact-match-on-Enter auto-adds), cart panel (~40%, qty inline-editable, payment radio, change calc). Keyboard shortcuts: **F2** search · **F4** customer · **F5** Rx · **F8** complete · **Esc** clear. Cart state lives in the backend `sales` row (DRAFT); the UI fetches/mutates via TanStack Query (no separate cart store).
- **Rx-required auto-pick**: when the cashier clicks an Rx-required medicine and the sale has no prescription attached, POS does NOT call `AddItem`. Instead it opens the `PrescriptionPickerDialog` pre-filtered to ACTIVE prescriptions that include the clicked medicine with remaining qty, with a caption "Need prescription for: <medicine>". After the cashier picks an Rx, the wrapper calls `AttachPrescription` then immediately `AddItem` for the deferred medicine — a single user gesture lands both. If no covering Rx exists, the picker shows the localized empty state with a "Create a prescription" link to `/prescriptions`. The cashier never has to memorize F5 in the common case; the F5 shortcut still works for manual attaches and switching. Per-item remaining qty (`Paracetamol  3/10 remaining`) renders under each prescription card so the cashier sees coverage at a glance.
- **Receipt**: on-screen Chakra Dialog after CompleteSale. ESC/POS thermal printer wiring deferred to Phase 7.
- **Order history**: route `/orders` (OWNER + PHARMACIST + CASHIER) — a server-paginated list of sales with a debounced search (sale no / customer / medicine sold), a **state-driven `Tabs.Root` status filter** (All / Completed / Voided; defaults to COMPLETED — single route, so `Tabs.Root` directly per the Tabs rule), and an **always-visible `<DateRangeFilter>` (defaults to the last 30 days)**. Backed by `SaleService.ListSales` (`query` + `from_unix`/`to_unix` + `status` + `limit`/`offset`); the search joins `customers` + `sale_items` + `medicines` via an `id IN (sub)` subquery. `ListSales` denormalizes `Sale.customer_name` + `SaleItem.medicine_name` per page via `enrichSaleNames` (batch-loaded, no N+1); `Sale.warehouse_id` is also surfaced. A **summary bar** (Sales count · Items sold · Revenue) sits above the table, fed by **`SaleService.GetSalesSummary`** — a server-side SQL aggregate over **all** rows matching the *same* filters (NOT a client-side sum of the page). `ListSales` + `GetSalesSummary` share `applySaleFilters` ([service/sales.go](backend/internal/service/sales.go)) so list and summary always agree. **History shows finalized orders only**: there is no Draft tab, and `applySaleFilters` excludes DRAFT when status is UNSPECIFIED ("All"), so in-progress carts never appear; POS also **deletes an abandoned DRAFT cart on leaving `/pos` (and on warehouse switch)** via `SaleService.DiscardSale` (unmount cleanup in [Pos.tsx](frontend/src/routes/Pos.tsx) → raw `discardSale`, best-effort), and an in-process **sweeper deletes DRAFTs idle >24h** the client missed — so abandoned carts **vanish entirely** (no DRAFT *or* VOIDED trace). See "Abandoned-draft cleanup" below. An **Export CSV** button downloads all rows matching the active filters (see the CSV-export note); the CSV includes the "Created by" cashier name resolved via the imperative `resolveUserMap` helper ([queries/refs.ts](frontend/src/queries/refs.ts), mirrors `resolveSupplierMap`/`resolveBatchMap`). The list shows a **"Created by" column** (resolves `sale.cashier_user_id` via `useUserRefs`) and **rows are clickable** → `/orders/:id` ([routes/OrderDetail.tsx](frontend/src/routes/OrderDetail.tsx)). The detail page renders the standard `<BackButton>` + breadcrumbs + Info section (date / Created by / customer / payment / prescription / completedAt) + Totals card (subtotal · discount · total · paid · change) + Tax block (when `tax_invoice_no != ""`) + per-item table (medicine name + qty/unit + unit price snapshot + line discount + line total); referenced names resolve via `useCustomerRefs`/`useUserRefs`/`useMedicineRefs`. Sidebar entry "Order history" / "Riwayat order" (Receipt icon).
- **Abandoned-draft cleanup (delete, not void)**: a DRAFT sale has no `stock_movements` and no `sale_no`, so nothing references it — it's safe to **hard-delete**. `SaleService.DiscardSale(sale_id)` ([service/sales.go](backend/internal/service/sales.go); OWNER+PHARMACIST+CASHIER, same guards as `VoidSale`) loads the sale, rejects non-DRAFT with `FailedPrecondition`, then in one tx deletes `sale_items WHERE sale_id = ?` + the `sales` row. POS calls it (raw client, best-effort) on **leaving `/pos`** and on **warehouse switch** (replaces the old `VoidSale`, so abandoned carts no longer show up in the order-history Voided tab). A server-side **sweeper** ([service/draftsweeper.go](backend/internal/service/draftsweeper.go), started from [cmd/server/main.go](backend/cmd/server/main.go)) catches drafts the client missed: an in-process goroutine (`time.Ticker`, every **1h**) deletes DRAFTs whose `updated_at` is older than **24h** (an actively-edited cart bumps `updated_at`, so it's spared). In-process / single-node, like the rate limiter; constants, not configured. The exported `service.SweepStaleDrafts(ctx, db, maxIdle)` does one sweep (driven directly by the e2e test).

## Purchasing model (Phase 5)
- **Proto package**: `purchasing_iface.v1` with **three services**:
  - `PurchaseOrderService` — `ListPurchaseOrders` (with `only_outstanding` filter), `GetPurchaseOrder`, `CreatePurchaseOrder` (auto-assigns `po_no`), `UpdatePurchaseOrder` (DRAFT only, full item replace), `SendPurchaseOrder` (DRAFT→SENT), `VoidPurchaseOrder` (DRAFT/SENT only).
  - `PurchaseReceiptService` — `CreateReceipt` (the meaty piece — see below), `ListReceipts` (for a PO), `GetReceipt`.
  - `PurchasePaymentService` — `PayPurchase` (increments `paid_amount` + maybe closes), `GetSupplierBalances` (aggregated outstanding per supplier).
- **All RPCs are OWNER + PHARMACIST only** via proto-declared `allowed_roles`. CASHIER doesn't see purchasing.
- **PO state machine**: `DRAFT → SENT → PARTIALLY_RECEIVED → RECEIVED → CLOSED` (or `DRAFT/SENT → VOIDED`). `CLOSED` is reached when `paid_amount >= ordered_total` while `RECEIVED`. `recomputePOStatus` advances DRAFT/SENT/PARTIALLY_RECEIVED/RECEIVED based on summed `received_qty` vs `ordered_qty`; `maybeCloseIfPaid` finalizes to CLOSED after payment. Both helpers live in [service/purchasing_orders.go](backend/internal/service/purchasing_orders.go) and are called from receipts/payments services.
- **PO + receipt numbering**: human-friendly per-year format `PO-2026-0001` and `RCV-2026-0001`. `po_no_counters(year, last_seq)` and `rcv_no_counters(year, last_seq)` rows are incremented under `FOR UPDATE` lock inside the relevant tx. UUID `id` is the primary key; the `_no` is for human reference.
- **Receipt → batch creation chain** (the key piece, in `CreateReceipt`'s single tx):
  1. Lock the PO `FOR UPDATE`; reject unless status is `SENT` or `PARTIALLY_RECEIVED`.
  2. Assign `receipt_no` via the counter; insert `purchase_receipts` row.
  3. For each line: load the PO item, validate `qty ≤ ordered_qty - received_qty`, insert a new `batches` row (carrying `medicine_id`, `supplier_id` from the PO, `batch_number`, `expiry_date`, `cost_price` from the PO line — so cost basis tracks reality, not the original PO if it was edited), insert a `PURCHASE` `stock_movements` row (positive qty, links to batch), insert `purchase_receipt_items` with `batch_id` pinned, increment `purchase_order_items.received_qty`.
  4. Call `recomputePOStatus(tx, &po)` to advance the parent PO's status.
- **Stock arrival**: the only path that creates batches today is `PurchaseReceiptService.CreateReceipt`. The legacy `MedicineService.CreateBatch` (Phase 2) remains as an emergency manual-entry path but should not be the primary flow once a supplier+PO is involved.
- **Buy-in-units**: PO + receipt lines carry a chosen `purchasable` unit (`medicine_unit_id`/`unit_name`/`unit_factor`); `ordered_qty`/`received_qty`/`qty` stay in **base units** (entered qty × factor, converted at the edge in `CreatePurchaseOrder`/`CreateReceipt`). PO cost is the line total (per-base derived); receive is in the PO line's unit. See "Units of measure".
- **Per-PO payment tracking**: `purchase_orders.paid_amount` is updated by `PayPurchase`; no separate payment ledger today. If history becomes needed, add a `purchase_payments(id, purchase_order_id, paid_at, amount, method, note)` table — out of scope this phase.
- **Supplier outstanding balance**: `GetSupplierBalances` runs one SQL aggregation joining `suppliers` → `purchase_orders`, summing `ordered_total` and `paid_amount` grouped by supplier. `outstanding = SUM(ordered_total) - SUM(paid_amount)`. The `only_outstanding` filter adds `HAVING outstanding > 0`. Open POs counted via `COUNT(*) FILTER (WHERE status NOT IN ('CLOSED', 'VOIDED'))`.
- **`branch_id` placeholder**: nullable column on `purchase_orders` (reserved for the multi-branch phase, same as sales tables).
- **Supplier code (`00020`)**: `suppliers.code` is a unique, required, **editable** business code (backfilled `SUP-NNNN` for legacy rows). `CreateSupplier`/`UpdateSupplier` require it; `SearchSuppliers` ILIKEs it; every supplier `<SearchableSelect>` shows `code · name`.
- **Nomor faktur (`00021`)**: `purchase_receipts.invoice_no` (supplier faktur) is captured in the Receive dialog. The PO list surfaces the **most recent** receipt's `received_at` + `invoice_no` (denormalized onto the `PurchaseOrder` proto in `ListPurchaseOrders` via `enrichList`, alongside per-item `medicine_name`).
- **PO list query** (`ListPurchaseOrders`): besides `status`/`supplier_id`/`only_outstanding`/`limit`, supports `query` (ILIKE PO no / supplier name / supplier code / medicine name via an `id IN (subquery)` join), and a `from_unix`/`to_unix` range over `date_field` ("created" | "received"). `enrichList` batch-loads names + latest receipts (no N+1).
- **Route**: `/purchasing` — the tab row is **per-status routes** (`/all`,`/draft`,`/sent`,`/partial`,`/received`,`/closed`,`/voided`) + a **Pemasok** (Suppliers Ledger) tab; "Outstanding" is a toggle on the list, not a tab. `/purchasing/new` (create — line input is the **total cost**, unit cost derived = total/qty rounded), `/purchasing/:id` (detail; Send/Receive/Pay/Void). List shows PO# · supplier (`code · name`) · ordered items · status · created · received · faktur · total · outstanding, with a debounced server-side search. OWNER + PHARMACIST only. (Static status segments outrank the dynamic `:id` route; `Purchasing.tsx` `isSubpage` excludes the known tab paths.)

## Receipt printing (Phase 7-printer)
- **ESC/POS over raw TCP**, dispatched directly from the backend. No separate print-bridge daemon — most network thermal printers (Epson TM-T, Star, generic 58/80mm Chinese units) listen on port 9100 and accept raw bytes. For a single-shop local deploy the backend is on the same LAN as the printer; that's good enough today.
- **Package**: [internal/printer/](backend/internal/printer/) — three files, no new Go deps:
  - `escpos.go` — minimal `Builder` wrapping the byte commands actually used (init, align, bold, double-size, line, feed, cut, drawer kick).
  - `receipt.go` — `Render(Receipt, Settings)` turns a denormalized `Receipt` struct into a full ESC/POS byte stream. The package does NOT touch the DB — the caller (sales service) preloads medicine names, cashier name, and customer name.
  - `dispatch.go` — `DispatchTCP(address, bytes, timeout)`. Dial, write, close.
- **Config** ([config.example.yaml](config.example.yaml)): `printer.enabled`, `printer.address` (host:port), `printer.width` (32 for 58mm, 48 for 80mm), `printer.timeout`, `printer.open_drawer`, `printer.header[]` (shop name/address lines printed at top), `printer.footer[]` (closing lines). When `enabled: false`, `PrintReceipt` returns `FailedPrecondition` with a clear message.
- **RPC**: `SaleService.PrintReceipt(sale_id)` ([service/sales.go](backend/internal/service/sales.go)). Open to CASHIER + PHARMACIST + OWNER. Requires the sale to be in `COMPLETED` status — reprints work because the RPC is idempotent and stateless. Returns `bytes_sent` for debugging. Network/printer errors come back as `Unavailable`.
- **Frontend UX**: the receipt dialog after CompleteSale now has a **Print** button next to **New sale**. Reprint from the Sales list is not wired yet — straightforward follow-up if needed.
- **Multi-shop limitation**: TCP dispatch assumes backend and printer share a LAN. If the backend ever moves to a hosted deployment, swap the dispatch path for the original "render bytes, return them, client-side print bridge POSTs to localhost:9100" architecture (the `Render` / `Dispatch` split makes this a 1-file change).

## e-Faktur / tax invoice (Phase 7 sub-project 2)
- **Bookkeeping only**, not a live DJP integration. The actual e-Nofa / DJP web-service client would need a digital certificate + merchant credentials that aren't wired today. The shipped pieces:
  - `nsfp_pool` table — the seller pre-purchases NSFP (Nomor Seri Faktur Pajak) ranges from DJP via e-Nofa, then imports the range via `TaxInvoiceService.ImportNsfpRange`. Each NSFP body is stored as `XXX.YY.NNNNNNNN`.
  - `TaxInvoiceService.ImportNsfpRange` / `ListNsfp` / `ListTaxInvoices` / `GetTaxInvoice`. Owner-only (List/Get also Pharmacist).
  - Customer fields: `npwp` (15-digit tax ID, free-text formatted) and `address`. Customers with a non-empty NPWP are treated as PKP/B2B.
  - `sales.tax_invoice_no` / `tax_invoice_code` / `tax_invoice_dpp` / `tax_invoice_ppn` / `tax_invoice_issued_at` — populated atomically in `CompleteSale` when the attached customer has an NPWP (`AssignTaxInvoiceForSaleTx` in [service/tax.go](backend/internal/service/tax.go)).
- **Auto-assign rule** (inside `CompleteSale`'s tx): if the sale has a customer with non-empty NPWP, lock the lowest-numbered unused NSFP for the sale's fiscal year (`FOR UPDATE SKIP LOCKED`), mark it used, stamp the sale. If no NSFP is available, the whole `CompleteSale` rolls back with `FailedPrecondition` — better to fail loudly than miss a tax invoice.
- **PPN**: assumed 11% (Indonesia rate as of 2026). DPP = total × (100/111); PPN = total - DPP. `total` is VAT-inclusive (apotek convention).
- **CSV export**: client-side via [lib/csv.ts](frontend/src/lib/csv.ts) on the `/tax` route — DPP/PPN/total in BIGINT minor units (whole rupiah), one row per issued invoice. Suitable as a base for SPT Masa PPN reconciliation; map columns to the DJP CSV layout in a follow-up if you ever submit electronically.
- **Route**: `/tax` (OWNER only) — tabs Issued invoices / NSFP pool. Sidebar entry "Tax" / "Pajak".

## BPJS Kesehatan (Phase 7 sub-project 3 — STUB)
- **Local claim tracking only.** The actual BPJS API client requires merchant credentials + Apotek-Vendor certification with BPJS. Today `BpjsClaimService.SubmitClaim` is a stub that flips DRAFT→SUBMITTED with a timestamp; no HTTP call is made. Wire a real client in [service/bpjs.go](backend/internal/service/bpjs.go) where the `TODO: real BPJS HTTP call goes here` comment lives.
- **Schema** (`bpjs_claims`): one row per claim; FKs to `sales` + `customers`; status enum `DRAFT|SUBMITTED|APPROVED|REJECTED|PAID`; `external_ref` slot for the BPJS response code.
- **RPCs**: `ListClaims`, `GetClaim`, `CreateClaim` (from a sale, requires customer with `bpjs_no`), `SubmitClaim` (stub), `ResolveClaim` (manual ack: APPROVED/REJECTED/PAID + external_ref + note). OWNER + PHARMACIST.
- **Route**: `/bpjs`. Sidebar shows it when the user has a role with access.

## Multi-branch (Phase 8) — DEPRECATED, superseded by the Warehouse model
> Branches were placeholder scaffolding (stock was never partitioned by branch). The **Warehouse model** (above) replaces this: it is the real per-location stock concept. The tables/proto/service/columns below remain in the codebase but are **dormant** — the app no longer reads or writes `branch_id`, the Branch menu/selector/route are removed from the UI, and the `X-Branch-Id` header is no longer sent. Kept non-destructively; a later migration can drop them. The rest of this section is retained for historical context.
- **Schema**:
  - `branches(id, code, name, address, phone, active)` — seeded with one row `code='MAIN', name='Main pharmacy'` so existing single-shop data has a home.
  - `user_branches(user_id, branch_id, is_default)` — many-to-many. Partial unique index `(user_id) WHERE is_default` enforces one default branch per user.
  - Backfill: migration `00015_branches.sql` UPDATEs `sales / sale_items / stock_movements / purchase_orders / prescriptions` setting `branch_id` to MAIN where NULL. Existing user rows are joined to MAIN as their default.
- **Branch context flows through a header.** Frontend [lib/transport.ts](frontend/src/lib/transport.ts) sends `X-Branch-Id: <uuid>` on every RPC (value persisted in `localStorage["apotech_branch_id"]`). Backend [auth.NewInterceptor](backend/internal/auth/interceptor.go) reads it into `Principal.BranchID`. Handlers use `caller.BranchID` to stamp creates (e.g. `Sales.StartSale` stamps `branch_id` on the DRAFT sale).
- **`BranchService`** ([service/branches.go](backend/internal/service/branches.go)):
  - `ListBranches` — all roles.
  - `CreateBranch` / `UpdateBranch` / `ArchiveBranch` — OWNER only.
  - `GrantBranchAccess` / `RevokeBranchAccess` — OWNER only.
  - `ListUserBranches` — self for any role; other users for OWNER. Hydrates the branches alongside the memberships so the TopBar selector renders in one fetch.
  - `SetDefaultBranch` — self or, for OWNER, any user. Clears other defaults atomically.
- **Branch selector** lives in `TopBar` (hidden when the user has access to only one branch). Switching branches `window.location.reload()`s the page so every TanStack-Query cache is refetched with the new header.
- **`branch_id` columns stay nullable.** NULL means "unscoped / legacy" — backwards compatible with rows that predate Phase 8. Per-list branch filtering is opt-in per RPC; **only `Sales.StartSale` is currently retrofitted** to stamp branch on create. Adding `WHERE branch_id = ?` to existing List* queries is straightforward follow-up — left for when multi-branch is actually deployed.
- **Route**: `/branches` (OWNER only). Minimal admin: list + create. Editing/grants live in follow-up; SQL works fine for now.

## Production hardening (Phase 9)
- **Login rate limit** ([auth/ratelimit.go](backend/internal/auth/ratelimit.go)): in-process token bucket keyed by lowercase email. Defaults: capacity 5, refill 1/60s. Trips with `ResourceExhausted` and a friendly message. In-process means multi-replica deployments don't share counters — acceptable for single-shop local deploy; swap to Redis if you ever scale out.
- **Audit log** (`audit_log` table + [auth/audit.go](backend/internal/auth/audit.go)):
  - Records every NON-read RPC (List/Get/Search/Me/Ping are skipped to keep the table actionable).
  - Captures user_id, role, branch_id, procedure, ok/code/message, ip, user_agent, duration_ms.
  - Wired as a second interceptor on every handler in `main.go`. Writes are async (`go func`) on a detached context so audit overhead never blocks the user; a crash mid-write loses the queued row — acceptable for this app.
  - Query example in [DEPLOYMENT.md](DEPLOYMENT.md).
- **Password reset** ([service/users.go](backend/internal/service/users.go)):
  - `UserService.IssuePasswordResetToken` — OWNER only. Mints a 32-byte random token; raw value returned ONCE to the OWNER's UI; SHA-256 hash stored in `password_reset_tokens` with 24h TTL.
  - `UserService.RedeemPasswordResetToken` — public RPC. Takes the raw token + new password (min 8 chars); validates, updates the user's bcrypt hash, marks token used. Idempotent failure on replay (already-used token returns `Unauthenticated`).
  - **No SMTP integration.** Owner hands the token to the user out-of-band (verbal, SMS, paper). For a real email-based forgot-password flow, replace the OWNER-issue path with a Login-side `RequestPasswordReset(email)` RPC that mints and emails.
- **Structured logging via `log/slog`** (JSON handler on stdout). Boot in `main.go`. Adopt `slog.With(...)` in services as needed; today only main.go writes structured logs.
- **Backups**: see the **Database backups** section below — `BackupService` (in-app OWNER-only Create / List / Delete) + `make backup` (CLI / cron) write the same per-timestamp directory layout. Restore procedure in [DEPLOYMENT.md](DEPLOYMENT.md).
- **Deployment runbook**: [DEPLOYMENT.md](DEPLOYMENT.md) covers first-time setup, daily ops, backups, observability, security checklist, upgrade flow, and known limitations.
- **What was NOT shipped** in Phase 9 (out of scope of "execute all roadmap"):
  - Live SMTP-based forgot-password flow (placeholder above is OWNER-issued only).
  - Prometheus `/metrics` endpoint.
  - HA / multi-node deploy. Rate limit is in-process; the audit log is async-fire-and-forget.
  - Frontend UI for IssuePasswordResetToken / RedeemPasswordResetToken (only backend wired; users self-serve via direct API for now).

## Database backups
- **Per-timestamp directory layout** — every backup is its own folder, so the in-app feature, the CLI Makefile target, and the Windows installer's `.bat` all produce the same shape:
  ```
  <config.backup.directory>/backup_YYYY-mm-dd_HHMMSS/
    database.sql.gz   (pg_dump --compress=6; Windows .bat writes uncompressed .sql)
    manifest.txt      (created_at + app/db version + schema_version + size_bytes)
  ```
  `<config.backup.directory>` defaults to `./backups`, Docker uses `/var/lib/apotech/backups` (a named volume in `docker-compose.prod.yml`), Windows uses `C:\ProgramData\Apotech\backups`. Override at runtime with `APOTECH_BACKUP_DIR`.
- **In-app `BackupService`** (proto `backup_iface/v1`, OWNER-only): `CreateBackup` subprocesses `pg_dump` into a fresh `backup_<ts>/` dir + writes `manifest.txt`; partial dirs are removed on failure. `ListBackups` scans the directory (regex `^backup_\d{4}-\d{2}-\d{2}_\d{6}$`), sorts newest-first, surfaces size + parsed schema_version. `DeleteBackup` validates the name against the same regex (refuses `..` / arbitrary paths) then `os.RemoveAll`. Surfaced on the Settings page as a Backups section with a Create button + table (name · created · size · Delete) + confirm dialog. Refreshes every 60s.
- **CLI parity**: `make backup` (docker-compose; gzip) and `packaging/windows/apotech-backup.bat` (bundled `pg_dump.exe`; uncompressed) both write the same `backup_<ts>/database.sql{.gz} + manifest.txt` layout, so in-app and CLI backups are interchangeable in the listing.
- **Docker base image**: switched from `gcr.io/distroless/static-debian12:nonroot` to `debian:bookworm-slim` + `postgresql-client` so `pg_dump` is on PATH inside the container. ~80 MB heavier image, no longer "distroless"; in exchange the in-app Create-backup feature works without a sidecar. The container runs as a `apotech` non-root UID/GID (mirrors the distroless `nonroot` posture).
- **Restore is intentionally manual** (needs maintenance/read-only mode UX). Documented in [DEPLOYMENT.md](DEPLOYMENT.md): `gunzip < database.sql.gz | docker compose exec -T db psql -U apotech apotech` (or load the .sql directly on Windows).
- **pg_dump resolution** ([backend/internal/service/pgdump.go](backend/internal/service/pgdump.go) `resolvePgDump`): on every `CreateBackup`, look up the pg_dump binary in this order — (1) `exec.LookPath("pg_dump")` (Docker / Linux apt / Windows PATH), (2) **bundled next to the apotech binary** at `<exeDir>/pgsql/bin/pg_dump<.exe>` (this is the Windows installer's layout — fixes a latent bug where the installer ships pg_dump.exe but doesn't add `pgsql\bin` to PATH, so the in-app Create button used to fail even on a "properly installed" Windows host), (3) the cache `<cfg.Backup.PgToolsDir>/pgsql/bin/`, (4) on **Windows only**, with `autoFetchPgDump=true` (the prod constructor sets it; tests' `NewBackupsWithDir` sets it false so a missing pg_dump still skips cleanly without a 75 MB download), download EDB's official binaries zip (`PgToolsVersion = "16.4-1"`, **kept in sync with [packaging/windows/build-windows.ps1](packaging/windows/build-windows.ps1) `$PgVersion`**) into the cache, return the extracted `pg_dump.exe`. Step 4 only triggers once per cache; subsequent boots reuse it. Otherwise return `FailedPrecondition` with an OS-specific install hint (apt on Linux, libpq on macOS).
- **`backup.pg_tools_dir`** config key (env override `APOTECH_PG_TOOLS_DIR`) controls the cache root. Default is `os.UserCacheDir() + "/apotech/pgtools"` (Windows → `%LOCALAPPDATA%\apotech\pgtools`; Linux → `~/.cache/apotech/pgtools`). Docker never triggers the download (PATH hits at step 1), so the dir stays empty in containers.
- **Files**: `proto/backup_iface/v1/backup.proto`, `backend/internal/service/backup.go`, `backend/internal/service/pgdump.go` (resolver), `backend/e2e/backup_test.go` (skips when `pg_dump` isn't on host PATH — tests use the autoFetch=false constructor), `backend/internal/service/pgdump_test.go` (resolver cache-priority + missing-error tests), `frontend/src/queries/backup.ts`, Settings.tsx Backups section.

## Prescriptions model (Phase 6)
- **Proto package**: `prescription_iface.v1` with a single `PrescriptionService` (5 RPCs: `ListPrescriptions`, `GetPrescription`, `CreatePrescription`, `UpdatePrescription`, `VoidPrescription`). Sale-side attach/detach RPCs live on `pos_iface.v1.SaleService` (`AttachPrescription`, `DetachPrescription`) so they share the sale tx surface.
- **Role guards**:
  - `List` / `Get` open to **OWNER + PHARMACIST + CASHIER** (cashier needs them to pick a Rx during POS).
  - `Create` / `Update` / `Void` are **OWNER + PHARMACIST only** — cashiers don't issue scripts.
  - `AttachPrescription` / `DetachPrescription` open to all three roles (POS flow).
- **Issuer model**: free-text `issuer_name` column on `prescriptions`. No `doctors` table; matches paper-script workflow. Promote to a normalized table later if loyalty/reporting per doctor is ever scoped.
- **Customer required**: every prescription has a non-null `customer_id`. The patient is the script's anchor for `ListPrescriptions(customer_id=...)` and the POS picker.
- **Numbering**: per-year human-friendly `RX-2026-0001`. `rx_no_counters(year, last_seq)` row is incremented under `FOR UPDATE` lock inside `CreatePrescription`'s tx. UUID `id` is the primary key.
- **Status is computed, not stored** (except the binary ACTIVE/VOIDED). `computeRxStatus` ([service/prescriptions.go](backend/internal/service/prescriptions.go)) derives the live status from stored fields each time:
  - `VOIDED` — explicit `Status = 'VOIDED'`
  - `DISPENSED` — every item's `dispensed_qty == prescribed_qty`
  - `EXPIRED` — `now > expires_at + 1d` (end-of-day grace) and not voided/dispensed
  - `ACTIVE` — otherwise
- **Expiry**: `expires_at` defaults to `issued_at + 90d` server-side when omitted; editable on create/update. No background sweeper — status is read-through.
- **Partial dispensing across sales**: `prescription_items.dispensed_qty` accumulates per medicine line, with DB constraint `dispensed_qty <= prescribed_qty`. The same Rx can back multiple visits until fully dispensed.
- **Sales-side enforcement** (lives in [service/sales.go](backend/internal/service/sales.go)):
  - `AttachPrescription`: requires status `ACTIVE`; rejects DISPENSED/EXPIRED/VOIDED; rejects if sale already has a customer that doesn't match the Rx customer; auto-fills the sale's customer if it was empty.
  - `DetachPrescription`: blocked if any current cart line is Rx-required (forces user to remove those items first).
  - `AddItem` / `SetItemQuantity`: `assertRxCovers` checks that a medicine with `prescription_required = true` has an attached, ACTIVE prescription that includes the medicine with `prescribed_qty - dispensed_qty >= newQty`. Returns `FailedPrecondition` if any precondition fails — surfaced to the user via the global toast.
  - `CompleteSale`: after FEFO allocation succeeds, `incrementRxDispensed` sums per-medicine cart qty and bumps `prescription_items.dispensed_qty` for each Rx-required medicine. Same tx as the rest of CompleteSale, so rollback is atomic.
- **Edit constraint**: `UpdatePrescription` is allowed only while status is `ACTIVE` **and** no item has any `dispensed_qty > 0` yet. After the first dispense the script is effectively frozen — only `Void` is available.
- **Schema link**: nullable `prescription_id` FK on `sales`. Indexed for fast lookups (`sales_prescription_idx WHERE prescription_id IS NOT NULL`). `branch_id` placeholder column also present on `prescriptions` (reserved for the multi-branch phase, same as sales/POs).
- **POS UX**: a new "Attach prescription" bar under the customer bar in the cart panel. Shortcut **F5** opens the picker (filtered to ACTIVE Rx, scoped to the sale's customer when set). Search results show a red **Rx** badge on `prescription_required` medicines. Adding an Rx-required medicine without an attached prescription surfaces the backend's `FailedPrecondition` as a toast.
- **Route**: `/prescriptions` — single page with filter (status + customer) + table + create/edit drawer. OWNER + PHARMACIST only (sidebar). Edit drawer hides once any dispensing has happened; Void button stays available while ACTIVE.

## Inventory model
- **Money**: `unit_price` (medicines) and `cost_price` (batches) are `BIGINT` storing the **minor currency unit**. For IDR (no subdivision) that's whole rupiah. Never floats. Frontend formats with `Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' })`.
- **Stock ledger**: `stock_movements` is **insert-only**. Current batch stock = `SUM(qty) WHERE batch_id = $1`. No mutable counter on `batches`. Movement types: `PURCHASE` (inserted automatically by `CreateBatch` for `initial_quantity`), `SALE` (will come from POS in the next phase), `ADJUSTMENT`, `WRITE_OFF` (manual via `RecordMovement`). The handler refuses any movement that would drive a batch's stock below zero (post-insert check inside the tx; rollback on violation).
- **Concurrency / per-lot locking (HARD RULE)**: because stock is a `SUM` over the ledger (no balance column), under Postgres READ COMMITTED two concurrent transactions can each read the same availability and both insert → **oversell**. So **every stock-*consuming* path takes a `FOR UPDATE` lock on the `batches` row(s) before the read-check-insert**, using the `batches` row as the per-lot mutex. Helpers `lockBatchesByID(tx, ids)` / `lockBatchesByMedicine(tx, medicineIDs)` ([service/batches.go](backend/internal/service/batches.go)) lock in deterministic **`ORDER BY id`** order (one statement) so concurrent txs serialize per lot and never deadlock. Used by `CompleteSale` (locks the cart's medicines' lots before FEFO), `CreateTransfer` (line batches before the source-avail check), `RecordMovement`, and `CompleteStocktake` (before the negative guard). Additive paths (`CreateReceipt`/`CreateBatch` — PURCHASE only adds) need no lock. Stays on READ COMMITTED; no SERIALIZABLE/retry.
- **Pricing version**: `medicine_prices` is the authoritative price history. Exactly one row per medicine has `effective_to IS NULL` at any time — enforced by a partial unique index (`medicine_prices_open_idx`). `medicines.unit_price` is a denormalized "current price" cache. **All price writes go through `MedicineService.UpdateMedicine`'s tx**, which (a) **locks the `medicine` row `FOR UPDATE`** (serializes concurrent price edits so the close-open + insert-open sequence can't collide on the `*_open_idx` unique index and fail spuriously), (b) closes the current open row, (c) inserts a new open row, (d) updates the cache, all in one GORM transaction. Direct SQL writes to `medicines.unit_price` would diverge and must be avoided.
- **FEFO consumption**: not enforced yet. `ListBatches` sorts by `expiry_date ASC` so the UI surfaces soon-to-expire first. The actual "which batch to consume when selling" rule will land with POS.
- **Soft delete**: catalog entities use `active = false` rather than DELETE; rows referenced by movements or price history must persist.
- **Medicines list stock columns**: `ListMedicines` enriches each page (via `enrichStock`) with `ready_stock` = on-hand `SUM(stock_movements.qty)` in the **caller's active warehouse** (`resolveWarehouse`), and `on_order_stock` = `SUM(ordered_qty − received_qty)` across open POs (`status NOT IN (VOIDED, CLOSED, RECEIVED)`). The `/medicines` table renders these as **Ready** + **Ongoing** columns (Ongoing muted, "—" when 0). Both are computed per request — `ready` is active-warehouse-scoped like the rest of the app; **`on_order_stock` stays company-wide** (carve-out — `purchase_order_items` has no warehouse column, so incoming POs aren't in any warehouse yet).
- **Low-stock alerts**: a Bell in the TopBar (OWNER + PHARMACIST only — gated on `user.role !== Role.CASHIER`) polls `MedicineService.ListLowStock` every 60s and shows a count badge + dropdown of medicines whose `ready_stock` **in the caller's active warehouse** is `≤ low_stock_threshold`. Click an item → opens its detail. The list is one query: `LEFT JOIN batches ← stock_movements ON warehouse_id = ?`, `GROUP BY m.id`, `HAVING SUM(qty) <= ?`, `ORDER BY ready ASC`, capped at 100. Auto-refetches on warehouse switch (existing `invalidateQueries()`) and on threshold update. The threshold is one app-wide number stored via the Settings model (below); OWNER edits it at `/settings`.
- **App settings (key/value)**: a generic `app_settings(key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at)` table ([migration 00025](backend/migrations/00025_app_settings.sql), [model/app_setting.go](backend/internal/model/app_setting.go)) backs shop-wide settings. `SettingsService` ([proto/settings_iface/v1/settings.proto](proto/settings_iface/v1/settings.proto), [service/settings.go](backend/internal/service/settings.go)) exposes `GetSettings` (OWNER + PHARMACIST) and `UpdateSettings` (OWNER only). Today the only wired key is `low_stock_threshold` (default **10** when no row exists — read via `getLowStockThreshold` helper, shared with `MedicineService.ListLowStock`); the table is open for future keys without a new migration. Frontend: [queries/settings.ts](frontend/src/queries/settings.ts) + [routes/Settings.tsx](frontend/src/routes/Settings.tsx) (a tiny OWNER-only form under `/settings`, in the sidebar's OWNER section).
- **Obat (Medicines) page**: route `/medicines` (top-level sidebar item, OWNER + PHARMACIST) — a server-paginated, **server-searchable** list (debounced `query` → `ListMedicines`). **Rows are clickable → `/medicines/:id`** ([routes/inventory/MedicineDetail.tsx](frontend/src/routes/inventory/MedicineDetail.tsx)), where **Edit + Archive** live (the list keeps only "Add"). The Add/Edit forms are shared via [routes/inventory/medicineDrawers.tsx](frontend/src/routes/inventory/medicineDrawers.tsx) (`CreateMedicineDialog` on the list, `EditMedicineDialog` on the detail) and render as a **centered modal popup** ([components/EntityDialog.tsx](frontend/src/components/EntityDialog.tsx)) — a Dialog-based counterpart of `EntityDrawer` with the same prop API. (Obat is the one entity that uses a modal for create/edit, by request; other entities still use the slide-over `EntityDrawer`.) The form surfaces **markup % / margin % pricing** ([lib/pricing.ts](frontend/src/lib/pricing.ts) `marginPct`/`priceFromMarkup`): each price row (base + every unit) shows a live margin % vs the **reference cost** (`Medicine.reference_cost` × the row's factor) and a **markup %** input that derives that row's price (`sell = round(cost × (1 + markup%))`). Markup is over cost, margin over sell (matches analytics); it's a **frontend-only set-time helper** — derived prices persist via the unchanged `UpdateMedicine`. Hidden when `reference_cost` is 0 (no batch received yet, incl. the Create form). The **detail page**: an **Info** section (SKU/manufacturer/unit/price/Rx/active + tiles **Ready** [active warehouse], **On-order** [global — PO data model has no warehouse], **Stock valuation** [**active warehouse**, at cost = Σ qty × cost_price], **Last cost** [`reference_cost` = the latest batch's `cost_price` per base unit, global — the markup/margin reference], + **Last restock** = most recent stock arrival **into the active warehouse** · that batch's supplier — all `GetMedicine`-only enrichments). The old "Total stock" tile was **removed**: once scoped to the active warehouse it equals Ready (the `total_stock` proto field is still set, just unrendered). Then a **tab strip** (state-driven `Tabs.Root`): **Batches** (`ListBatches{medicine_id, only_in_stock}` — active-warehouse, in-stock-here lots only, expiry badge via the shared [components/ExpiryBadge.tsx](frontend/src/components/ExpiryBadge.tsx)) · **Riwayat harga / Price history** (`ListMedicineUnitPrices` — per-unit history) · **Mutasi / Recent movements** (`ListMovements{medicine_id}` — `medicine_id` filter + active-warehouse-scoped).

## Units of measure (box / strip / tablet)
- **Stock is stored in the smallest base unit** on the insert-only ledger; units convert only at the edges. This makes "buy a box, sell a tablet" trivial — FEFO consumes **base units** across batches.
- **Schema** (migration `00022_medicine_units.sql`): `medicine_units(id, medicine_id, name, factor BIGINT, is_base, sell_price BIGINT, sellable, purchasable, sort_order, active, …)` — `factor` = base units per 1 of this unit (base = 1); partial unique `(medicine_id) WHERE is_base` (one base), partial unique `(medicine_id, name) WHERE active`; **independent `sell_price` per unit**. Backfill: one base row per existing medicine from `medicines.unit`/`unit_price` (which stay as the base-unit cache). `sale_items` gains `medicine_unit_id` + `unit_name` + `unit_factor` + `base_qty` (= qty × factor; backfilled = qty for old rows).
- **Proto**: `Medicine.units` (`MedicineUnit[]`); `Create/UpdateMedicineRequest.units` (`MedicineUnitInput[]` — the **non-base** larger units; base comes from `unit`/`unit_price`). `AddItemRequest.medicine_unit_id` (empty → base); `SaleItem` carries `medicine_unit_id`/`unit_name`/`unit_factor`/`base_qty`.
- **Service** ([service/medicines.go](backend/internal/service/medicines.go)): `syncMedicineUnits` upserts the base (from `unit`/`unit_price`) + non-base units (removed ones deactivated); `attachUnits` hydrates `Medicine.units` on Get/List/Search/Create/Update. ([service/sales.go](backend/internal/service/sales.go)): `AddItem` resolves the sellable unit, snapshots `unit.sell_price`, stores `base_qty`, merges a draft line per **medicine+unit**; Rx coverage/dispense work in **base units**. `CompleteSale` FEFO consumes `base_qty` across batches and writes one SALE movement per batch linked to the line — it **no longer splits `sale_items` per batch** (one line per selling unit; `sale_items.batch_id` unused).
- **COGS** ([service/analytics_margin.go](backend/internal/service/analytics_margin.go)) now sums **SALE `stock_movements`** (`|qty| × batches.cost_price`, by `sale_item_id`), not `sale_items.batch_id` — correct under multi-unit + multi-batch lines.
- **Frontend**: medicine form ([routes/inventory/medicineDrawers.tsx](frontend/src/routes/inventory/medicineDrawers.tsx)) has a units editor (base = the unit/price fields; add larger units with name + factor + sell price); detail page shows the units; POS ([routes/Pos.tsx](frontend/src/routes/Pos.tsx)) cart line has a per-line unit `<EnumSelect>` (when >1 sellable unit) that re-adds at the chosen unit. Stock everywhere still displays in **base units**.
- **Quantity aggregations count base units**: `GetTopSellers`, `GetSalesSummary`, and `GetTodaySnapshot` sum `base_qty` (NOT `qty`, which is now the selling-unit count — a "1 box" sale counts 100, not 1). **Unit names shown on quantity displays**: thermal receipt ([printer/receipt.go](backend/internal/printer/receipt.go) `ReceiptLine.UnitName`), the POS on-screen receipt, the order-history item list, and PO/receipt rows render the selling/purchasable unit name (bare number kept when `unit_name` empty).
- **Phase 2 — buy-in-units + per-unit sell-price history (shipped)**:
  - **Buy in units** (migration `00023_purchasing_units.sql`): `purchase_order_items` + `purchase_receipt_items` gain `medicine_unit_id`/`unit_name`/`unit_factor`; `ordered_qty`/`received_qty`/`qty` **stay BASE units**. `resolvePurchaseUnit` ([service/purchasing_orders.go](backend/internal/service/purchasing_orders.go), filters `purchasable && active`) resolves the unit; `CreatePurchaseOrder`/`UpdatePurchaseOrder` + `CreateReceipt` convert the entered qty → base at the edge (`base = qty × factor`), so `recomputePOStatus`/batch/movement logic is unchanged. PO cost is entered as the **line total** (per-base cost derived = `total / base_qty`). Receive uses the **PO line's unit** (no separate receive-unit picker). [NewPurchaseOrder.tsx](frontend/src/routes/purchasing/NewPurchaseOrder.tsx) + the ReceiveDialog show a unit picker / unit name; `SearchableSelect` gained an `onSelectItem` callback so the form can capture the picked medicine's units.
  - **Per-unit sell-price history** (migration `00024_medicine_unit_prices.sql`): `medicine_unit_prices` mirrors `medicine_prices` keyed by `medicine_unit_id` (one open row per unit; `changed_by` nullable for the backfill). `syncMedicineUnits` writes a version row when a unit's `sell_price` changes and seeds one on create (via `recordUnitPrice`). `MedicineService.ListMedicineUnitPrices(medicine_id)` returns the per-unit history; the medicine-detail **Price history** tab renders it (Unit · From · To · Price) — supersedes the base-only `ListMedicinePrices` view (the latter + `medicine_prices` are kept for back-compat).
- **Still deferred**: humanized multi-pack display ("2 box 4 strip 5 tablet") + per-unit availability hint in POS; a receive-unit picker (receive in a different unit than the PO line); **prescriptions Rx-unit affordance** (Rx `prescribed_qty`/`dispensed_qty`/coverage are all base units but the Rx form has no unit picker/label).

## Stocktake model
- **Two new tables + two new columns on `stock_movements`** (migration `00018_stocktake.sql`):
  - `stocktake_sessions(id, name, status, branch_id, created_by, created_at, completed_at, voided_at)` — status CHECK ∈ {`DRAFT`,`COMPLETED`,`VOIDED`}.
  - `stocktake_lines(id, session_id, batch_id, expected_qty, counted_qty, disposition, write_off_kind, disposition_note, counted_at, counted_by, created_at)` — `counted_qty` nullable until counted; `disposition` ∈ {`ADJUSTMENT`,`WRITE_OFF`} default `ADJUSTMENT`; `write_off_kind` ∈ {`EXPIRED`,`DAMAGED`,`LOST`,`THEFT`,`OTHER`} nullable; `UNIQUE(session_id, batch_id)`.
  - `stock_movements.stocktake_line_id UUID NULL REFERENCES stocktake_lines(id)` — populated by `CompleteStocktake`, NULL for ad-hoc adjustments. Enables auditing every stocktake-driven movement back to the count event.
  - `stock_movements.write_off_kind TEXT NULL` — propagated from the line's `write_off_kind` when a stocktake produces a WRITE_OFF movement. NULL for ad-hoc Record-Movement write-offs. Enables `SUM by write_off_kind` analytics across all stocktake-generated write-offs.
- **Proto package**: `stocktake_iface.v1` with a single `StocktakeService`. RPCs: `StartStocktake`, `AddBatchesToSession`, `AddAllInStockBatches`, `RecordCount`, `SetLineDisposition`, `RemoveLine`, `CompleteStocktake`, `VoidStocktake`, `ListStocktakes`, `GetStocktake`. Role guards: OWNER + PHARMACIST everywhere; `RecordCount` additionally accepts CASHIER (the floor-walking step — a helper can punch in counts but cannot start/complete sessions or change disposition).
- **One DRAFT session per warehouse.** `StartStocktake` resolves the active warehouse, then rejects with `FailedPrecondition` only if a DRAFT already exists **in that warehouse** ("a draft stocktake is already open in this warehouse") — so two warehouses can be counted concurrently, but not two sessions in the same warehouse. **Warehouse-scoped throughout**: the session carries `warehouse_id` (+ hydrated `warehouse_name`, shown as a badge on the detail page); `ListStocktakes` is scoped to the caller's active warehouse (`resolveWarehouse` → `WHERE warehouse_id = ?`, header-driven like `ListMovements`); counting (seed/expected_qty/completion movements) is per the session's warehouse (below).
- **Per-batch granularity.** Each line = one batch. `expected_qty` is **snapshotted at line creation** via the existing `batchCurrentQty()` helper in [service/batches.go](backend/internal/service/batches.go). Concurrent SALE movements between snapshot and Complete will reflect as variance; pharmacist runs stocktake outside operating hours or accepts the drift.
- **Per-line disposition** covers broken goods inside the same workflow. Each line carries `disposition` ∈ {`ADJUSTMENT`,`WRITE_OFF`}, an optional structured `write_off_kind` (required when disposition is WRITE_OFF), and a free-text `disposition_note`. Defaults to `ADJUSTMENT` with no kind. Single shelf walk handles reconciliation + write-offs; structured kinds enable per-cause reporting (`SUM WHERE write_off_kind = 'EXPIRED'`).
- **Completion** (`CompleteStocktake` in [service/stocktake.go](backend/internal/service/stocktake.go)) runs in one DB tx. (1) Validates every line: positive variance forbidden with `WRITE_OFF`; `WRITE_OFF` requires a `write_off_kind`. Fails the entire tx with `FailedPrecondition` if any line violates. (2) For each counted line with `counted_qty != expected_qty`, inserts one `stock_movements` row: `qty = counted_qty - expected_qty` (signed), `type = line.disposition`, `reason = "Stocktake: <name> — <KIND> — <note>"`, `stocktake_line_id = line.id`, `write_off_kind = line.write_off_kind`. (3) Marks session `COMPLETED`. Lines never counted produce no movements.
- **Negative-stock guard** runs after each insert inside the same tx: if applying the variance would drive a batch's stock below zero, the whole Complete rolls back.
- **`VoidStocktake`** marks a DRAFT session `VOIDED` and writes no movements.
- **Route**: `/inventory/stocktake` (list) + `/inventory/stocktake/:id` (detail). Sub-tab added to the Inventory tab strip. Detail page hides the tabs and shows a breadcrumb back to the list. Inline `counted_qty` input commits on blur/Enter; when variance < 0, a per-line `<EnumSelect>` for disposition appears, and when disposition flips to WRITE_OFF a second `<EnumSelect>` for `write_off_kind` + a free-text note field renders below. Positive variance hides the disposition controls entirely (always ADJUSTMENT).

## Warehouse model (multi-warehouse / gudang)
**Warehouses replace branches** as the stock-location concept. Stock is partitioned per warehouse and movable between warehouses. (The `branches`/`user_branches` tables + `branch_iface` proto + `BranchService` + `branch_id` columns remain in the codebase but are **dormant/deprecated** — the app no longer reads or writes them. A later migration can drop them.)
- **Stock lives on the ledger, not the batch.** A `batch` is a **global lot** (no warehouse column). Per-warehouse stock = `SUM(stock_movements.qty) WHERE batch_id AND warehouse_id`. The same lot can exist in multiple warehouses. Two helpers in [service/batches.go](backend/internal/service/batches.go): `batchCurrentQty` (global total) and `batchQtyInWarehouse(batchID, warehouseID)` (per-location).
- **Every movement carries `warehouse_id` (NOT NULL).** Migration `00019_warehouses.sql` adds it, backfills all existing rows to the seeded default warehouse `MAIN` ("Gudang Utama"), then sets NOT NULL. All create-paths stamp it via `resolveWarehouse(ctx, db, caller)` ([service/warehouses.go](backend/internal/service/warehouses.go)) which returns the `X-Warehouse-Id` header → else the user's default membership → else the global default warehouse (never empty).
- **Schema** (`00019_warehouses.sql`): `warehouses(id, code, name, address, phone, is_default, active, …)` (partial unique index on `is_default`); `user_warehouses(user_id, warehouse_id, is_default)` (mirrors `user_branches`); `stock_transfers(id, transfer_no, from_warehouse_id, to_warehouse_id, note, created_by, created_at)` + `transfer_no_counters`; `stock_movements.warehouse_id` (NOT NULL) + `stock_movements.transfer_id`; `sales.warehouse_id`; `stocktake_sessions.warehouse_id`. The `stock_movements.type` CHECK is widened with `TRANSFER_IN`, `TRANSFER_OUT`.
- **Proto**: `warehouse_iface.v1` with **two services** — `WarehouseService` (List/Create/Update/Archive + GrantAccess/RevokeAccess/ListUserWarehouses/SetDefaultWarehouse; CreateWarehouse auto-grants the creator) and `StockTransferService` (`CreateTransfer`, `ListTransfers`, `GetTransfer`; OWNER + PHARMACIST).
- **Active warehouse context** flows through the **`X-Warehouse-Id` header** (frontend [lib/transport.ts](frontend/src/lib/transport.ts) sends `localStorage["apotech_warehouse_id"]`; [auth.NewInterceptor](backend/internal/auth/interceptor.go) reads it into `Principal.WarehouseID`). Switching the active warehouse (TopBar selector + POS gate) writes the new id then calls `queryClient.invalidateQueries()` — **no full page reload**; the transport reads `localStorage` per request so refetches carry the new header in place.
- **Warehouse admin** (`/warehouses`, OWNER): server-paginated list (default 25/page) + **create + edit** (name/address/phone; `code` is immutable, shown read-only in the edit drawer) + **archive** (hidden for the default / inactive). Backed by `WarehouseService.ListWarehouses` (limit/offset/total) + `UpdateWarehouse` / `ArchiveWarehouse`. Selector callers (Transfers From/To pickers, TopBar — via `ListUserWarehouses` which is per-user-membership and intentionally unpaginated) use `useAllWarehousesQuery` (a thin wrapper over `useWarehousesQuery({ pageSize: ALL_LIMIT })`).
- **POS / Sales is the enforced per-warehouse surface.** `Sales.StartSale` stamps `sale.warehouse_id`; `CompleteSale` FEFO ([service/sales.go](backend/internal/service/sales.go)) allocates batches **only from `sale.warehouse_id`** (insufficient there → `FailedPrecondition`); SALE movements carry that warehouse. `GetStockLevels` + `ListBatches` report qty for the caller's **active** warehouse, so the POS medicine search shows that warehouse's availability.
- **All inventory *reads* are active-warehouse-scoped** (not just POS): `GetStockLevels`, `ListBatches`, `ListMovements` (Mutasi page + medicine-detail Movements tab + Mutasi CSV), `GetBatch`/`SearchBatches` qty, `ListMedicines.ready_stock`, the medicine-detail **Stock valuation** + **Last restock**, Stocktake, and the four **Inventory Analytics** (turnover / dead-stock / days-of-stock / expiry). Each resolves `warehouseID := resolveWarehouse(ctx, db, caller)` and adds `AND sm.warehouse_id = ?` to the `stock_movements` join/where (or uses `batchQtyInWarehouse` over the global `batchCurrentQty`). **Carve-out: `on_order_stock` stays company-wide** — `purchase_order_items` has no warehouse column (incoming POs aren't in any warehouse yet). SalesAnalytics / MarginAnalytics are sales-domain and untouched.
- **List *rows* are scoped too, not just the qty number**: the **Batches list** + medicine-detail **Batches tab** pass `only_in_stock: true` so a global lot only appears when it has stock in the active warehouse (`ListBatches`' warehouse-scoped `HAVING SUM(sm.qty) > 0`); a lot whose stock is entirely in another warehouse no longer shows at qty 0. The **Mutasi list** rows are filtered by `warehouse_id`. The **Transfers list** (`ListTransfers`) defaults to transfers **touching the active warehouse** (`from_warehouse_id = ? OR to_warehouse_id = ?` via `resolveWarehouse`; the explicit `warehouse_id` request param overrides) — frontend sends nothing extra, the header drives it. A batch *lot* itself stays global (no `warehouse_id` on `batches`).
- **POS warehouse selection = the shared `<WarehouseSelect>` popup** ([components/WarehouseSelect.tsx](frontend/src/components/WarehouseSelect.tsx)) — the same searchable modal used by the TopBar + Transfers (standardized). POS is full-screen (no TopBar), so on entering `/pos` a **gate** (rendered only when the user has >1 warehouse and none is persisted) shows the picker; choosing one persists `apotech_warehouse_id` and mounts the cart. The cart **header** carries the same `<WarehouseSelect>` to switch in place — picking a different warehouse voids the in-progress DRAFT cart and refetches (no reload, no re-gate). Single-warehouse users skip the gate and see a static warehouse-name label.
- **Rx-required row cue**: in the POS search list, an Rx-required medicine with no prescription attached renders dimmed with a lock + "perlu resep" cue but stays clickable (still auto-opens the picker).
- **Stock transfers (mutasi)**: `StockTransferService.CreateTransfer` runs one tx — validates `from != to`, assigns `TRF-YYYY-NNNN`, and per line validates `batchQtyInWarehouse(from) >= qty` then writes a `TRANSFER_OUT` (−qty @ from) + `TRANSFER_IN` (+qty @ to) pair linked via `transfer_id`. Negative-source guard. UI: an Inventory sub-tab `/inventory/transfers` (list + create drawer).
- **Receive / manual-adjust / stocktake** stamp `warehouse_id` from the active warehouse implicitly (no per-surface warehouse picker yet — deferred). Stocktake's `expected_qty` snapshot + completion movements are scoped to the session's warehouse.
- **resolveWarehouse trusts the header** (no membership enforcement) — acceptable for a trusted single-pharmacy staff; membership drives the UI selector only.
- **Deferred** (data model supports them): an explicit warehouse picker / "all warehouses" toggle on analytics + medicine detail (reads already scope to the *active* warehouse, but you can't pick a different one or aggregate across all from those surfaces); explicit warehouse pickers in the receive + stocktake dialogs; warehouse columns on purchase orders / prescriptions; dropping the dormant branch tables.

## Known gaps (not yet implemented)
- No CI runner — both `make test-e2e` (Go integration) and `make test-browser` (Playwright) run locally only. Wiring `.github/workflows/test.yml` is the planned next step.
- Tests share the dev DB (read-mostly today). Once test suites grow write-heavy (full POS / Purchasing coverage), introduce a separate `apotech_test` database or transactional rollback per test.
- Browser E2E coverage is the Phase-A scaffold only: auth + analytics + popups. POS / Purchasing / Prescriptions / Tax / Branches / role-gating specs are documented in the testing plan but not yet written.
- No live BPJS Kesehatan API client — Phase 7-BPJS is a local-tracking stub. Requires merchant credentials + Apotek-Vendor certification with BPJS.
- No live DJP / e-Faktur web-service client — Phase 7-eFaktur is bookkeeping only. The shipped CSV is a base for SPT Masa PPN reconciliation but not the official DJP-submission format.
- No SMTP-based forgot-password flow — Phase 9 ships an OWNER-issued one-shot reset token (raw value handed off out-of-band).
- No frontend UI for password reset issue/redeem — the RPCs exist; wire `/reset?token=...` in a follow-up.
- No Prometheus `/metrics` endpoint; logs are structured JSON via `log/slog` on stdout only.
- Per-list branch filtering is opt-in per RPC — only `Sales.StartSale` stamps `branch_id` on create today. Existing List* queries don't yet filter by `caller.BranchID`; add `WHERE branch_id = ?` as multi-branch traffic appears.
- No discounts in POS (line and cart discounts are schema-ready but the `DiscountService` proto + UI controls are deferred).
- No returns / refunds flow.
- Prescriptions are unit-unaware in the UI — Rx `prescribed_qty`/`dispensed_qty` are base units (internally consistent with POS coverage/dispense) but the Rx form has no unit picker/label. Deferred follow-up.
- No barcode scanning hardware wiring (POS search input is scanner-friendly via SKU-exact-Enter, but no HID/serial layer).
- No reprint from the Sales list (Print is wired on the receipt dialog only).
- Admin "force logout user" RPC (data model supports it via `refresh_tokens.user_id`; ship later).
- Login rate limiter and audit-write queue are in-process — single-node only. Move to Redis + a real write-ahead log for HA.

## Out of scope (for the current phase)
Do not invent these without an explicit user request:
- Discount controls in POS (line + cart discounts)
- Multi-tenant / multi-branch (branch_id columns are placeholders only)
- Production deployment, CI, Dockerfiles for app code

## Update policy
Update this file when any of the following changes:
- A new top-level directory is added.
- A dependency is added, removed, or replaced.
- A convention is introduced or changed.
- A new service/proto package is added.
- The "current phase" or scope changes.

**Roadmap section is mandatory.** This file is the only place a fresh agent looking at the repo will learn what comes next. Keep the Roadmap table current: move the 🚧 pointer at the start of each phase; flip its row to ✓ shipped at the end. Per-phase implementation detail (schemas, RPC lists, file lists) lives in the per-user plan file, not here.

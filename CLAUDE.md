# CLAUDE.md — Apotech Project Memory

Living doc for Claude. Update this file whenever the stack, layout, or conventions change (new service, new dep, new top-level dir).

## Project overview
- **What**: Management app for an apotek (pharmacy) store.
- **Current phase**: Phase 3 shipped (POS / Sales: customers, sales, FEFO consumption, split-view cashier UI, today's-snapshot dashboard). Receipt rendering is on-screen only; thermal printer lands in Phase 7. Line/cart discounts deferred to a follow-up.
- **Next phase**: Phase 4 — Analytics.

## Roadmap

| # | Phase | Status |
|---|---|---|
| 0 | Skeleton (Go + ConnectRPC + Postgres + React/Chakra + Buf + goose + Makefile) | ✓ shipped |
| 1 | Auth + user/role management (JWT, proto-declared role guards) | ✓ shipped |
| 2 | Inventory foundation (suppliers, medicines + price-version table, batches, stock ledger) | ✓ shipped |
| — | Frontend foundation refactor (TanStack Query + RHF + Zod + i18n + Chakra `defaultSystem`) | ✓ shipped |
| — | Refresh tokens (rotation + reuse detection; access 1h, refresh 30d) | ✓ shipped |
| 3 | POS / Sales (customers, sales, FEFO consumption, split-view cashier UI, on-screen receipt) | ✓ shipped |
| 4 | Analytics (sales / inventory / margin via Recharts; live Postgres queries) | 🚧 next |
| 5 | Purchasing (purchase orders, supplier ledger, formal receive flow) | 📋 |
| 6 | Prescriptions (Rx validation tied to a Sale, customer-linked) | 📋 |
| 7 | Indonesia integrations (ESC/POS thermal, e-Faktur, BPJS Kesehatan) | 📋 |
| 8 | Multi-branch (promote `branch_id` from placeholder to first-class) | 📋 |
| 9 | Production hardening (password reset, rate limit, audit log, backups, deploy) | 📋 |

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
| Charts | Recharts (Phase 4 — not yet installed) |
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
│   └── pos_iface/v1/       # Sale, SaleItem, PaymentSource + SaleService (FEFO consumption)
├── buf.yaml                # buf v2 module config (root)
├── buf.gen.yaml            # buf v2 codegen config (root)
├── backend/                # Go service
│   ├── cmd/server/         # main API entrypoint
│   ├── cmd/migrate/        # goose wrapper (CWD-aware: uses backend/migrations)
│   ├── internal/
│   │   ├── auth/           # JWT issuer, password hash, ctx Principal, interceptor
│   │   ├── config/         # YAML loader (Server, Database, Auth, Bootstrap)
│   │   ├── db/             # GORM open + ping
│   │   ├── model/          # GORM models (User, …)
│   │   └── service/        # ConnectRPC service implementations (Auth, Users, Health)
│   ├── gen/                # GENERATED — do not hand-edit
│   └── migrations/         # goose .sql files
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
│       │   └── format.ts     # locale-aware money/date helpers
│       ├── stores/
│       │   └── preferences.ts # theme + locale + sidebar (Zustand, persisted)
│       ├── queries/        # one file per domain — TanStack hooks wrapping Connect clients
│       │   ├── users.ts
│       │   ├── suppliers.ts
│       │   ├── medicines.ts
│       │   ├── batches.ts
│       │   └── stock.ts
│       ├── locales/{en,id}.json
│       ├── components/
│       │   ├── AppShell.tsx          # sidebar + topbar wrapper
│       │   ├── Sidebar.tsx           # 3-state responsive sidebar
│       │   ├── TopBar.tsx            # lang/theme/user menu chrome
│       │   ├── PageHeader.tsx        # title + breadcrumbs + actions
│       │   ├── EntityDrawer.tsx      # slide-over for create/edit
│       │   ├── FormField.tsx         # RHF Controller + Chakra Field
│       │   ├── ErrorBoundary.tsx
│       │   └── ProtectedRoute.tsx
│       ├── routes/{Login,Dashboard,Users,Inventory}.tsx
│       ├── routes/inventory/{Medicines,Suppliers,Batches,Movements}.tsx
│       ├── App.tsx           # picks AppShell vs bare layout (login)
│       └── main.tsx          # ErrorBoundary > QueryClient > Chakra > Auth > Router + AppToaster
├── docker-compose.yml      # postgres:latest
├── config.example.yaml     # template; copy to config.yaml (gitignored)
├── config.yaml             # local runtime config (gitignored, lives at root)
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
- **goose owns the schema.** Migration files live in `backend/migrations/`.
- **GORM is query-only.** Do NOT use `AutoMigrate` — it causes drift with goose.
- DSN is built from `config.yaml` (`database.{host,port,user,password,name,sslmode}`).
- Make targets `migrate-up` / `migrate-down` read `config.yaml` and call goose.

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
make generate        # buf generate -> backend/gen + frontend/src/gen
make tidy            # go -C backend mod tidy
make migrate-up      # apply goose migrations
make migrate-down    # rollback one migration
make migrate-status  # show migration state
make migrate-create name=add_medicines_table   # new migration file
make run             # API server on :8080
make web-install     # npm install (frontend)
make web             # Vite dev server on :5173
```

## Conventions
- **New RPC method**: add it to a `.proto` file under `proto/<pkg>/v1/`, run `make generate`, implement the handler in `backend/internal/service/`, register it in `backend/cmd/server/main.go`.
- **New domain**: create a new `proto/<domain>_iface/v1/` folder and proto files; generated Go lands in `backend/gen/<domain>_iface/v1/` and TS in `frontend/src/gen/<domain>_iface/v1/`.
- **Frontend dev API calls**: hit `/api/...` — Vite proxies to `http://localhost:8080`.
- **Chakra UI v3**: use **`defaultSystem`** (no custom system, no custom palette, no custom semantic tokens). Compose with other components via `asChild` (e.g. `<ChakraLink asChild><RouterLink to="/">...</RouterLink></ChakraLink>`). Brand accent is expressed via `colorPalette="blue"` on interactive components; surface and text tokens (`bg`, `bg.subtle`, `bg.muted`, `fg`, `fg.muted`, `border`, `border.muted`) come straight from `defaultSystem`.
- **Module path**: `github.com/apotech/backend` (placeholder — rename after pushing to a real Git host).
- **Migrations dir**: `cmd/migrate/main.go` passes the literal string `"migrations"` to goose, resolved relative to the binary's CWD. The Makefile invokes the binary via `go -C backend run ./cmd/migrate ...`, so CWD = `backend/` and goose finds `backend/migrations/`. If you run it directly, do so from `backend/` (or change that string).
- **Postgres volume mount**: `docker-compose.yml` mounts `./data/postgres` at `/var/lib/postgresql` (NOT `/var/lib/postgresql/data`). Required for `postgres:18+`, which stores data in a version-suffixed subdirectory and refuses to start if you mount directly at `/data`. Do not "fix" the mount target back.

## Frontend conventions

### ChakraUI-first (HARD RULE)
**Before writing any custom JSX or hand-rolled UI primitive, search the Chakra v3 component catalog (https://chakra-ui.com) and use the built-in equivalent.** Chakra ships rich components for nearly every common need: `Field` (form fields), `Input`, `NativeSelect`/`Select`, `Switch`, `Checkbox`, `RadioGroup`, `Slider`, `Editable`, `Tabs`, `Table`, `Card`, `Tag`, `Badge`, `Avatar`, `Accordion`, `Tooltip`, `Popover`, `Menu`, `Drawer`, `Dialog`, `Toast`, `Steps`, `Progress`, `Stat`, `Skeleton`, `Spinner`, `Pagination`, `Pin Input`, `File Upload`, `Number Input`, `Date Picker`, `Combobox`, `Tree View`, etc. **Compose Chakra components rather than rebuilding them.** Custom JSX is only acceptable when the requirement is genuinely outside Chakra's catalog. This rule supersedes any urge to hand-roll a UI primitive.

### Visual identity & theme
- **Chakra `defaultSystem` only.** No custom system, no custom palette, no custom semantic tokens, no custom font. Brand accent uses the default `blue` palette via `colorPalette="blue"` on interactive components. Surface/text tokens (`bg`, `bg.subtle`, `bg.muted`, `fg`, `fg.muted`, `border`, `border.muted`) come straight from `defaultSystem` and flip light/dark automatically.
- **Typography**: default system font stack from Chakra `defaultSystem`. No Google Fonts.
- **Icons**: Lucide (`lucide-react`). Tree-shakable. Domain mapping (Pill = Medicines, Truck = Suppliers, Boxes = Batches, ArrowLeftRight = Movements, BarChart3 = Analytics, ShoppingCart = POS, Users = Users, LayoutDashboard = Dashboard).
- **Layout**: `AppShell` (sidebar + sticky top bar) wraps every authenticated route. Sidebar is 3-state responsive (expanded 240px on desktop, icon-only 64px when collapsed, slide-over drawer on mobile via the hamburger in `TopBar`). Sidebar items filtered by role: CASHIER sees Dashboard+POS, PHARMACIST adds Inventory+Analytics+Customers+Prescriptions, OWNER adds Users. Login route uses a bare layout (no shell).
- **Pages start with `<PageHeader>`**: breadcrumbs → title → optional description → right-aligned actions → hairline divider. Non-sticky.
- **Create/Edit uses `<EntityDrawer>`** (slide-over from right). Modal-confirm reserved for destructive actions (future).
- **Data fetching = TanStack Query**: each domain has `src/queries/<domain>.ts` exporting `useXxxQuery` and `useXxxMutation` hooks that wrap the Connect client from [lib/clients.ts](frontend/src/lib/clients.ts). Query keys are tuples (`["medicines", "list", { includeInactive }]`). Mutations call `invalidateQueries` on the relevant list keys. Defaults: `staleTime: 30_000`, `refetchOnWindowFocus: false`, `retry: 1`. Don't fetch with raw `useEffect` + `setState`.
- **Forms = React Hook Form + Zod**: schema-first. Each form file declares a `z.object({...})`, derives `type FormValues = z.infer<typeof Schema>`, uses `useForm({ resolver: zodResolver(Schema) })`, and renders fields via the shared `<FormField>` component (wraps Chakra `Field.Root` + RHF `Controller`). No manual `setError`/`setBusy` boilerplate.
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
- **Frontend tokens**: stored in `localStorage["apotech_access_token"]` and `localStorage["apotech_refresh_token"]`. Only `AuthProvider` ([lib/auth.tsx](frontend/src/lib/auth.tsx)) and the transport interceptor ([lib/transport.ts](frontend/src/lib/transport.ts)) touch them; everything else uses `useAuth()`. `logout()` is async — calls `AuthService.Logout` first (best-effort), then clears both keys.
- **Route protection**: `<ProtectedRoute>` (with optional `requiredRole`) wraps router children. UI gating only — the backend `auth.NewInterceptor` is the real enforcement.

## Sales model (Phase 3)
- **Customers**: light table (`name`, `phone`, `bpjs_no`, `notes`, `active`). Sales may attach a customer or stay anonymous. CASHIER+PHARMACIST+OWNER can list/get/search/create; only OWNER+PHARMACIST can update/archive. `bpjs_no` column reserved for the BPJS integration phase.
- **Sale state machine**: `DRAFT → COMPLETED` or `DRAFT → VOIDED`. `ON_HOLD` is intentionally not modeled. Only DRAFT sales accept item/customer mutations.
- **Sale numbering**: human-friendly per-year format `INV-2026-0001`. A `sale_no_counters(year, last_seq)` row is incremented inside `CompleteSale`'s tx; the resulting `sale_no` is unique. UUID `id` is the primary key; `sale_no` is for human reference.
- **Money snapshots**: `sale_items.unit_price_snapshot` captures the price at the moment the line was added (via `AddItem`). Historical reporting therefore survives `medicines.unit_price` edits made after the sale. `line_discount` and `cart_discount` columns are placeholders for the upcoming discount-service slice; currently always 0.
- **FEFO consumption rule** lives inside `CompleteSale` (see [service/sales.go](backend/internal/service/sales.go)). For each cart line: load batches for that medicine ordered by `expiry_date ASC`, compute current per-batch stock from `stock_movements`, allocate greedily. If a line spans multiple batches, the placeholder `sale_items` row is deleted and one new row per consumed batch is inserted — each with its `batch_id` pinned and `stock_movements(type=SALE, qty=-N, sale_item_id=<that row>)` linked. This keeps the audit chain bidirectional: every SALE movement has a sale_item, every sale_item has a batch.
- **Insufficient stock**: if any line can't be fully allocated, the whole `CompleteSale` tx rolls back with `FailedPrecondition`. The user retries after adjusting qty or restocking.
- **`branch_id` placeholders**: added (nullable) to `sales`, `sale_items`, and `stock_movements` — all reserved for the multi-branch phase.
- **Today's snapshot**: `SaleService.GetTodaySnapshot` aggregates revenue / sale count / items sold / top medicine for COMPLETED sales since `00:00 server-local`. The Dashboard page calls it and renders three tiles. Polls on each visit (`staleTime: 30s`).
- **POS UI**: route `/pos`, available to CASHIER + PHARMACIST + OWNER. Opts out of `AppShell` — full-screen via the bare layout branch in [App.tsx](frontend/src/App.tsx). Split view: medicine search (~60%, auto-focused, barcode-scanner friendly — SKU-exact-match-on-Enter auto-adds), cart panel (~40%, qty inline-editable, payment radio, change calc). Keyboard shortcuts: **F2** search · **F4** customer · **F8** complete · **Esc** clear. Cart state lives in the backend `sales` row (DRAFT); the UI fetches/mutates via TanStack Query (no separate cart store).
- **Receipt**: on-screen Chakra Dialog after CompleteSale. ESC/POS thermal printer wiring deferred to Phase 7.

## Inventory model
- **Money**: `unit_price` (medicines) and `cost_price` (batches) are `BIGINT` storing the **minor currency unit**. For IDR (no subdivision) that's whole rupiah. Never floats. Frontend formats with `Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' })`.
- **Stock ledger**: `stock_movements` is **insert-only**. Current batch stock = `SUM(qty) WHERE batch_id = $1`. No mutable counter on `batches`. Movement types: `PURCHASE` (inserted automatically by `CreateBatch` for `initial_quantity`), `SALE` (will come from POS in the next phase), `ADJUSTMENT`, `WRITE_OFF` (manual via `RecordMovement`). The handler refuses any movement that would drive a batch's stock below zero (post-insert check inside the tx; rollback on violation).
- **Pricing version**: `medicine_prices` is the authoritative price history. Exactly one row per medicine has `effective_to IS NULL` at any time — enforced by a partial unique index (`medicine_prices_open_idx`). `medicines.unit_price` is a denormalized "current price" cache. **All price writes go through `MedicineService.UpdateMedicine`'s tx**, which (a) closes the current open row, (b) inserts a new open row, (c) updates the cache, all in one GORM transaction. Direct SQL writes to `medicines.unit_price` would diverge and must be avoided.
- **FEFO consumption**: not enforced yet. `ListBatches` sorts by `expiry_date ASC` so the UI surfaces soon-to-expire first. The actual "which batch to consume when selling" rule will land with POS.
- **Soft delete**: catalog entities use `active = false` rather than DELETE; rows referenced by movements or price history must persist.

## Known gaps (not yet implemented)
- No rate limiting / brute-force protection on `Login`.
- No password reset / forgot-password flow.
- No audit log of admin actions (role changes, deactivations, price changes other than the dedicated `medicine_prices` history).
- No discounts in POS (line and cart discounts are schema-ready but the `DiscountService` proto + UI controls are deferred).
- No returns / refunds flow.
- No stocktake / reconciliation workflow.
- No barcode scanning hardware wiring (POS search input is scanner-friendly via SKU-exact-Enter, but no HID/serial layer).
- No ESC/POS thermal-printer rendering (Phase 7).
- Admin "force logout user" RPC (data model supports it via `refresh_tokens.user_id`; ship later).

## Out of scope (for the current phase)
Do not invent these without an explicit user request:
- Discount controls in POS (line + cart discounts)
- Prescription handling
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

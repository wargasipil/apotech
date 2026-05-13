# CLAUDE.md — Apotech Project Memory

Living doc for Claude. Update this file whenever the stack, layout, or conventions change (new service, new dep, new top-level dir).

## Project overview
- **What**: Management app for an apotek (pharmacy) store.
- **Current phase**: Phase 2 — Inventory foundation: suppliers, medicines (with full price-version table), batches/expiry, stock-movement ledger. Sits behind the Phase 1 auth gate.
- **Next phase**: POS / sales screen (cashier rings up sales, FEFO consumption rule fires, stock decrements via `RecordMovement(type=SALE)`).

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
│   ├── user_iface/v1/      # User + UserService + AuthService (login/me)
│   └── inventory_iface/v1/ # Supplier, Medicine (+ MedicinePrice), Batch, StockMovement
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
- **Login**: `AuthService.Login` returns a JWT (HS256, signed with `cfg.Auth.JWTSecret`). Client puts it in `Authorization: Bearer <jwt>`.
- **Policy is declared in proto, not Go.** Each rpc carries one of:
  - `option (auth_iface.v1.public) = true;` — no auth (Login, Ping)
  - `option (auth_iface.v1.allowed_roles) = ROLE_xxx;` (repeated for multi-role) — authn + role-in-set
  - *no option* — authn only (e.g. `Me`, `ChangePassword`)
- **`auth.BuildPolicy()`** ([policy.go](backend/internal/auth/policy.go)) walks `protoregistry.GlobalFiles` at boot and produces `map[procedure]Policy`. Called once in `main.go` and passed to the interceptor.
- **`auth.NewInterceptor`** ([interceptor.go](backend/internal/auth/interceptor.go)) is the **single auth gate**. It (a) skips public procedures, (b) parses the JWT, (c) enforces `AllowedRoles`, (d) injects `auth.Principal{UserID, Role}` into the request context.
- **Handlers must NOT call role-check helpers.** They use `auth.MustPrincipal(ctx)` only for row-level decisions (e.g. `ChangePassword` self-vs-other). Adding a role check inside a handler is a code smell — the policy belongs in the proto.
- **Default policy**: a new RPC with no annotation is authenticated-only. To open it up you must explicitly mark `public = true`. Safe failure mode for forgotten annotations.
- **Bootstrap owner**: on every boot, `Users.EnsureBootstrapOwner` upserts the user described by `cfg.Bootstrap.owner_email/owner_password`. Empty email → skip. Changing the password in `config.yaml` rotates it on next restart.
- **Frontend token**: stored in `localStorage["apotech_token"]`. Only `AuthProvider` ([lib/auth.tsx](frontend/src/lib/auth.tsx)) touches it; everything else uses `useAuth()`. The Connect transport interceptor ([lib/transport.ts](frontend/src/lib/transport.ts)) reads it fresh on each request, so logout takes effect without recreating the transport.
- **Route protection**: `<ProtectedRoute>` (with optional `requiredRole`) wraps router children. UI gating only — the backend `auth.NewInterceptor` is the real enforcement.

## Inventory model
- **Money**: `unit_price` (medicines) and `cost_price` (batches) are `BIGINT` storing the **minor currency unit**. For IDR (no subdivision) that's whole rupiah. Never floats. Frontend formats with `Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' })`.
- **Stock ledger**: `stock_movements` is **insert-only**. Current batch stock = `SUM(qty) WHERE batch_id = $1`. No mutable counter on `batches`. Movement types: `PURCHASE` (inserted automatically by `CreateBatch` for `initial_quantity`), `SALE` (will come from POS in the next phase), `ADJUSTMENT`, `WRITE_OFF` (manual via `RecordMovement`). The handler refuses any movement that would drive a batch's stock below zero (post-insert check inside the tx; rollback on violation).
- **Pricing version**: `medicine_prices` is the authoritative price history. Exactly one row per medicine has `effective_to IS NULL` at any time — enforced by a partial unique index (`medicine_prices_open_idx`). `medicines.unit_price` is a denormalized "current price" cache. **All price writes go through `MedicineService.UpdateMedicine`'s tx**, which (a) closes the current open row, (b) inserts a new open row, (c) updates the cache, all in one GORM transaction. Direct SQL writes to `medicines.unit_price` would diverge and must be avoided.
- **FEFO consumption**: not enforced yet. `ListBatches` sorts by `expiry_date ASC` so the UI surfaces soon-to-expire first. The actual "which batch to consume when selling" rule will land with POS.
- **Soft delete**: catalog entities use `active = false` rather than DELETE; rows referenced by movements or price history must persist.

## Known gaps (not yet implemented)
- No rate limiting / brute-force protection on `Login`.
- No password reset / forgot-password flow.
- No refresh tokens — clients must re-login when the JWT expires (`cfg.Auth.token_ttl`).
- No audit log of admin actions (role changes, deactivations, price changes other than the dedicated `medicine_prices` history).
- No FEFO consumption rule (deferred to POS phase).
- No stocktake / reconciliation workflow.
- No barcode scanning.

## Out of scope (for the current phase)
Do not invent these without an explicit user request:
- POS / sales screen (next phase — plugs into `GetStockLevels` + `RecordMovement(type=SALE)`)
- Prescription handling
- Reports / dashboards
- Multi-tenant / multi-branch
- Production deployment, CI, Dockerfiles for app code

## Update policy
Update this file when any of the following changes:
- A new top-level directory is added.
- A dependency is added, removed, or replaced.
- A convention is introduced or changed.
- A new service/proto package is added.
- The "current phase" or scope changes.

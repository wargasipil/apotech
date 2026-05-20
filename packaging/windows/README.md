# Apotech — Windows installer

Builds a single `ApotechSetup-<version>.exe` that installs a fully self-contained
Apotech on a Windows PC: the app server, a **bundled PostgreSQL**, both
registered as auto-start Windows Services, plus a desktop/Start-menu shortcut.
No prerequisites for the pharmacist.

## What's in the box
| Piece | How it runs |
|---|---|
| `apotech.exe` | Windows Service `apotech-server` (via **WinSW**). Serves UI + `/api` on one port; auto-migrates on boot. |
| Bundled PostgreSQL (`pgsql\`) | Native Windows Service `apotech-postgres` (via `pg_ctl register`, runs as `NetworkService`, bound to `127.0.0.1`). |
| `config.yaml`, `pgdata\`, `backups\`, `logs\` | Under `C:\ProgramData\Apotech\`. App + program files under `C:\Program Files\Apotech\`. |

## Build prerequisites (developer machine, Windows x64)
- **Go 1.25+** and **Node 20+** (to compile `apotech.exe` with the SPA embedded).
- **Inno Setup 6** — `ISCC.exe` (https://jrsoftware.org/isdl.php).
- Internet on first run (downloads bundled PostgreSQL + WinSW; cached in `.cache\`).

## Build
```powershell
# from the repo root
powershell -ExecutionPolicy Bypass -File packaging\windows\build-windows.ps1 -AppVersion 0.1.0
# or: make installer   (runs dist-windows first, then this script)
```
Output: `dist\ApotechSetup-0.1.0.exe`.

Pin versions with `-PgVersion 16.4-1` / `-WinswVersion 2.12.0`. Use the **same**
PostgreSQL major as the Docker image (`postgres:18` in `docker-compose.prod.yml`)
once you settle on one, so behavior matches across flavors.

## Install (target PC)
Double-click `ApotechSetup-*.exe`. The wizard asks for:
1. **Owner email + password** — the first OWNER login (change after first login).
2. **Network access** — *This computer only* (`127.0.0.1`) or *Other computers on
   the shop network* (`0.0.0.0` + a Windows Firewall rule).
3. **Ports** — app (default `8080`) and database (default `5433`, to avoid
   clashing with an existing PostgreSQL on `5432`).

Post-install (`setup.ps1`) initializes the DB, writes `config.yaml` (random
`jwt_secret` + DB password), registers + starts both services, adds the firewall
rule in LAN mode, drops shortcuts, and opens the browser.

## Verify on a clean Windows VM (acceptance)
1. Run the installer → finishes without errors; `services.msc` shows
   **apotech-postgres** and **apotech-server** *Running*.
2. The shortcut opens `http://localhost:<port>`; log in as the owner; create a sale.
3. **Reboot** → both services auto-start; data persists.
4. **LAN mode**: from a second PC, browse `http://<server-ip>:<port>`; confirm the
   firewall rule (`netsh advfirewall firewall show rule name="Apotech Server"`).
5. **Single-PC mode**: the port is not reachable from another machine.
6. Backup: run `scripts\apotech-backup.bat` → a `.sql` lands in `backups\`.
7. Uninstall → services removed; choose whether to keep or drop the data dir.

## Known caveats
- **Unsigned** installer + exe → SmartScreen "unknown publisher". Code-sign with
  an Authenticode cert for production distribution.
- The **PostgreSQL service account + `pgdata` ACLs** (`NetworkService`) are the
  most likely thing to need tuning on locked-down/domain-joined hosts.
- Config edits (host/ports) take effect after `Restart-Service apotech-server`.

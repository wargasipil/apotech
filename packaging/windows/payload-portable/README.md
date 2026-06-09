# Apotech Portable (Windows, SQLite)

The portable flavor is a single-PC, USB-stick-friendly variant of Apotech:

* **One self-contained binary** (`apotech-portable.exe`, ~15 MB after embed).
* **Embedded SQLite** — no external database, no service, no firewall, no admin.
* **One file = your database**: `data\apotech.db` next to the binary.
* **Auto-opens the browser** to `http://127.0.0.1:8080` on launch.

> Need a multi-PC / shop-network install (Postgres + WinSW service)?
> Use the full **Apotech** installer (`ApotechSetup-*.exe`) instead.

## Install

1. Run `ApotechPortableSetup-<version>.exe`.
2. Choose a folder (per-user install, no admin required).
3. Enter the **owner email + password** (this is your first login).
4. Default port `8080` is fine unless another program already uses it.
5. Finish — the launcher opens your browser to `http://127.0.0.1:8080`.

## Daily use

* Start menu → **Apotech Portable** to launch.
* The optional **Run on Windows startup** tickbox adds a per-user Run entry —
  *not* a Windows service.
* All your data stays inside the install folder: copying it (or backing it up
  via the script below) is sufficient for off-site backup.

## Backups

* The in-app **Settings → Backups** page works the same as the full flavor.
  Each backup is a per-timestamp directory:
  ```
  backups\backup_2026-06-09_153012\
    database.db    (consistent SQLite snapshot via VACUUM INTO)
    manifest.txt   (created_at + app/db version + schema_version + size_bytes)
  ```
* For scheduled backups, wire **Task Scheduler** to
  `apotech-portable-backup.bat` (in the install folder). It runs
  `apotech-portable.exe --backup` and writes one snapshot.

## Restore

1. Stop Apotech Portable (close the EXE — there's no service).
2. Replace `data\apotech.db` with the chosen backup's `database.db`.
3. Start Apotech Portable again.

## Uninstall

The uninstaller leaves `data\` and `backups\` in place by default — you can
keep them for a future reinstall. Delete the install folder manually if you
want a clean wipe.

## Notes

* The portable flavor uses `modernc.org/sqlite` (pure-Go, no CGO) — same
  data file is readable by any standard SQLite tool (e.g. **DB Browser for
  SQLite**).
* Indonesian e-Faktur (NSFP) integration and ESC/POS thermal printing both
  work; the latter still needs a network printer reachable from this PC.
* The portable build serializes writes (SQLite single-writer) — fine for a
  single-shop pharmacy.

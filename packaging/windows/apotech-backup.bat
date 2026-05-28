@echo off
REM Apotech database backup. Writes a plain SQL dump (Windows ships no gzip)
REM into C:\ProgramData\Apotech\backups\backup_YYYYMMDD_HHMMSS\database.sql
REM plus a manifest.txt next to it, using the bundled pg_dump. Same per-
REM timestamp directory layout as BackupService / `make backup` so the in-app
REM list and the CLI list see the same backups. Schedule via Task Scheduler
REM for nightly backups.
REM
REM Reads the DB port + password from config.yaml so credentials stay in one
REM place.

setlocal
set "APPDIR=%ProgramFiles%\Apotech"
set "DATADIR=%ProgramData%\Apotech"
set "PGBIN=%APPDIR%\pgsql\bin"
set "BACKUPS=%DATADIR%\backups"
if not exist "%BACKUPS%" mkdir "%BACKUPS%"

REM Pull port + password out of config.yaml (simple line scrape).
for /f "tokens=2 delims=:" %%P in ('findstr /b /c:"  port:" "%DATADIR%\config.yaml"') do set "PGPORT=%%P"
for /f "tokens=2 delims=:" %%P in ('findstr /c:"  password:" "%DATADIR%\config.yaml"') do set "PGPASSWORD=%%P"
set "PGPORT=%PGPORT: =%"
set "PGPASSWORD=%PGPASSWORD: =%"

for /f "tokens=2 delims==" %%I in ('wmic os get localdatetime /value') do set "DT=%%I"
set "STAMP=%DT:~0,8%_%DT:~8,6%"

set "DUMPDIR=%BACKUPS%\backup_%STAMP%"
mkdir "%DUMPDIR%"
set "DUMPFILE=%DUMPDIR%\database.sql"

"%PGBIN%\pg_dump.exe" -h 127.0.0.1 -p %PGPORT% -U apotech --no-password --clean --if-exists apotech > "%DUMPFILE%"
if errorlevel 1 (
  echo pg_dump failed; removing partial backup directory.
  rmdir /s /q "%DUMPDIR%"
  exit /b 1
)

REM Read the goose schema version for the manifest (best effort).
set "PGVER=0"
for /f %%V in ('"%PGBIN%\psql.exe" -h 127.0.0.1 -p %PGPORT% -U apotech -d apotech -tA -c "SELECT COALESCE(MAX(version_id),0) FROM goose_db_version WHERE is_applied"') do set "PGVER=%%V"

REM Manifest (ASCII; mirrors BackupService writeManifest output).
for %%A in ("%DUMPFILE%") do set "SIZE=%%~zA"
> "%DUMPDIR%\manifest.txt" (
  echo created_at_iso=%DT:~0,4%-%DT:~4,2%-%DT:~6,2%T%DT:~8,2%:%DT:~10,2%:%DT:~12,2%
  echo app_version=dev
  echo schema_version=%PGVER%
  echo size_bytes=%SIZE%
)

echo Wrote %DUMPDIR%\
endlocal

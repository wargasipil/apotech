@echo off
REM Apotech Portable backup script.
REM Invokes the EXE in --backup mode, which runs one CreateBackup via the
REM in-process service (no Postgres, no pg_dump) and exits. The result lands
REM in <AppDir>\backups\backup_YYYY-MM-DD_HHMMSS\database.db + manifest.txt.
REM
REM Wire to Task Scheduler for nightly backups.

setlocal
set "HERE=%~dp0"
"%HERE%apotech-portable.exe" --backup
if errorlevel 1 (
  echo Backup failed with exit code %errorlevel%.
  exit /b %errorlevel%
)
echo Backup completed.
endlocal

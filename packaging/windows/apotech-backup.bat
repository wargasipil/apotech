@echo off
REM Apotech database backup. Writes a gzip-less plain SQL dump (Windows ships no
REM gzip) to C:\ProgramData\Apotech\backups\apotech_YYYYMMDD_HHMMSS.sql using the
REM bundled pg_dump. Schedule via Task Scheduler for nightly backups.
REM
REM Reads the DB password from config.yaml so it stays in one place.

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

"%PGBIN%\pg_dump.exe" -h 127.0.0.1 -p %PGPORT% -U apotech apotech > "%BACKUPS%\apotech_%STAMP%.sql"
echo Wrote %BACKUPS%\apotech_%STAMP%.sql
endlocal

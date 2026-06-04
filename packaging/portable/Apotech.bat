@echo off
REM Apotech portable launcher. Double-click to start the pharmacy app.
REM Runs entirely from this folder (SQLite, no install, no admin).
cd /d "%~dp0"
powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0launch.ps1"
echo.
echo Apotech has stopped. Press any key to close this window.
pause >nul

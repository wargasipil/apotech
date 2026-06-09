@echo off
REM Apotech Portable - one-time setup.
REM Generates a fresh JWT secret, prompts for the owner email/password,
REM and writes config.yaml next to this script. Refuses to overwrite an
REM existing config.yaml (delete it first if you want to reinitialize).
REM
REM Stays ASCII; Windows PowerShell 5.1 reads -File as Windows-1252.

setlocal
set "HERE=%~dp0"
if "%HERE:~-1%"=="\" set "HERE=%HERE:~0,-1%"

if exist "%HERE%\config.yaml" (
  echo.
  echo config.yaml already exists at "%HERE%\config.yaml".
  echo Delete it first if you want to reinitialize.
  echo.
  pause
  exit /b 1
)

echo.
echo === Apotech Portable - first-run setup ===
echo.

powershell -NoProfile -ExecutionPolicy Bypass -Command ^
  "$here = '%HERE%';" ^
  "while ($true) { $email = Read-Host 'Owner email'; if ([string]::IsNullOrWhiteSpace($email)) { Write-Host 'Email cannot be empty.' -ForegroundColor Yellow; continue }; break };" ^
  "while ($true) { $sec = Read-Host 'Owner password (min 8 chars)' -AsSecureString; $b = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec); $pw = [Runtime.InteropServices.Marshal]::PtrToStringAuto($b); [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($b) | Out-Null; if ($pw.Length -lt 8) { Write-Host 'Password must be at least 8 characters.' -ForegroundColor Yellow; continue }; break };" ^
  "$bytes = New-Object byte[] 32; [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes); $jwt = ($bytes | ForEach-Object { $_.ToString('x2') }) -join '';" ^
  "$dbPath = ($here + '\data\apotech.db') -replace '\\','/';" ^
  "$bkDir = $here -replace '\\','/';" ^
  "$lines = @('server:','  host: 127.0.0.1','  port: 8080','  open_browser_on_start: true','','database:','  driver: sqlite','  path: ' + $dbPath,'  auto_migrate: true','','auth:','  jwt_secret: ' + $jwt,'  access_token_ttl: 1h','  refresh_token_ttl: 720h','','bootstrap:','  owner_email: ' + $email.Trim(),'  owner_password: ' + $pw,'','backup:','  directory: ' + $bkDir + '/backups','');" ^
  "Set-Content -Path ($here + '\config.yaml') -Value ($lines -join \"`r`n\") -Encoding utf8 -NoNewline;" ^
  "New-Item -ItemType Directory -Force -Path ($here + '\data') | Out-Null;" ^
  "New-Item -ItemType Directory -Force -Path ($here + '\backups') | Out-Null;" ^
  "Write-Host '';" ^
  "Write-Host 'config.yaml written. Owner email:' $email.Trim();"

if errorlevel 1 (
  echo.
  echo Setup failed. See the message above.
  echo.
  pause
  exit /b 1
)

echo.
echo Done. Double-click apotech-portable.exe to start.
echo The browser will open automatically at http://127.0.0.1:8080
echo.
pause
endlocal

<#
  Assembles the Windows installer payload and runs Inno Setup.

  Prerequisites on the build machine:
    - Go 1.25+ and Node 20+ (to build apotech.exe), unless -SkipExeBuild and a
      prebuilt dist\apotech.exe already exists.
    - Inno Setup 6 (ISCC.exe on PATH or in the default Program Files location).
    - Internet access on first run (downloads bundled PostgreSQL + WinSW; cached).

  Output: dist\ApotechSetup-<version>.exe

  Usage:
    powershell -ExecutionPolicy Bypass -File packaging\windows\build-windows.ps1 `
      -AppVersion 0.1.0
#>
[CmdletBinding()]
param(
  [string] $AppVersion = "0.1.0",
  [string] $PgVersion  = "16.4-1",        # EDB Windows binary zip version. Keep in sync with backend/internal/service/pgdump.go const PgToolsVersion.
  [string] $WinswVersion = "2.12.0",
  [switch] $SkipExeBuild
)

$ErrorActionPreference = "Stop"
$here     = $PSScriptRoot
$root     = (Resolve-Path (Join-Path $here "..\..")).Path
$dist     = Join-Path $root "dist"
$payload  = Join-Path $here "payload"
$cache    = Join-Path $here ".cache"
$embedDir = Join-Path $root "backend\internal\web\dist"

New-Item -ItemType Directory -Force -Path $dist,$cache | Out-Null

# --- 1. Build apotech.exe (SPA + migrations embedded) ------------------------
$exe = Join-Path $dist "apotech.exe"
if (-not $SkipExeBuild -or -not (Test-Path $exe)) {
  Write-Host "Building frontend + Windows binary..."
  Push-Location (Join-Path $root "frontend")
  npm ci
  npm run build
  Pop-Location
  Remove-Item -Recurse -Force (Join-Path $embedDir "assets") -ErrorAction SilentlyContinue
  Copy-Item -Recurse -Force (Join-Path $root "frontend\dist\*") $embedDir
  Push-Location (Join-Path $root "backend")
  $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
  go build -ldflags "-s -w" -o $exe ./cmd/server
  Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED
  Pop-Location
}

# --- 2. Assemble payload -----------------------------------------------------
Write-Host "Assembling payload..."
Remove-Item -Recurse -Force $payload -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path `
  $payload, (Join-Path $payload "winsw"), (Join-Path $payload "scripts") | Out-Null

Copy-Item $exe (Join-Path $payload "apotech.exe")

# WinSW (service wrapper for the app)
$winsw = Join-Path $cache "WinSW-x64-$WinswVersion.exe"
if (-not (Test-Path $winsw)) {
  $u = "https://github.com/winsw/winsw/releases/download/v$WinswVersion/WinSW-x64.exe"
  Write-Host "Downloading WinSW $WinswVersion..."
  Invoke-WebRequest -Uri $u -OutFile $winsw
}
Copy-Item $winsw (Join-Path $payload "winsw\apotech-server.exe")
Copy-Item (Join-Path $here "apotech-server.xml") (Join-Path $payload "winsw\apotech-server.xml")

# PostgreSQL Windows binaries (EDB zip -> top-level pgsql\)
$pgZip = Join-Path $cache "postgresql-$PgVersion-windows-x64-binaries.zip"
if (-not (Test-Path $pgZip)) {
  $u = "https://get.enterprisedb.com/postgresql/postgresql-$PgVersion-windows-x64-binaries.zip"
  Write-Host "Downloading PostgreSQL $PgVersion (large)..."
  Invoke-WebRequest -Uri $u -OutFile $pgZip
}
$pgExtract = Join-Path $cache "pg-$PgVersion"
if (-not (Test-Path (Join-Path $pgExtract "pgsql\bin\postgres.exe"))) {
  Remove-Item -Recurse -Force $pgExtract -ErrorAction SilentlyContinue
  Expand-Archive -Path $pgZip -DestinationPath $pgExtract -Force
}
# Ship only what the server + tooling need (skip docs/symbols to shrink size).
Copy-Item -Recurse -Force (Join-Path $pgExtract "pgsql") (Join-Path $payload "pgsql")

# Setup/teardown/backup scripts
Copy-Item (Join-Path $here "setup.ps1")          (Join-Path $payload "scripts\setup.ps1")
Copy-Item (Join-Path $here "uninstall.ps1")      (Join-Path $payload "scripts\uninstall.ps1")
Copy-Item (Join-Path $here "apotech-backup.bat") (Join-Path $payload "scripts\apotech-backup.bat")

# --- 3. Run Inno Setup -------------------------------------------------------
$iscc = (Get-Command iscc.exe -ErrorAction SilentlyContinue).Source
if (-not $iscc) {
  foreach ($p in @(
      "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
      "$env:ProgramFiles\Inno Setup 6\ISCC.exe")) {
    if (Test-Path $p) { $iscc = $p; break }
  }
}
if (-not $iscc) { throw "Inno Setup (ISCC.exe) not found. Install Inno Setup 6." }

Write-Host "Compiling installer with $iscc ..."
& $iscc "/DAppVersion=$AppVersion" (Join-Path $here "apotech.iss")
Write-Host "Done -> $dist\ApotechSetup-$AppVersion.exe"

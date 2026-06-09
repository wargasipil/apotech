<#
  Assembles the Windows PORTABLE installer payload and runs Inno Setup.

  Differs from build-windows.ps1:
    * Builds apotech-portable.exe (with the `sqlite` build tag) instead of
      apotech.exe — the binary embeds SQLite (no Postgres).
    * NO PostgreSQL EDB zip download.
    * NO WinSW service wrapper.
    * Tiny payload (apotech-portable.exe + config + .bat + a setup script).

  Prerequisites:
    * Go 1.25+ and Node 20+ (unless -SkipExeBuild and a prebuilt
      dist\apotech-portable.exe already exists).
    * Inno Setup 6 (ISCC.exe on PATH or in the default Program Files location).

  Output: dist\ApotechPortableSetup-<version>.exe

  Usage:
    powershell -ExecutionPolicy Bypass -File packaging\windows\build-windows-portable.ps1 `
      -AppVersion 0.1.0
#>
[CmdletBinding()]
param(
  [string] $AppVersion = "0.1.0",
  [switch] $SkipExeBuild
)

$ErrorActionPreference = "Stop"
$here     = $PSScriptRoot
$root     = (Resolve-Path (Join-Path $here "..\..")).Path
$dist     = Join-Path $root "dist"
$payload  = Join-Path $here "payload-portable"
$embedDir = Join-Path $root "backend\internal\web\dist"

New-Item -ItemType Directory -Force -Path $dist | Out-Null

# --- 1. Build apotech-portable.exe (SPA + SQLite migrations embedded) --------
$exe = Join-Path $dist "apotech-portable.exe"
if (-not $SkipExeBuild -or -not (Test-Path $exe)) {
  Write-Host "Building frontend + Windows binary (sqlite tag)..."
  Push-Location (Join-Path $root "frontend")
  npm ci
  npm run build
  Pop-Location
  Remove-Item -Recurse -Force (Join-Path $embedDir "assets") -ErrorAction SilentlyContinue
  Copy-Item -Recurse -Force (Join-Path $root "frontend\dist\*") $embedDir
  Push-Location (Join-Path $root "backend")
  $env:GOOS = "windows"; $env:GOARCH = "amd64"; $env:CGO_ENABLED = "0"
  go build -tags sqlite -ldflags "-s -w" -o $exe ./cmd/server
  Remove-Item Env:\GOOS, Env:\GOARCH, Env:\CGO_ENABLED
  Pop-Location
}

# --- 2. Assemble payload -----------------------------------------------------
Write-Host "Assembling portable payload..."
Remove-Item -Recurse -Force $payload -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $payload, (Join-Path $payload "scripts") | Out-Null

Copy-Item $exe (Join-Path $payload "apotech-portable.exe")
Copy-Item (Join-Path $here "apotech-portable-backup.bat") (Join-Path $payload "apotech-portable-backup.bat")
Copy-Item (Join-Path $here "setup-portable.ps1")          (Join-Path $payload "scripts\setup-portable.ps1")
Copy-Item (Join-Path $here "README-portable.md")          (Join-Path $payload "README.md")

# --- 3. Run Inno Setup -------------------------------------------------------
$iscc = (Get-Command iscc.exe -ErrorAction SilentlyContinue).Source
if (-not $iscc) {
  foreach ($p in @(
      "${env:ProgramFiles(x86)}\Inno Setup 6\ISCC.exe",
      "$env:ProgramFiles\Inno Setup 6\ISCC.exe",
      "$env:USERPROFILE\scoop\apps\innosetup-np\current\ISCC.exe")) {
    if (Test-Path $p) { $iscc = $p; break }
  }
}
if (-not $iscc) { throw "Inno Setup (ISCC.exe) not found. Install Inno Setup 6 (e.g. scoop install innosetup-np)." }

Write-Host "Compiling portable installer with $iscc ..."
& $iscc "/DAppVersion=$AppVersion" (Join-Path $here "apotech-portable.iss")
Write-Host "Done -> $dist\ApotechPortableSetup-$AppVersion.exe"

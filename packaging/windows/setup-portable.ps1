<#
  Post-install bootstrap for the portable (SQLite) flavor.

  Writes {AppDir}\config.yaml with:
    * driver: sqlite, path: {AppDir}\data\apotech.db
    * a freshly generated 64-byte hex JWT secret
    * the wizard's owner email/password (bootstrap.owner_*)
    * server.host=127.0.0.1, server.port={AppPort}
    * server.open_browser_on_start=true (auto-opens 127.0.0.1:{AppPort})

  No services are registered, no firewall rules are added. The EXE is launched
  from a Start Menu shortcut (or the optional Run-on-startup task).

  This script is invoked by apotech-portable.iss [Run] step. Stays ASCII so
  Windows PowerShell 5.1 (-File mode) parses it correctly.
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory=$true)] [string] $AppDir,
  [Parameter(Mandatory=$true)] [string] $OwnerEmail,
  [Parameter(Mandatory=$true)] [string] $OwnerPassword,
  [string] $AppPort = "8080"
)

$ErrorActionPreference = "Stop"

# --- Generate a JWT secret (64 hex chars = 256 bits) ------------------------
$bytes = New-Object byte[] 32
[System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
$jwtSecret = ($bytes | ForEach-Object { $_.ToString("x2") }) -join ""

# --- Compose config.yaml ----------------------------------------------------
$dbPath = (Join-Path $AppDir "data\apotech.db") -replace '\\','/'
$lines = @(
  "server:",
  "  host: 127.0.0.1",
  "  port: $AppPort",
  "  open_browser_on_start: true",
  "",
  "database:",
  "  driver: sqlite",
  "  path: $dbPath",
  "  auto_migrate: true",
  "",
  "auth:",
  "  jwt_secret: $jwtSecret",
  "  access_token_ttl: 1h",
  "  refresh_token_ttl: 720h",
  "",
  "bootstrap:",
  "  owner_email: $OwnerEmail",
  "  owner_password: $OwnerPassword",
  "",
  "backup:",
  "  directory: $($AppDir -replace '\\','/')/backups",
  ""
)

$configPath = Join-Path $AppDir "config.yaml"
Set-Content -Path $configPath -Value ($lines -join "`r`n") -Encoding utf8 -NoNewline

# --- Make sure data\ exists so the first launch can create the DB ----------
New-Item -ItemType Directory -Force -Path (Join-Path $AppDir "data") | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $AppDir "backups") | Out-Null

Write-Output "Apotech Portable initialized at $AppDir"

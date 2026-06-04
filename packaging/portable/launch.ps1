# Apotech portable launcher (SQLite, zero-install).
#
# Double-clicked via Apotech.bat. Runs the self-contained apotech.exe out of the
# folder it lives in, with all data under .\data\ so the whole folder is movable
# (USB stick, copy elsewhere). No PostgreSQL, no Windows service, no admin.
#
# ASCII-ONLY: Windows PowerShell 5.1 reads -File as Windows-1252, so non-ASCII
# (em dashes, curly quotes) corrupts parsing. Keep this file ASCII; write data
# files with -Encoding utf8.

$ErrorActionPreference = 'Stop'

$appDir  = $PSScriptRoot
$dataDir = Join-Path $appDir 'data'
$config  = Join-Path $dataDir 'config.yaml'
$dbFile  = Join-Path $dataDir 'apotech.db'
$port    = 8080
$url     = "http://127.0.0.1:$port"

Set-Location $appDir
New-Item -ItemType Directory -Force -Path $dataDir | Out-Null
New-Item -ItemType Directory -Force -Path (Join-Path $dataDir 'backups') | Out-Null

# CSPRNG hex string (mirrors packaging/windows/setup.ps1 New-Secret).
function New-Secret([int]$bytes = 32) {
  $b = New-Object byte[] $bytes
  [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
  -join ($b | ForEach-Object { $_.ToString('x2') })
}

# True when config.yaml has a non-empty owner_email (i.e. bootstrap creds still live).
function Test-OwnerEmailSet([string]$path) {
  if (-not (Test-Path $path)) { return $false }
  $raw = Get-Content -Path $path -Raw
  $m = [regex]::Match($raw, '(?m)^\s*owner_email:\s*(.*)$')
  if (-not $m.Success) { return $false }
  $v = $m.Groups[1].Value.Trim().Trim('"').Trim("'").Trim()
  return ($v -ne '')
}

# Read a non-empty password of at least 8 chars from the console.
function Read-OwnerPassword {
  while ($true) {
    $secure = Read-Host -AsSecureString 'Owner password (min 8 chars)'
    $bstr   = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($secure)
    $plain  = [Runtime.InteropServices.Marshal]::PtrToStringBSTR($bstr)
    [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr)
    if ($plain.Length -ge 8) { return $plain }
    Write-Host 'Password must be at least 8 characters. Try again.'
  }
}

if (-not (Test-Path $config)) {
  # --- First run: prompt for the owner login + write config.yaml -------------
  Write-Host ''
  Write-Host 'First run: create the owner login for this pharmacy.'
  $email = (Read-Host 'Owner email [owner@apotech.local]').Trim()
  if ($email -eq '') { $email = 'owner@apotech.local' }
  $pw = Read-OwnerPassword
  $jwt = New-Secret 32

  # Single-quote the credentials so any special characters are YAML-safe
  # (single-quoted scalars only need '' for a literal quote).
  $emailY = "'" + ($email -replace "'", "''") + "'"
  $pwY    = "'" + ($pw    -replace "'", "''") + "'"

  $yaml = @"
server:
  host: 127.0.0.1
  port: $port

database:
  driver: sqlite
  path: ./data/apotech.db
  auto_migrate: true

auth:
  jwt_secret: $jwt
  access_token_ttl: 1h
  refresh_token_ttl: 720h

bootstrap:
  owner_email: $emailY
  owner_password: $pwY

backup:
  directory: ./data/backups
"@
  Set-Content -Path $config -Value $yaml -Encoding utf8
  Write-Host "Wrote $config"
}
elseif ((Test-Path $dbFile) -and (Test-OwnerEmailSet $config)) {
  # --- Second+ run after a successful bootstrap ------------------------------
  # The owner already exists in the DB. Blank the bootstrap credentials so the
  # server stops re-applying them on every boot (EnsureBootstrapOwner skips when
  # owner_email is empty), letting in-app password changes persist. Everything
  # else (jwt_secret, etc.) is preserved.
  $raw = Get-Content -Path $config -Raw
  $raw = [regex]::Replace($raw, '(?m)^(\s*owner_email:).*$', '$1 ""')
  $raw = [regex]::Replace($raw, '(?m)^(\s*owner_password:).*$', '$1 ""')
  Set-Content -Path $config -Value $raw -Encoding utf8
}
# Otherwise (config exists but the DB is missing -> a failed/aborted first run,
# or the user deleted only the DB): leave config as-is so the next boot
# re-bootstraps from the still-present credentials. Full reset = delete data\.

# Absolute path, recomputed every launch, so it survives the folder being moved.
$env:APOTECH_CONFIG = $config

# Open the browser once the server is actually accepting requests.
Start-Job -ArgumentList $url -ScriptBlock {
  param($u)
  for ($i = 0; $i -lt 60; $i++) {
    Start-Sleep -Milliseconds 500
    try {
      $r = Invoke-WebRequest -Uri "$u/healthz" -UseBasicParsing -TimeoutSec 2
      if ($r.StatusCode -eq 200) { Start-Process $u; return }
    } catch { }
  }
} | Out-Null

Write-Host ''
Write-Host "Starting Apotech at $url  (close this window to stop)"
Write-Host ''

# Run the server in the foreground: logs are visible and closing the window
# stops it. No service, no detached process to clean up.
& (Join-Path $appDir 'apotech.exe')

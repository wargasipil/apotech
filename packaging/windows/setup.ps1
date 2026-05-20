<#
  Apotech Windows post-install setup. Invoked by the Inno Setup installer (as
  Administrator) after files are copied. Idempotent: re-running on an existing
  install (upgrade) reuses the data dir + config and only refreshes services.

  Responsibilities:
    1. initdb the bundled PostgreSQL data dir (first install only) + create the
       `apotech` database, restrict ACLs, register it as the `apotech-postgres`
       Windows service (native pg_ctl, runs under NetworkService).
    2. Generate config.yaml in ProgramData (random jwt_secret + DB password;
       chosen host/ports + bootstrap owner). Never clobbered on upgrade.
    3. Register + start the `apotech-server` Windows service via WinSW.
    4. Add a firewall rule when LAN mode is selected.
    5. Create Start-menu + desktop shortcuts and open the browser.

  NOTE: This script targets a clean Windows 10/11 x64 machine. The PostgreSQL
  service account + data-dir ACLs are the most likely thing to need tuning on
  locked-down hosts -see packaging/windows/README.md.
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory)] [string] $AppDir,        # e.g. C:\Program Files\Apotech
  [Parameter(Mandatory)] [string] $DataDir,       # e.g. C:\ProgramData\Apotech
  [Parameter(Mandatory)] [string] $OwnerEmail,
  [Parameter(Mandatory)] [string] $OwnerPassword,
  [Parameter(Mandatory)] [string] $BindHost,      # 127.0.0.1 (single) or 0.0.0.0 (LAN)
  [Parameter(Mandatory)] [int]    $AppPort,
  [Parameter(Mandatory)] [int]    $PgPort,
  [int] $Lan = 0,
  [string] $Tz = "Asia/Jakarta"
)

$ErrorActionPreference = "Stop"
$pgsql   = Join-Path $AppDir "pgsql"
$pgbin   = Join-Path $pgsql  "bin"
$pgdata  = Join-Path $DataDir "pgdata"
$logdir  = Join-Path $DataDir "logs"
$backups = Join-Path $DataDir "backups"
$config  = Join-Path $DataDir "config.yaml"
$winsw   = Join-Path $AppDir "winsw\apotech-server.exe"
$winswXml= Join-Path $AppDir "winsw\apotech-server.xml"

New-Item -ItemType Directory -Force -Path $DataDir,$logdir,$backups | Out-Null

function New-Secret([int]$bytes = 32) {
  $b = New-Object byte[] $bytes
  [Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
  -join ($b | ForEach-Object { $_.ToString("x2") })
}

# --- 1. PostgreSQL: init + service (first install only) ----------------------
$pgFirstInit = -not (Test-Path (Join-Path $pgdata "PG_VERSION"))
if ($pgFirstInit) {
  Write-Host "Initializing PostgreSQL data directory..."
  $dbPassword = New-Secret 24
  $pwFile = Join-Path $env:TEMP "apotech_pw.txt"
  Set-Content -Path $pwFile -Value $dbPassword -NoNewline -Encoding ascii

  New-Item -ItemType Directory -Force -Path $pgdata | Out-Null
  & "$pgbin\initdb.exe" -D "$pgdata" -U apotech -A scram-sha-256 `
      --pwfile="$pwFile" --encoding=UTF8 -E UTF8 | Out-Null
  Remove-Item $pwFile -Force

  # Postgres stays bound to localhost regardless of app topology: clients hit
  # the Apotech server, never the DB directly.
  Add-Content (Join-Path $pgdata "postgresql.conf") "`nport = $PgPort`nlisten_addresses = '127.0.0.1'`n"

  # Restrict the data dir to NetworkService (service account) + Administrators.
  icacls "$pgdata" /inheritance:r /grant:r `
      "NT AUTHORITY\NetworkService:(OI)(CI)F" "Administrators:(OI)(CI)F" | Out-Null

  # Bring it up briefly to create the application database, then stop.
  & "$pgbin\pg_ctl.exe" -D "$pgdata" -o "-p $PgPort" -w start | Out-Null
  $env:PGPASSWORD = $dbPassword
  & "$pgbin\createdb.exe" -h 127.0.0.1 -p $PgPort -U apotech apotech
  & "$pgbin\pg_ctl.exe" -D "$pgdata" -w stop | Out-Null
  Remove-Item Env:\PGPASSWORD

  # Register the native Windows service (runs under NetworkService).
  & "$pgbin\pg_ctl.exe" register -N "apotech-postgres" -D "$pgdata" -S auto `
      -U "NT AUTHORITY\NetworkService" -o "-p $PgPort" | Out-Null
} else {
  Write-Host "Existing PostgreSQL data dir found -reusing it."
  # On upgrade we keep the existing DB password from config.yaml (read below).
}

Start-Service apotech-postgres -ErrorAction SilentlyContinue

# --- 2. config.yaml (first install only -never clobber existing secrets) -----
if (-not (Test-Path $config)) {
  if (-not $dbPassword) {
    throw "config.yaml missing but PostgreSQL already initialized; cannot recover DB password. Reinstall fresh or restore config.yaml."
  }
  $jwt = New-Secret 32
  $yaml = @"
server:
  host: $BindHost
  port: $AppPort

database:
  host: 127.0.0.1
  port: $PgPort
  user: apotech
  password: $dbPassword
  name: apotech
  sslmode: disable
  auto_migrate: true

auth:
  jwt_secret: $jwt
  access_token_ttl: 1h
  refresh_token_ttl: 720h

bootstrap:
  owner_email: $OwnerEmail
  owner_password: $OwnerPassword

printer:
  enabled: false
  address: 192.168.1.100:9100
  width: 32
  timeout: 5s
  open_drawer: false
  header:
    - "APOTECH PHARMACY"
  footer:
    - "Thank you!"
"@
  Set-Content -Path $config -Value $yaml -Encoding utf8
  Write-Host "Wrote $config"
} else {
  Write-Host "Existing config.yaml found -leaving it untouched."
}

# --- 3. App service via WinSW ------------------------------------------------
# Literal token replacement (paths contain backslashes -no regex).
$xml = (Get-Content $winswXml -Raw).
  Replace('@@CONFIG@@', $config).
  Replace('@@LOGDIR@@', $logdir).
  Replace('@@TZ@@', $Tz)
Set-Content $winswXml -Value $xml -Encoding utf8

& $winsw stop      2>$null
& $winsw uninstall 2>$null
& $winsw install
& $winsw start

# --- 4. Firewall (LAN mode only) --------------------------------------------
netsh advfirewall firewall delete rule name="Apotech Server" 2>$null | Out-Null
if ($Lan -eq 1) {
  netsh advfirewall firewall add rule name="Apotech Server" dir=in action=allow `
      protocol=TCP localport=$AppPort | Out-Null
  Write-Host "Firewall rule added for TCP $AppPort (LAN mode)."
}

# --- 5. Shortcuts + open browser --------------------------------------------
$url = "http://localhost:$AppPort"
$ws = New-Object -ComObject WScript.Shell
foreach ($dir in @(
    [Environment]::GetFolderPath("CommonDesktopDirectory"),
    (Join-Path $env:ProgramData "Microsoft\Windows\Start Menu\Programs"))) {
  $lnk = $ws.CreateShortcut((Join-Path $dir "Apotech.url"))
  $lnk.TargetPath = $url
  $lnk.Save()
}
Start-Process $url
Write-Host "Apotech setup complete: $url"

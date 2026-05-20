<#
  Apotech Windows teardown. Invoked by the Inno Setup uninstaller (as Admin)
  before files are removed. Stops + removes both services and the firewall rule.
  The PostgreSQL data dir + config are preserved unless -PurgeData 1 is passed
  (the uninstaller asks the user).
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory)] [string] $AppDir,
  [Parameter(Mandatory)] [string] $DataDir,
  [int] $PurgeData = 0
)

$ErrorActionPreference = "SilentlyContinue"
$pgbin  = Join-Path $AppDir "pgsql\bin"
$pgdata = Join-Path $DataDir "pgdata"
$winsw  = Join-Path $AppDir "winsw\apotech-server.exe"

# App service (WinSW)
& $winsw stop
& $winsw uninstall

# PostgreSQL service (native pg_ctl registration)
Stop-Service apotech-postgres -Force
& "$pgbin\pg_ctl.exe" unregister -N "apotech-postgres"

# Firewall rule
netsh advfirewall firewall delete rule name="Apotech Server" | Out-Null

if ($PurgeData -eq 1) {
  Remove-Item -Recurse -Force $DataDir
  Write-Host "Removed all Apotech data: $DataDir"
} else {
  Write-Host "Kept Apotech data (database + config) at $DataDir"
}

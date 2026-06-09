<#
  Assembles the Apotech Portable ZIP distribution.

  Produces a drop-anywhere folder containing apotech-portable.exe,
  a credentials-prompt first-run.bat, and a bilingual README.txt.
  Zipped via Compress-Archive (ships with Windows PowerShell 5.1).

  Differs from build-windows-portable.ps1: no Inno Setup, no installer
  scripts, no Start Menu shortcuts. Same EXE inside both flavors.

  Prerequisites:
    * Go 1.25+ and Node 20+ (unless -SkipExeBuild and a prebuilt
      dist\apotech-portable.exe already exists).

  Output: dist\ApotechPortable-<version>.zip

  Usage:
    powershell -ExecutionPolicy Bypass -File packaging\windows\build-zip-portable.ps1 `
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
$payload  = Join-Path $here "zip-portable"
$stage    = Join-Path $here "zip-stage"
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

# --- 2. Stage ApotechPortable\ ------------------------------------------------
Write-Host "Assembling ZIP stage..."
Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue
$inner = Join-Path $stage "ApotechPortable"
New-Item -ItemType Directory -Force -Path $inner | Out-Null

Copy-Item $exe                                       (Join-Path $inner "apotech-portable.exe")
Copy-Item (Join-Path $payload "first-run.bat")       (Join-Path $inner "first-run.bat")
Copy-Item (Join-Path $payload "README.txt")          (Join-Path $inner "README.txt")

# --- 3. Compress to dist\ApotechPortable-<version>.zip ------------------------
$out = Join-Path $dist ("ApotechPortable-$AppVersion.zip")
Remove-Item -Force $out -ErrorAction SilentlyContinue
Write-Host "Compressing $out ..."
Compress-Archive -Path $inner -DestinationPath $out -CompressionLevel Optimal -Force

# --- 4. Cleanup stage --------------------------------------------------------
Remove-Item -Recurse -Force $stage -ErrorAction SilentlyContinue

$size = (Get-Item $out).Length
Write-Host ("Done -> {0} ({1:N0} bytes)" -f $out, $size)

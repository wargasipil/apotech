<#
  Assembles the Apotech "portable" Windows package: a zip the user unzips and
  runs directly (SQLite, no installer, no PostgreSQL, no service).

  Prerequisite: dist\apotech.exe already exists. Build it first with
  `make dist-windows` (the `make dist-portable-windows` target does this for
  you). This script does NOT compile anything.

  Output: dist\ApotechPortable-win64.zip  (and the staged dist\Apotech-Portable\)

  Usage:
    powershell -ExecutionPolicy Bypass -File packaging\portable\build-portable.ps1

  ASCII-only (Windows PowerShell 5.1 reads -File as Windows-1252).
#>
[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$here  = $PSScriptRoot
$root  = (Resolve-Path (Join-Path $here "..\..")).Path
$dist  = Join-Path $root "dist"
$exe   = Join-Path $dist "apotech.exe"
$stage = Join-Path $dist "Apotech-Portable"
$zip   = Join-Path $dist "ApotechPortable-win64.zip"

if (-not (Test-Path $exe)) {
  throw "dist\apotech.exe not found. Build it first: make dist-windows"
}

Write-Host "Staging portable package..."
if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
New-Item -ItemType Directory -Force -Path $stage | Out-Null

Copy-Item $exe                              (Join-Path $stage "apotech.exe")
Copy-Item (Join-Path $here "Apotech.bat")   (Join-Path $stage "Apotech.bat")
Copy-Item (Join-Path $here "launch.ps1")    (Join-Path $stage "launch.ps1")
Copy-Item (Join-Path $here "README.txt")    (Join-Path $stage "README.txt")

Write-Host "Compressing $zip ..."
if (Test-Path $zip) { Remove-Item -Force $zip }
Compress-Archive -Path (Join-Path $stage "*") -DestinationPath $zip -Force

$sizeMb = [math]::Round((Get-Item $zip).Length / 1MB, 1)
Write-Host ""
Write-Host "Portable package ready:"
Write-Host "  $zip ($sizeMb MB)"
Write-Host "  $stage\ (unzipped)"

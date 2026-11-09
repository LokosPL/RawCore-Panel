$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$version = [regex]::Match((Get-Content updater.go -Raw), 'const appVersion = "([^"]+)"').Groups[1].Value
if (-not $version) { throw 'Missing application version' }
$compiler = $env:RAWCRAFT_ISCC
if (-not $compiler) { $compiler = Join-Path ${env:ProgramFiles(x86)} 'Inno Setup 6\ISCC.exe' }
if (-not (Test-Path $compiler)) { throw 'Install Inno Setup 6 to build the optional Windows installer.' }
& $compiler "/DAppVersion=$version" installer/RAWCRAFT-Control.iss
if ($LASTEXITCODE -ne 0) { throw "Installer compilation failed: $LASTEXITCODE" }
@('RAWCRAFT-Control.exe','RAWCRAFT-Control-Setup.exe') | ForEach-Object {
  $hash = (Get-FileHash (Join-Path dist $_) -Algorithm SHA256).Hash.ToLower()
  "$hash  $_"
} | Set-Content -Encoding ascii dist/SHA256SUMS

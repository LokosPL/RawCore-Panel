$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
$version = [regex]::Match((Get-Content updater.go -Raw), 'const appVersion = "([^"]+)"').Groups[1].Value
$installDir = Join-Path $env:RUNNER_TEMP 'RAWCRAFT installer smoke test'
$settingsDir = Join-Path $env:APPDATA 'RawcraftControl'
New-Item -ItemType Directory -Force $settingsDir | Out-Null
$sentinel = Join-Path $settingsDir 'installer-smoke-sentinel.txt'
Set-Content $sentinel 'preserve user settings'
function Run-Setup($exe, $arguments) {
  $process = Start-Process -FilePath $exe -ArgumentList $arguments -Wait -PassThru
  if ($process.ExitCode -ne 0) { throw "Setup failed with exit code $($process.ExitCode)" }
}
Run-Setup (Join-Path $PWD 'dist/RAWCRAFT-Control-Setup.exe') "/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /DIR=`"$installDir`""
$exe = Join-Path $installDir 'RAWCRAFT-Control.exe'
if ((Get-FileHash $exe).Hash -ne (Get-FileHash 'dist/RAWCRAFT-Control.exe').Hash) { throw 'Installed binary differs from release binary' }
$key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\RAWCRAFT-Control_is1'
Set-ItemProperty $key DisplayVersion '0.0.0'
# A real updater replacement must preserve uninstaller, chosen directory and profile.
$env:RAWCRAFT_UPDATE_SMOKE_DIR = $installDir
go test -v -run 'TestInstalledUpdatePreservesInstallation|TestFullUpdateRestartsVisiblePanel|TestLegacyHiddenUpdaterReopensVisiblePanel|TestFailedUpdateKeepsOnlyInstaller' .
if ($LASTEXITCODE -ne 0) { throw 'Installed update test failed' }
if ((Get-ItemProperty $key).DisplayVersion -ne $version) { throw 'Updater failed to synchronize installed version' }
# Reinstall without /DIR must reuse the registered custom location.
Run-Setup (Join-Path $PWD 'dist/RAWCRAFT-Control-Setup.exe') '/VERYSILENT /SUPPRESSMSGBOXES /NORESTART'
$key = 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\RAWCRAFT-Control_is1'
$registered = Get-ItemProperty $key
if ($registered.InstallLocation.TrimEnd('\') -ne $installDir.TrimEnd('\')) { throw 'Installer changed the chosen path' }
if ($registered.DisplayVersion -ne $version) { throw 'Installed version metadata incorrect' }
Run-Setup (Join-Path $installDir 'unins000.exe') '/VERYSILENT /SUPPRESSMSGBOXES /NORESTART'
if (Test-Path $exe) { throw 'Uninstall left application binary' }
if (-not (Test-Path $sentinel)) { throw 'Uninstall removed user settings' }
Remove-Item $sentinel
# Full removal on request: /PURGE deletes settings, saved secrets, cache, WebView data and shortcuts.
Run-Setup (Join-Path $PWD 'dist/RAWCRAFT-Control-Setup.exe') "/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /DIR=`"$installDir`""
if (-not (Test-Path $exe)) { throw 'Reinstall before purge failed' }
$desktopLink = Join-Path ([Environment]::GetFolderPath('Desktop')) 'RAWCRAFT Control.lnk'
if (-not (Test-Path $desktopLink)) { throw 'Desktop shortcut was not created by default' }
$cacheDir = Join-Path $env:LOCALAPPDATA 'RawcraftControl'
New-Item -ItemType Directory -Force $settingsDir, (Join-Path $cacheDir 'updates') | Out-Null
Set-Content (Join-Path $settingsDir 'purge-sentinel.txt') 'remove me'
Set-Content (Join-Path $cacheDir 'updates\purge-sentinel.txt') 'remove me'
Set-Content (Join-Path $installDir 'RAWCRAFT-Control.exe.old') 'stale update leftover'
Run-Setup (Join-Path $installDir 'unins000.exe') '/VERYSILENT /SUPPRESSMSGBOXES /NORESTART /PURGE'
if (Test-Path $exe) { throw 'Purge uninstall left application binary' }
for ($i = 0; ($i -lt 20) -and (Test-Path $installDir); $i++) { Start-Sleep -Milliseconds 500 }
if (Test-Path $installDir) { Get-ChildItem -Force $installDir | Format-Table -AutoSize | Out-String | Write-Host; throw 'Purge uninstall left the installation folder' }
if (Test-Path $settingsDir) { throw 'Purge uninstall kept user settings' }
if (Test-Path $cacheDir) { throw 'Purge uninstall kept cache and WebView data' }
if (Test-Path $desktopLink) { throw 'Uninstall left the desktop shortcut' }
if (Test-Path 'HKCU:\Software\Microsoft\Windows\CurrentVersion\Uninstall\RAWCRAFT-Control_is1') { throw 'Uninstall left the registry entry' }
Write-Host 'PASS: install, update, reinstall, keep-settings uninstall and full /PURGE uninstall'

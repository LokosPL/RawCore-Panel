$ErrorActionPreference = 'Stop'
Set-Location (Split-Path $PSScriptRoot -Parent)
go mod download
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go test ./...
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
go run github.com/akavel/rsrc@v0.10.2 -ico assets/app.ico -o resource_windows_amd64.syso
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
New-Item -ItemType Directory -Force dist | Out-Null
go build -trimpath -ldflags="-H=windowsgui -s -w" -o dist/RAWCRAFT-Control.exe .
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
$hash = (Get-FileHash dist/RAWCRAFT-Control.exe -Algorithm SHA256).Hash.ToLower()
"$hash  RAWCRAFT-Control.exe" | Set-Content -Encoding ascii dist/SHA256SUMS
Copy-Item README_PL.txt,THIRD_PARTY_NOTICES.txt dist/

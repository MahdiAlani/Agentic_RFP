# Starts the full Agentic RFP stack: infra containers, migrations, API, frontend.
# Run from the repo root:  ./start.ps1
# (Workers are started separately - see README.)

# Note: we intentionally do NOT set ErrorActionPreference='Stop'. In Windows
# PowerShell that turns any stderr line from a native command (e.g. a benign
# docker/psql warning) into a fatal error. We check exit codes explicitly instead.

$root = $PSScriptRoot
Set-Location $root

Write-Host "==> Freeing ports 8080 and 5173 (stopping any leftover API/frontend)..." -ForegroundColor Cyan
foreach ($port in 8080, 5173) {
    $conns = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
    foreach ($c in $conns) {
        try { Stop-Process -Id $c.OwningProcess -Force -ErrorAction Stop } catch {}
    }
}

Write-Host "==> Starting infra containers (postgres, redis, minio)..." -ForegroundColor Cyan
docker compose up -d postgres redis minio
if ($LASTEXITCODE -ne 0) { Write-Host "docker compose up failed." -ForegroundColor Red; exit 1 }

# The migrate service waits for Postgres to be healthy, then applies anything new.
Write-Host "==> Applying migrations..." -ForegroundColor Cyan
docker compose run --rm migrate
if ($LASTEXITCODE -ne 0) {
    Write-Host "Migrations failed. If this database was set up before the migrate service existed," -ForegroundColor Red
    Write-Host "record its current version once with:  docker compose run --rm migrate force 9" -ForegroundColor Red
    exit 1
}

# Quiet the collation-version warning that appears after switching Postgres images.
docker compose exec -T postgres psql -U postgres -d app -c "ALTER DATABASE app REFRESH COLLATION VERSION;" 2>$null | Out-Null

Write-Host "==> Launching API (backend) in a new window..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$root/backend'; go run ./cmd/api"

Write-Host "==> Launching frontend in a new window..." -ForegroundColor Cyan
Start-Process powershell -ArgumentList "-NoExit", "-Command", "Set-Location '$root/frontend'; if (-not (Test-Path node_modules)) { npm install }; npm run dev"

Write-Host ""
Write-Host "All set:" -ForegroundColor Green
Write-Host "  Frontend : http://localhost:5173"
Write-Host "  API      : http://localhost:8080"
Write-Host "  MinIO    : http://localhost:9001  (credentials from .env)"
Write-Host ""
Write-Host "Start the workers in another terminal:" -ForegroundColor Yellow
Write-Host "  cd workers; pip install -r requirements.txt; python main.py"

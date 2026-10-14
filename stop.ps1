# Stops the Agentic RFP stack started by start.ps1:
# the API (:8080), the frontend dev server (:5173), and the infra containers.
# Workers (if running) stop with Ctrl+C in their own terminal.
# Run from the repo root:  ./stop.ps1

$root = $PSScriptRoot
Set-Location $root

Write-Host "==> Stopping app processes (ports 8080, 5173)..." -ForegroundColor Cyan
foreach ($port in 8080, 5173) {
    $conns = Get-NetTCPConnection -LocalPort $port -State Listen -ErrorAction SilentlyContinue
    foreach ($c in $conns) {
        try {
            Stop-Process -Id $c.OwningProcess -Force -ErrorAction Stop
            Write-Host "    stopped PID $($c.OwningProcess) on :$port"
        } catch {}
    }
}

Write-Host "==> Stopping infra containers..." -ForegroundColor Cyan
docker compose stop

Write-Host ""
Write-Host "Stopped. Data is preserved (containers stopped, volumes kept)." -ForegroundColor Green
Write-Host "Restart with ./start.ps1"

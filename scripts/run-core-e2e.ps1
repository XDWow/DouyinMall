$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$backendDir = Join-Path $repoRoot "backend"
$composeFile = Join-Path $repoRoot "docker-compose.yml"
$gocacheDir = Join-Path $repoRoot ".gocache"

Write-Host "Starting core compose services..."
docker compose -f $composeFile up -d mysql redis etcd kafka product-service order-service inventory-service payment-service checkout-service seckill-service

Push-Location $backendDir
try {
    $env:GOCACHE = $gocacheDir
    Write-Host "Running core compose E2E tests..."
    go test -tags=integration ./internal/checkout/integration ./internal/seckill/integration -v
}
finally {
    Pop-Location
}

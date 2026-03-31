param(
    [int]$CheckoutIterations = 200,
    [int]$CheckoutVUs = 20,
    [int]$SeckillIterations = 1000,
    [int]$SeckillVUs = 100,
    [string]$BffBaseUrl = "http://127.0.0.1:8080"
)

$ErrorActionPreference = "Stop"

$root = "D:\workspace\go\DouyinMall"
$backend = Join-Path $root "backend"
$binDir = Join-Path $backend ".bin"
$bffExe = Join-Path $binDir "bff.exe"
$checkoutOut = Join-Path $backend ".k6-checkout-formal.txt"
$seckillOut = Join-Path $backend ".k6-seckill-formal.txt"

New-Item -ItemType Directory -Path $binDir -Force | Out-Null

Push-Location $backend
try {
    $env:GOCACHE = Join-Path $backend ".gocache"
    go build -o $bffExe ./cmd/bff

    if (Test-Path $checkoutOut) { Remove-Item $checkoutOut -Force }
    if (Test-Path $seckillOut) { Remove-Item $seckillOut -Force }

    $proc = Start-Process -FilePath $bffExe -ArgumentList "--config", (Join-Path $backend "internal\bff\config\dev.yaml") -WindowStyle Hidden -PassThru
    try {
        Start-Sleep -Seconds 4

        k6 run (Join-Path $root "scripts\k6\bff_checkout_place_order.js") `
            -e "BFF_BASE_URL=$BffBaseUrl" `
            -e "ITERATIONS=$CheckoutIterations" `
            -e "VUS=$CheckoutVUs" | Out-File -FilePath $checkoutOut -Encoding utf8

        k6 run (Join-Path $root "scripts\k6\bff_seckill_submit.js") `
            -e "BFF_BASE_URL=$BffBaseUrl" `
            -e "ITERATIONS=$SeckillIterations" `
            -e "VUS=$SeckillVUs" | Out-File -FilePath $seckillOut -Encoding utf8

        Write-Host "==== checkout ===="
        Get-Content $checkoutOut
        Write-Host ""
        Write-Host "==== seckill ===="
        Get-Content $seckillOut
    }
    finally {
        if ($null -ne $proc) {
            Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
        }
    }
}
finally {
    Pop-Location
}

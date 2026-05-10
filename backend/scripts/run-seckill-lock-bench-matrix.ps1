param(
    [string]$ConfigMode = "dev",
    [string]$RocketMQEndpoint = "43.136.121.171:8081",
    [string]$RocketMQNameServer = "43.136.121.171:9876",
    [string]$DbPassword = "0324",
    [string]$RedisPassword = "0624",
    [string]$HandleTimeoutSec = "25",
    [int]$Requests = 2000,
    [int]$Concurrency = 300,
    [int]$Stock = 0,
    [int]$ProbeSeconds = 60,
    [int]$ProbeIntervalMs = 100,
    [int]$WarmupSeconds = 3
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$matrixRoot = Join-Path $repoRoot ".local\benchmarks\seckill-lock"
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$stockLabel = if ($Stock -gt 0) { "stock$Stock" } else { "stockAuto" }
$summaryPath = Join-Path $matrixRoot "matrix-$timestamp-r$Requests-c$Concurrency-$stockLabel.json"
$singleScript = Join-Path $PSScriptRoot "run-seckill-lock-bench.ps1"

New-Item -ItemType Directory -Force -Path $matrixRoot | Out-Null

$cases = @(
    @{
        Name = "gw4"
        GlobalWorkerNum = "4"
    },
    @{
        Name = "gw8"
        GlobalWorkerNum = "8"
    },
    @{
        Name = "gw10"
        GlobalWorkerNum = "10"
    },
    @{
        Name = "gw12"
        GlobalWorkerNum = "12"
    },
    @{
        Name = "gw16"
        GlobalWorkerNum = "16"
    }
)

$summaries = @()

foreach ($case in $cases) {
    Write-Host ("running case: {0}" -f $case.Name)

    & $singleScript `
        -ConfigMode $ConfigMode `
        -RocketMQEndpoint $RocketMQEndpoint `
        -RocketMQNameServer $RocketMQNameServer `
        -DbPassword $DbPassword `
        -RedisPassword $RedisPassword `
        -GlobalWorkerNum $case.GlobalWorkerNum `
        -HandleTimeoutSec $HandleTimeoutSec `
        -Requests $Requests `
        -Concurrency $Concurrency `
        -Stock $Stock `
        -ProbeSeconds $ProbeSeconds `
        -ProbeIntervalMs $ProbeIntervalMs `
        -WarmupSeconds $WarmupSeconds `
        -StopStackAfterRun

    $pattern = "*-gw$($case.GlobalWorkerNum)-r$Requests-c$Concurrency-*-stock*"
    $resultDir = Get-ChildItem -Path $matrixRoot -Directory |
        Where-Object { $_.Name -like $pattern } |
        Sort-Object LastWriteTime -Descending |
        Select-Object -First 1

    if ($null -eq $resultDir) {
        throw "result directory not found for case $($case.Name)"
    }

    $benchPath = Join-Path $resultDir.FullName "bench.json"
    $lockPath = Join-Path $resultDir.FullName "lockprobe.json"
    $bench = Get-Content -Raw -Path $benchPath | ConvertFrom-Json
    $lock = Get-Content -Raw -Path $lockPath | ConvertFrom-Json

    $avgLockWaitMs = 0.0
    if ($lock.deltaLockWaits -gt 0) {
        $avgLockWaitMs = [double]$lock.deltaLockTimeMs / [double]$lock.deltaLockWaits
    }

    $summary = [pscustomobject]@{
        case = $case.Name
        globalWorkerNum = [int]$case.GlobalWorkerNum
        requests = $Requests
        concurrency = $Concurrency
        stock = if ($Stock -gt 0) { $Stock } else { $bench.expected_stock }
        success = $bench.success
        failure = $bench.failure
        expectedStock = $bench.expected_stock
        finalSuccessCount = $bench.final_success_count
        finalAvailableStock = $bench.final_available_stock
        oversoldCount = $bench.oversold_count
        submitDurationMs = $bench.submit_duration_ms
        settleDurationMs = $bench.settle_duration_ms
        endToEndDurationMs = $bench.end_to_end_duration_ms
        submitRps = $bench.rps
        submitP99Ms = $bench.p99_ms
        deltaLockWaits = $lock.deltaLockWaits
        deltaLockTimeMs = $lock.deltaLockTimeMs
        avgLockWaitMs = $avgLockWaitMs
        maxCurrentWaits = $lock.maxCurrentWaits
        maxDataLockWaits = $lock.maxDataLockWaits
        dataLockWaitsReady = $lock.dataLockWaitsReady
        statusCounts = $bench.status_counts
        resultDir = $resultDir.FullName
    }

    $summaries += $summary
    $summaries | ConvertTo-Json -Depth 8 | Set-Content -Encoding UTF8 -Path $summaryPath
}

Write-Host "matrix summary: $summaryPath"
$summaries | Format-Table case, globalWorkerNum, finalSuccessCount, settleDurationMs, deltaLockWaits, deltaLockTimeMs, avgLockWaitMs, maxCurrentWaits -AutoSize

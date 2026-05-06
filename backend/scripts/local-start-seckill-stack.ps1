param(
    [string]$ConfigMode = "dev",
    [string]$RocketMQEndpoint = "43.136.121.171:8081",
    [string]$DbPassword = "0324",
    [string]$RedisPassword = "0624",
    [string]$SnowflakeNodeId = "1"
)

$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$runtimeDir = Join-Path $repoRoot ".local\seckill-stack"
$logDir = Join-Path $runtimeDir "logs"
$pidFile = Join-Path $runtimeDir "pids.json"

New-Item -ItemType Directory -Force -Path $logDir | Out-Null

if (Test-Path $pidFile) {
    Write-Host "existing pid file found, stopping previous local stack first"
    & (Join-Path $PSScriptRoot "local-stop-seckill-stack.ps1")
}

$services = @(
    @{ Name = "order";     Config = "internal/order/config/$ConfigMode.yaml" },
    @{ Name = "payment";   Config = "internal/payment/config/$ConfigMode.yaml" },
    @{ Name = "inventory"; Config = "internal/inventory/config/$ConfigMode.yaml" },
    @{ Name = "coupon";    Config = "internal/coupon/config/$ConfigMode.yaml" },
    @{ Name = "cart";      Config = "internal/cart/config/$ConfigMode.yaml" },
    @{ Name = "seckill";   Config = "internal/seckill/config/$ConfigMode.yaml" }
)

$processes = @()

foreach ($service in $services) {
    $logPath = Join-Path $logDir ($service.Name + ".log")
    Set-Content -Path $logPath -Value ""
    $command = @(
        '$env:SNOWFLAKE_NODE_ID=''' + $SnowflakeNodeId + ''''
    )
    if ($DbPassword -ne "") {
        $command += '$env:DB_PASSWORD=''' + $DbPassword + ''''
    }
    if ($RedisPassword -ne "") {
        $command += '$env:REDIS_PASSWORD=''' + $RedisPassword + ''''
    }
    if ($RocketMQEndpoint -ne "") {
        $command += '$env:ROCKETMQ_ENDPOINT=''' + $RocketMQEndpoint + ''''
    }
    $command += 'Set-Location ''' + $repoRoot + ''''
    $command += 'go run ./cmd/' + $service.Name + ' --config ' + $service.Config + ' *>> ''' + $logPath + ''''

    $proc = Start-Process powershell `
        -ArgumentList @("-NoProfile", "-Command", ($command -join "; ")) `
        -WorkingDirectory $repoRoot `
        -WindowStyle Hidden `
        -PassThru

    $processes += [pscustomobject]@{
        Name   = $service.Name
        Pid    = $proc.Id
        Log    = $logPath
        Config = $service.Config
    }

    Start-Sleep -Seconds 2
}

$processes | ConvertTo-Json | Set-Content -Path $pidFile

Write-Host "local seckill stack started"
Write-Host "pid file: $pidFile"
Write-Host "logs:"
foreach ($proc in $processes) {
    Write-Host ("  {0}: {1}" -f $proc.Name, $proc.Log)
}

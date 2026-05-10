param(
    [string]$ConfigMode = "dev",
    [string]$RocketMQEndpoint = "43.136.121.171:8081",
    [string]$RocketMQNameServer = "43.136.121.171:9876",
    [string]$DbPassword = "0324",
    [string]$RedisPassword = "0624",
    [string]$GlobalWorkerNum = "10",
    [string]$HandleTimeoutSec = "25",
    [int]$Requests = 400,
    [int]$Concurrency = 100,
    [int]$Stock = 0,
    [int]$Users = 0,
    [int]$TargetQps = 0,
    [int]$DuplicatePercent = 0,
    [int]$HotPercent = 100,
    [int]$ProbeSeconds = 20,
    [int]$ProbeIntervalMs = 100,
    [int]$WarmupSeconds = 3,
    [int]$ReadyTimeoutSeconds = 90,
    [switch]$StopStackAfterRun
)

$ErrorActionPreference = "Stop"
if ($null -ne (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue)) {
    $PSNativeCommandUseErrorActionPreference = $false
}

function Wait-TcpPort {
    param(
        [string]$HostName,
        [int]$Port,
        [int]$TimeoutSeconds
    )

    $deadline = (Get-Date).AddSeconds($TimeoutSeconds)
    while ((Get-Date) -lt $deadline) {
        $client = New-Object System.Net.Sockets.TcpClient
        try {
            $connect = $client.BeginConnect($HostName, $Port, $null, $null)
            if ($connect.AsyncWaitHandle.WaitOne(1000, $false)) {
                $client.EndConnect($connect)
                return
            }
        }
        catch {
        }
        finally {
            $client.Close()
        }
        Start-Sleep -Milliseconds 500
    }

    throw "timeout waiting for ${HostName}:${Port}"
}

$repoRoot = Split-Path -Parent $PSScriptRoot
$timestamp = Get-Date -Format "yyyyMMdd-HHmmss"
$stockLabel = if ($Stock -gt 0) { "stock$Stock" } else { "stockAuto" }
$userLabel = if ($Users -gt 0) { "u$Users" } else { "uAuto" }
$qpsLabel = if ($TargetQps -gt 0) { "qps$TargetQps" } else { "qpsMax" }
$mixLabel = "dup$DuplicatePercent-hot$HotPercent"
$resultDir = Join-Path $repoRoot ".local\benchmarks\seckill-lock\$timestamp-gw$GlobalWorkerNum-r$Requests-c$Concurrency-$userLabel-$qpsLabel-$mixLabel-$stockLabel"
New-Item -ItemType Directory -Force -Path $resultDir | Out-Null

$mysqlDsn = "app:$DbPassword@tcp(134.175.67.79:3306)/douyin_mall?charset=utf8mb4&parseTime=True&loc=Local"
$benchPath = Join-Path $resultDir "bench.json"
$lockPath = Join-Path $resultDir "lockprobe.json"
$benchErrPath = Join-Path $resultDir "bench.stderr.log"
$lockErrPath = Join-Path $resultDir "lockprobe.stderr.log"
$benchExe = Join-Path $resultDir "bench.exe"
$lockProbeExe = Join-Path $resultDir "lockprobe.exe"

Push-Location $repoRoot
$stackStarted = $false
try {
Write-Host "building bench tools..."
& go build -o $benchExe ./cmd/bench
if ($LASTEXITCODE -ne 0) {
    throw "build bench failed, exitCode=$LASTEXITCODE"
}
& go build -o $lockProbeExe ./.local/lockprobe
if ($LASTEXITCODE -ne 0) {
    throw "build lockprobe failed, exitCode=$LASTEXITCODE"
}

& (Join-Path $PSScriptRoot "local-start-seckill-stack.ps1") `
    -ConfigMode $ConfigMode `
    -RocketMQEndpoint $RocketMQEndpoint `
    -RocketMQNameServer $RocketMQNameServer `
    -DbPassword $DbPassword `
    -RedisPassword $RedisPassword `
    -SeckillGlobalWorkerNum $GlobalWorkerNum `
    -SeckillHandleTimeoutSec $HandleTimeoutSec
$stackStarted = $true

if ($WarmupSeconds -gt 0) {
    Write-Host "warming up stack for $WarmupSeconds seconds..."
    Start-Sleep -Seconds $WarmupSeconds
}

Write-Host "waiting for local service ports..."
foreach ($port in @(8095, 8098)) {
    Wait-TcpPort -HostName "127.0.0.1" -Port $port -TimeoutSeconds $ReadyTimeoutSeconds
}

$lockProc = $null
$lockProc = Start-Process -FilePath $lockProbeExe `
    -ArgumentList @("-dsn", $mysqlDsn, "-duration", "$($ProbeSeconds)s", "-interval", "$($ProbeIntervalMs)ms") `
    -WorkingDirectory $repoRoot `
    -RedirectStandardOutput $lockPath `
    -RedirectStandardError $lockErrPath `
    -WindowStyle Hidden `
    -PassThru

try {
    $benchArgs = @(
        "-mode", "seckill",
        "-seckill_addr", "127.0.0.1:8098",
        "-mysql_dsn", $mysqlDsn,
        "-requests", $Requests,
        "-concurrency", $Concurrency,
        "-users", $Users,
        "-target_qps", $TargetQps,
        "-duplicate_percent", $DuplicatePercent,
        "-hot_percent", $HotPercent
    )
    if ($Stock -gt 0) {
        $benchArgs += @("-stock", $Stock)
    }
    $benchProc = Start-Process -FilePath $benchExe `
        -ArgumentList $benchArgs `
        -WorkingDirectory $repoRoot `
        -RedirectStandardOutput $benchPath `
        -RedirectStandardError $benchErrPath `
        -WindowStyle Hidden `
        -PassThru
    $benchProc.WaitForExit()
    $benchProc.Refresh()
    $benchExitCode = [int]$benchProc.ExitCode
    if ($benchExitCode -ne 0) {
        $errTail = ""
        if (Test-Path $benchErrPath) {
            $errTail = (Get-Content -Path $benchErrPath -Tail 40) -join "`n"
        }
        throw "bench failed, exitCode=$benchExitCode`n$errTail"
    }
    if (-not (Test-Path $benchPath) -or [string]::IsNullOrWhiteSpace((Get-Content -Raw -Path $benchPath))) {
        throw "bench produced no stdout"
    }
}
finally {
    if ($null -ne $lockProc) {
        $lockProc.WaitForExit()
        $lockProc.Refresh()
    }
}

$lockExitCode = 0
if ($null -ne $lockProc -and $null -ne $lockProc.ExitCode) {
    $lockExitCode = [int]$lockProc.ExitCode
}
if ($lockExitCode -ne 0) {
    $errTail = ""
    if (Test-Path $lockErrPath) {
        $errTail = (Get-Content -Path $lockErrPath -Tail 40) -join "`n"
    }
    throw "lockprobe failed, exitCode=$lockExitCode`n$errTail"
}

if (-not (Test-Path $lockPath)) {
    $errTail = ""
    if (Test-Path $lockErrPath) {
        $errTail = (Get-Content -Path $lockErrPath -Tail 40) -join "`n"
    }
    throw "lockprobe produced no output`n$errTail"
}


Write-Host "result dir: $resultDir"
Write-Host "bench: $benchPath"
Write-Host "lockprobe: $lockPath"
}
finally {
    if ($StopStackAfterRun -and $stackStarted) {
        & (Join-Path $PSScriptRoot "local-stop-seckill-stack.ps1")
    }
    Pop-Location
}

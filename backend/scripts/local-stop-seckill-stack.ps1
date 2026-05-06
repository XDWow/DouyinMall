$ErrorActionPreference = "Stop"

$repoRoot = Split-Path -Parent $PSScriptRoot
$runtimeDir = Join-Path $repoRoot ".local\seckill-stack"
$pidFile = Join-Path $runtimeDir "pids.json"

if (-not (Test-Path $pidFile)) {
    Write-Host "no local seckill stack pid file found"
} else {
    $processes = Get-Content -Path $pidFile | ConvertFrom-Json

    foreach ($proc in $processes) {
        try {
            $p = Get-Process -Id $proc.Pid -ErrorAction Stop
            Stop-Process -Id $p.Id -Force
            Write-Host ("stopped {0} pid={1}" -f $proc.Name, $proc.Pid)
        } catch {
            Write-Host ("skip {0} pid={1}, already exited" -f $proc.Name, $proc.Pid)
        }
    }

    Remove-Item -LiteralPath $pidFile -Force
}

$ports = @(8092, 8093, 8094, 8095, 8097, 8098, 8099, 18095, 18098, 18099)
$listenerPids = Get-NetTCPConnection -LocalPort $ports -ErrorAction SilentlyContinue |
    Where-Object { $_.State -eq "Listen" } |
    Select-Object -ExpandProperty OwningProcess -Unique

foreach ($listenerPid in $listenerPids) {
    try {
        $p = Get-Process -Id $listenerPid -ErrorAction Stop
        Stop-Process -Id $p.Id -Force
        Write-Host ("stopped listener process={0} pid={1}" -f $p.ProcessName, $p.Id)
    } catch {
        Write-Host ("skip listener pid={0}, already exited" -f $listenerPid)
    }
}

Write-Host "local seckill stack stopped"

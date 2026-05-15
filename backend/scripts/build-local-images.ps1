param(
    [string]$Namespace = "douyinmall",
    [string]$Tag = "local",
    [string[]]$Services = @(
        "aftersale",
        "agent",
        "bff",
        "cart",
        "checkout",
        "coupon",
        "inventory",
        "order",
        "payment",
        "product",
        "seckill",
        "user"
    ),
    [switch]$NoCache
)

$ErrorActionPreference = "Stop"
if ($null -ne (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue)) {
    $PSNativeCommandUseErrorActionPreference = $false
}

$repoRoot = Split-Path -Parent $PSScriptRoot
Push-Location $repoRoot
try {
    foreach ($service in $Services) {
        $dockerfile = "cmd/$service/Dockerfile"
        if (-not (Test-Path $dockerfile)) {
            throw "dockerfile not found: $dockerfile"
        }

        $image = "${Namespace}/${service}:${Tag}"
        Write-Host "=== Building $image ==="

        $args = @("build", "-f", $dockerfile, "-t", $image)
        if ($NoCache) {
            $args += "--no-cache"
        }
        $args += "."

        & docker @args
        if ($LASTEXITCODE -ne 0) {
            throw "docker build failed for $image"
        }
    }
}
finally {
    Pop-Location
}

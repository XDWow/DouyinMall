param(
    [Parameter(Mandatory = $true)]
    [string]$TargetNamespace,
    [string]$SourceNamespace = "douyinmall",
    [string]$SourceTag = "local",
    [string]$TargetTag = "latest",
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
    [switch]$Push
)

$ErrorActionPreference = "Stop"
if ($null -ne (Get-Variable -Name PSNativeCommandUseErrorActionPreference -ErrorAction SilentlyContinue)) {
    $PSNativeCommandUseErrorActionPreference = $false
}

foreach ($service in $Services) {
    $source = "${SourceNamespace}/${service}:${SourceTag}"
    $target = "${TargetNamespace}/${service}:${TargetTag}"

    Write-Host "=== Tagging $source -> $target ==="
    & docker tag $source $target
    if ($LASTEXITCODE -ne 0) {
        throw "docker tag failed for $source"
    }

    if ($Push) {
        Write-Host "=== Pushing $target ==="
        & docker push $target
        if ($LASTEXITCODE -ne 0) {
            throw "docker push failed for $target"
        }
    }
}

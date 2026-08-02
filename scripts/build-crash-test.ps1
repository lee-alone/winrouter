param([string]$Version = '0.1.0-dev')

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$output = Join-Path $root 'build\crash-test-bin'
$buildTime = [DateTime]::UtcNow.ToString('o')
$flags = "-X winrouter/internal/buildinfo.Version=$Version -X winrouter/internal/buildinfo.Commit=unknown -X winrouter/internal/buildinfo.BuildTime=$buildTime -X main.faultInjectionBuild=true"
Push-Location $root
try {
    $env:GOCACHE = Join-Path $root '.gocache'
    New-Item -ItemType Directory -Force -Path $output | Out-Null
    go build -ldflags $flags -o (Join-Path $output 'WinRouter-helper.exe') .\cmd\winrouter-helper
    if ($LASTEXITCODE -ne 0) { throw "test helper build failed ($LASTEXITCODE)" }
    go build -o (Join-Path $output 'WinRouter-core-smoke.exe') .\cmd\winrouter-core-smoke
    if ($LASTEXITCODE -ne 0) { throw "core smoke build failed ($LASTEXITCODE)" }
    New-Item -ItemType Directory -Force -Path (Join-Path $output 'resources\core') | Out-Null
    Copy-Item -LiteralPath 'resources\core\sing-box.exe','resources\core\manifest.json','resources\core\LICENSE' -Destination (Join-Path $output 'resources\core') -Force
} finally {
    Pop-Location
}

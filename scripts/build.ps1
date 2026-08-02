param([string]$Version = '0.1.0-dev')

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$commit = if (Test-Path (Join-Path $root '.git')) { git -C $root rev-parse --short HEAD } else { 'unknown' }
$buildTime = [DateTime]::UtcNow.ToString('o')
$flags = "-X winrouter/internal/buildinfo.Version=$Version -X winrouter/internal/buildinfo.Commit=$commit -X winrouter/internal/buildinfo.BuildTime=$buildTime"
Push-Location $root
try {
    $env:GOCACHE = Join-Path $root '.gocache'
    $env:npm_config_cache = Join-Path $root '.npm-cache'
    wails build -clean -ldflags $flags
    if ($LASTEXITCODE -ne 0) { throw "wails build failed ($LASTEXITCODE)" }
    go build -ldflags $flags -o build\bin\WinRouter-helper.exe .\cmd\winrouter-helper
    if ($LASTEXITCODE -ne 0) { throw "helper build failed ($LASTEXITCODE)" }
    go build -ldflags $flags -o build\bin\WinRouter-core-smoke.exe .\cmd\winrouter-core-smoke
    if ($LASTEXITCODE -ne 0) { throw "core smoke build failed ($LASTEXITCODE)" }
    New-Item -ItemType Directory -Force -Path 'build\bin\resources\core' | Out-Null
    Copy-Item -LiteralPath 'resources\core\sing-box.exe','resources\core\manifest.json','resources\core\LICENSE' -Destination 'build\bin\resources\core' -Force
    New-Item -ItemType Directory -Force -Path 'build\metadata' | Out-Null
    [ordered]@{
        version = $Version; commit = $commit; build_time = $buildTime
        go = (go version); node = (node --version)
        wails = ((wails version | Select-Object -First 1) -replace '\x1b\[[0-9;]*m', '')
        sing_box = '1.13.15'
    } | ConvertTo-Json | Set-Content -LiteralPath 'build\metadata\build.json' -Encoding UTF8
    go list -m -json all | Set-Content -LiteralPath 'build\metadata\go-modules.jsonl' -Encoding UTF8
    npm.cmd --prefix frontend ls --all --json | Set-Content -LiteralPath 'build\metadata\npm-dependencies.json' -Encoding UTF8
} finally { Pop-Location }

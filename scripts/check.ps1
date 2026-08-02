$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Push-Location $root
try {
    $env:GOCACHE = Join-Path $root '.gocache'
    $env:npm_config_cache = Join-Path $root '.npm-cache'
    Push-Location frontend
    try {
        npm.cmd ci
        if ($LASTEXITCODE -ne 0) { throw "npm ci failed ($LASTEXITCODE)" }
        npm.cmd run check
        if ($LASTEXITCODE -ne 0) { throw "frontend type check failed ($LASTEXITCODE)" }
        npm.cmd run build
        if ($LASTEXITCODE -ne 0) { throw "frontend build failed ($LASTEXITCODE)" }
    } finally { Pop-Location }
    $unformatted = gofmt -l .
    if ($unformatted) { throw "Go files require gofmt:`n$unformatted" }
    go vet ./...
    if ($LASTEXITCODE -ne 0) { throw "go vet failed ($LASTEXITCODE)" }
    go test ./...
    if ($LASTEXITCODE -ne 0) { throw "go test failed ($LASTEXITCODE)" }
} finally { Pop-Location }

param([string]$Version = '0.1.0-preview')

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$bin = Join-Path $root 'build\bin'
$releaseRoot = Join-Path $root 'build\release'
$stage = Join-Path $releaseRoot "WinRouter-$Version-windows-amd64"
$zip = "$stage.zip"

foreach ($required in @('WinRouter.exe', 'WinRouter-helper.exe', 'resources\core\sing-box.exe', 'resources\core\LICENSE')) {
    if (-not (Test-Path -LiteralPath (Join-Path $bin $required))) { throw "missing build artifact: $required" }
}
$metadata = Get-Content -Raw -LiteralPath (Join-Path $root 'build\metadata\build.json') | ConvertFrom-Json
if ($metadata.version -ne $Version) { throw "build version '$($metadata.version)' does not match package version '$Version'" }
if (Test-Path -LiteralPath $stage) { Remove-Item -LiteralPath $stage -Recurse -Force }
if (Test-Path -LiteralPath $zip) { Remove-Item -LiteralPath $zip -Force }

New-Item -ItemType Directory -Force -Path (Join-Path $stage 'resources\core'),(Join-Path $stage 'metadata') | Out-Null
Copy-Item -LiteralPath (Join-Path $bin 'WinRouter.exe'),(Join-Path $bin 'WinRouter-helper.exe') -Destination $stage
if (Test-Path (Join-Path $bin 'resources\core')) {
    Get-ChildItem -LiteralPath (Join-Path $bin 'resources\core') -File | Copy-Item -Destination (Join-Path $stage 'resources\core') -Force
}
Copy-Item -LiteralPath (Join-Path $root 'docs\release\phase-1-preview.md'),(Join-Path $root 'docs\release\THIRD-PARTY-NOTICES.md') -Destination $stage
Copy-Item -LiteralPath (Join-Path $root 'build\metadata\build.json'),(Join-Path $root 'build\metadata\go-modules.jsonl'),(Join-Path $root 'build\metadata\npm-dependencies.json') -Destination (Join-Path $stage 'metadata')

$hashes = Get-ChildItem -LiteralPath $stage -File -Recurse | Sort-Object FullName | ForEach-Object {
    $relative = $_.FullName.Substring($stage.Length + 1).Replace('\', '/')
    "{0}  {1}" -f (Get-FileHash -LiteralPath $_.FullName -Algorithm SHA256).Hash, $relative
}
$hashes | Set-Content -LiteralPath (Join-Path $stage 'SHA256SUMS.txt') -Encoding ascii
Compress-Archive -LiteralPath $stage -DestinationPath $zip -CompressionLevel Optimal
Get-FileHash -LiteralPath $zip -Algorithm SHA256

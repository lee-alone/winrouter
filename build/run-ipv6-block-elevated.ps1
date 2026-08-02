$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'
$stdoutPath = Join-Path $outputDirectory 'ipv6-block.stdout.json'
$stderrPath = Join-Path $outputDirectory 'ipv6-block.stderr.log'
$restorePath = Join-Path $outputDirectory 'ipv6-post-stop-aaaa.json'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
Set-Location -LiteralPath $projectRoot
$ErrorActionPreference = 'Continue'
& $probePath --run-ipv6 --stack system --core $corePath 1> $stdoutPath 2> $stderrPath
$exitCode = $LASTEXITCODE
$ErrorActionPreference = 'Stop'
if ($exitCode -ne 0) {
    exit $exitCode
}

$restored = Resolve-DnsName -Name cloudflare.com -Type AAAA -DnsOnly | Where-Object { $_.Type -eq 'AAAA' } | Select-Object Name,IPAddress
$restored | ConvertTo-Json | Set-Content -LiteralPath $restorePath -Encoding UTF8
if (-not $restored) {
    exit 2
}

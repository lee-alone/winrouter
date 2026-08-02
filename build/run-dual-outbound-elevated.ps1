$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$stdoutPath = Join-Path $outputDirectory 'dual-outbound.stdout.json'
$stderrPath = Join-Path $outputDirectory 'dual-outbound.stderr.log'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
$ErrorActionPreference = 'Continue'
Set-Location -LiteralPath $projectRoot
& $probePath --run-dual --stack system --core $corePath 1> $stdoutPath 2> $stderrPath
exit $LASTEXITCODE

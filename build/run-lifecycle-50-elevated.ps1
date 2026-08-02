$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'
$stdoutPath = Join-Path $outputDirectory 'lifecycle-50.stdout.json'
$stderrPath = Join-Path $outputDirectory 'lifecycle-50.stderr.log'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
Set-Location -LiteralPath $projectRoot
$ErrorActionPreference = 'Continue'
& $probePath --run-lifecycle --cycles 50 --stack system --core $corePath 1> $stdoutPath 2> $stderrPath
$exitCode = $LASTEXITCODE
$ErrorActionPreference = 'Stop'
exit $exitCode

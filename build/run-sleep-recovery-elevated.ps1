$ErrorActionPreference = 'Continue'
$projectRoot = Split-Path -Parent $PSScriptRoot
Set-Location -LiteralPath $projectRoot
& (Join-Path $projectRoot 'bin\winrouter-probe.exe') --run-dual --stack system --core (Join-Path $projectRoot 'resources\core\sing-box.exe') 1> (Join-Path $PSScriptRoot 'phase0\sleep-recovery.stdout.json') 2> (Join-Path $PSScriptRoot 'phase0\sleep-recovery.stderr.log')
exit $LASTEXITCODE

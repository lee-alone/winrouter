$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
Set-Location -LiteralPath $projectRoot

foreach ($mode in @('realip', 'fakeip')) {
    $stdoutPath = Join-Path $outputDirectory "$mode.stdout.json"
    $stderrPath = Join-Path $outputDirectory "$mode.stderr.log"
    $ErrorActionPreference = 'Continue'
    & $probePath "--run-$mode" --stack system --core $corePath 1> $stdoutPath 2> $stderrPath
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
    if ($exitCode -ne 0) {
        exit $exitCode
    }
}

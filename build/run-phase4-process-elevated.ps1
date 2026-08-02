$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'
$outputDirectory = Join-Path $PSScriptRoot 'phase4-process'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

foreach ($ruleType in @('name', 'path')) {
    $stdoutPath = Join-Path $outputDirectory "process-$ruleType.stdout.json"
    $stderrPath = Join-Path $outputDirectory "process-$ruleType.stderr.log"
    $process = Start-Process -FilePath $probePath -ArgumentList @('--run-process', '--process-rule', $ruleType, '--stack', 'system', '--core', $corePath) -Wait -PassThru -NoNewWindow -RedirectStandardOutput $stdoutPath -RedirectStandardError $stderrPath
    if ($process.ExitCode -ne 0) {
        throw "process $ruleType experiment failed: $(Get-Content -LiteralPath $stderrPath -Raw)"
    }
}

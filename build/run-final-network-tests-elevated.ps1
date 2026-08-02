$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
Set-Location -LiteralPath $projectRoot
Start-Transcript -Path (Join-Path $outputDirectory 'final-network-tests.transcript.log') -Force | Out-Null

function Invoke-CapturedExperiment {
    param(
        [string]$Name,
        [string[]]$Targets,
        [string]$Mode
    )

    $etlPath = Join-Path $outputDirectory "$Name.etl"
    $capturePath = Join-Path $outputDirectory "$Name.capture.txt"
    $stdoutPath = Join-Path $outputDirectory "$Name.stdout.json"
    $stderrPath = Join-Path $outputDirectory "$Name.stderr.log"

    $ErrorActionPreference = 'Continue'
    pktmon stop 2>$null | Out-Null
    $ErrorActionPreference = 'Stop'
    pktmon filter remove | Out-Null
    foreach ($target in $Targets) {
        pktmon filter add $target -i $target | Out-Null
    }
    try {
        pktmon start --capture --comp nics --pkt-size 0 --file-name $etlPath | Out-Null
        $ErrorActionPreference = 'Continue'
        & $probePath $Mode --stack system --core $corePath 1> $stdoutPath 2> $stderrPath
        $exitCode = $LASTEXITCODE
        $ErrorActionPreference = 'Stop'
    }
    finally {
        pktmon stop | Out-Null
        pktmon filter remove | Out-Null
    }
    pktmon etl2txt $etlPath --out $capturePath --brief | Out-Null
    if ($exitCode -ne 0) {
        throw "$Name experiment failed with exit code $exitCode"
    }
}

Invoke-CapturedExperiment -Name 'dual-outbound-captured' -Targets @('1.1.1.1', '8.8.8.8', '162.159.200.1', '162.159.200.123') -Mode '--run-dual'
Invoke-CapturedExperiment -Name 'dual-lan' -Targets @('10.12.85.232', '192.168.1.1') -Mode '--run-lan'
Stop-Transcript | Out-Null

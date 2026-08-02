$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'
$etlPath = Join-Path $outputDirectory 'dual-dns.etl'
$capturePath = Join-Path $outputDirectory 'dual-dns.capture.txt'
$stdoutPath = Join-Path $outputDirectory 'dual-dns.stdout.json'
$stderrPath = Join-Path $outputDirectory 'dual-dns.stderr.log'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
Set-Location -LiteralPath $projectRoot
Start-Transcript -Path (Join-Path $outputDirectory 'dual-dns.transcript.log') -Force | Out-Null

$ErrorActionPreference = 'Continue'
pktmon stop 2>$null | Out-Null
$ErrorActionPreference = 'Stop'
pktmon filter remove | Out-Null
pktmon filter add dns-a -i 1.1.1.1 -t UDP -p 53 | Out-Null
pktmon filter add dns-b -i 8.8.8.8 -t UDP -p 53 | Out-Null
try {
    pktmon start --capture --comp nics --pkt-size 0 --file-name $etlPath | Out-Null
    $ErrorActionPreference = 'Continue'
    & $probePath --run-dns --stack system --core $corePath 1> $stdoutPath 2> $stderrPath
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
}
finally {
    pktmon stop | Out-Null
    pktmon filter remove | Out-Null
}
pktmon etl2txt $etlPath --out $capturePath --brief | Out-Null
Stop-Transcript | Out-Null
exit $exitCode

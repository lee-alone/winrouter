$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'
$etlPath = Join-Path $outputDirectory 'encrypted-dns.etl'
$capturePath = Join-Path $outputDirectory 'encrypted-dns.capture.txt'
$stdoutPath = Join-Path $outputDirectory 'encrypted-dns.stdout.json'
$stderrPath = Join-Path $outputDirectory 'encrypted-dns.stderr.log'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
Set-Location -LiteralPath $projectRoot
$ErrorActionPreference = 'Continue'
pktmon stop 2>$null | Out-Null
$ErrorActionPreference = 'Stop'
pktmon filter remove | Out-Null
pktmon filter add dot-a -i 1.1.1.1 -t TCP -p 853 | Out-Null
pktmon filter add doh-b -i 8.8.8.8 -t TCP -p 443 | Out-Null
try {
    pktmon start --capture --comp nics --pkt-size 0 --file-name $etlPath | Out-Null
    $ErrorActionPreference = 'Continue'
    & $probePath --run-encrypted-dns --stack system --core $corePath 1> $stdoutPath 2> $stderrPath
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
}
finally {
    pktmon stop | Out-Null
    pktmon filter remove | Out-Null
}
pktmon etl2txt $etlPath --out $capturePath --brief | Out-Null
exit $exitCode

param(
    [string]$ProbePath = '',
    [string]$EvidenceDirectory = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
if (-not $ProbePath) { $ProbePath = Join-Path $root 'bin\winrouter-probe.exe' }
if (-not $EvidenceDirectory) { $EvidenceDirectory = Join-Path $root 'build\phase1-mvp-semantic' }
$ProbePath = [IO.Path]::GetFullPath($ProbePath)
$EvidenceDirectory = [IO.Path]::GetFullPath($EvidenceDirectory)
$corePath = [IO.Path]::GetFullPath((Join-Path $root 'resources\core\sing-box.exe'))
if (-not $ProbePath.StartsWith($root, [StringComparison]::OrdinalIgnoreCase) -or
    -not $EvidenceDirectory.StartsWith($root, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Probe and evidence paths must remain inside the WinRouter workspace.'
}
if (-not (Test-Path -LiteralPath $ProbePath -PathType Leaf)) { throw "Probe not found: $ProbePath" }

New-Item -ItemType Directory -Force -Path $EvidenceDirectory | Out-Null
$executionLog = Join-Path $EvidenceDirectory 'execution.log'
Start-Transcript -LiteralPath $executionLog -Force | Out-Null
try {
$etl = Join-Path $EvidenceDirectory 'mvp-semantic.etl'
$capture = Join-Path $EvidenceDirectory 'mvp-semantic.capture.txt'
$stdout = Join-Path $EvidenceDirectory 'mvp-semantic.stdout.json'
$stderr = Join-Path $EvidenceDirectory 'mvp-semantic.stderr.log'
$summaryPath = Join-Path $EvidenceDirectory 'mvp-semantic-summary.json'

$savedErrorAction = $ErrorActionPreference
$ErrorActionPreference = 'SilentlyContinue'
& pktmon stop 2>$null | Out-Null
$ErrorActionPreference = $savedErrorAction
& pktmon filter remove | Out-Null
foreach ($target in @('223.5.5.5', '1.1.1.1', '8.8.8.8')) {
    & pktmon filter add $target -i $target | Out-Null
}
try {
    & pktmon start --capture --comp nics --pkt-size 0 --file-name $etl | Out-Null
    $process = Start-Process -FilePath $ProbePath -ArgumentList @('--run-mvp-semantic', '--stack', 'system', '--core', $corePath) -WorkingDirectory $root -Wait -PassThru -NoNewWindow -RedirectStandardOutput $stdout -RedirectStandardError $stderr
} finally {
    & pktmon stop | Out-Null
    & pktmon filter remove | Out-Null
}
& pktmon etl2txt $etl --out $capture | Out-Null
if ($process.ExitCode -ne 0) { throw "MVP semantic probe failed; inspect $stderr" }

$report = Get-Content -LiteralPath $stdout -Raw -Encoding UTF8 | ConvertFrom-Json
$captureText = Get-Content -LiteralPath $capture -Raw -Encoding UTF8
function Get-IPv4Address($adapter) {
    return @($adapter.addresses | Where-Object { $_.ip -match '^\d+\.\d+\.\d+\.\d+$' } | Select-Object -ExpandProperty ip)[0]
}
$sourceA = Get-IPv4Address $report.interfaces[0]
$sourceB = Get-IPv4Address $report.interfaces[1]
if (-not $sourceA -or -not $sourceB) { throw 'Both selected interfaces must have an IPv4 address.' }
$aDomesticBusiness = [regex]::Escape($sourceA) + '\.\d+ > 223\.5\.5\.5\.443'
$bDomesticBusiness = [regex]::Escape($sourceB) + '\.\d+ > 223\.5\.5\.5\.443'
$bForeignBusiness = [regex]::Escape($sourceB) + '\.\d+ > 1\.1\.1\.1\.443'
$aForeignBusiness = [regex]::Escape($sourceA) + '\.\d+ > 1\.1\.1\.1\.443'
$aDomesticDNS = [regex]::Escape($sourceA) + '\.\d+ > 223\.5\.5\.5\.53'
$bDomesticDNS = [regex]::Escape($sourceB) + '\.\d+ > 223\.5\.5\.5\.53'
$bGlobalDNS = [regex]::Escape($sourceB) + '\.\d+ > 8\.8\.8\.8\.53'
$aGlobalDNS = [regex]::Escape($sourceA) + '\.\d+ > 8\.8\.8\.8\.53'

$checks = @(
    [ordered]@{ name = 'domestic-business-via-a'; passed = ($captureText -match $aDomesticBusiness) -and ($captureText -notmatch $bDomesticBusiness) },
    [ordered]@{ name = 'foreign-business-via-b'; passed = ($captureText -match $bForeignBusiness) -and ($captureText -notmatch $aForeignBusiness) },
    [ordered]@{ name = 'domestic-dns-via-a-no-leak'; passed = ($captureText -match $aDomesticDNS) -and ($captureText -notmatch $bDomesticDNS) },
    [ordered]@{ name = 'global-dns-via-b-no-leak'; passed = ($captureText -match $bGlobalDNS) -and ($captureText -notmatch $aGlobalDNS) },
    [ordered]@{ name = 'domestic-dns-route-log'; passed = $report.experiment.core_output -match 'outbound/direct\[domestic-direct\].*223\.5\.5\.5:53' },
    [ordered]@{ name = 'global-dns-route-log'; passed = $report.experiment.core_output -match 'outbound/direct\[foreign-direct\].*8\.8\.8\.8:53' },
    [ordered]@{ name = 'domestic-domain-resolved'; passed = [bool]$report.experiment.dns_probes[0].resolved },
    [ordered]@{ name = 'global-domain-resolved'; passed = [bool]$report.experiment.dns_probes[1].resolved },
    [ordered]@{ name = 'cleanup'; passed = [bool]($report.experiment.interface_cleaned -and $report.experiment.routes_cleaned) }
)
$summary = [ordered]@{
    timestamp = (Get-Date).ToString('o')
    interface_a = [ordered]@{ name = $report.interfaces[0].friendly_name; guid = $report.interfaces[0].guid; source_ipv4 = $sourceA }
    interface_b = [ordered]@{ name = $report.interfaces[1].friendly_name; guid = $report.interfaces[1].guid; source_ipv4 = $sourceB }
    checks = $checks
    passed = -not ($checks | Where-Object { -not $_.passed })
    evidence = @('mvp-semantic.stdout.json', 'mvp-semantic.capture.txt', 'mvp-semantic.etl', 'mvp-semantic.stderr.log')
}
$summary | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $summaryPath -Encoding UTF8
if (-not $summary.passed) { throw "MVP semantic validation failed; inspect $summaryPath" }
$summary | ConvertTo-Json -Depth 6
} catch {
    $_ | Format-List * -Force | Out-String | Set-Content -LiteralPath (Join-Path $EvidenceDirectory 'error.log') -Encoding UTF8
    throw
} finally {
    Stop-Transcript -ErrorAction SilentlyContinue | Out-Null
}

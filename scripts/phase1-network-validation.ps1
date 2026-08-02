param(
    [string]$ProbePath = '',
    [string]$EvidenceDirectory = ''
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
$executionLog = Join-Path $root 'build\phase1-network-validation.execution.log'
Start-Transcript -LiteralPath $executionLog -Force | Out-Null
try {
if (-not $ProbePath) { $ProbePath = Join-Path $root 'bin\winrouter-probe.exe' }
if (-not $EvidenceDirectory) { $EvidenceDirectory = Join-Path $root 'build\phase1' }
$ProbePath = [IO.Path]::GetFullPath($ProbePath)
$EvidenceDirectory = [IO.Path]::GetFullPath($EvidenceDirectory)
$corePath = [IO.Path]::GetFullPath((Join-Path $root 'resources\core\sing-box.exe'))
if (-not $ProbePath.StartsWith($root, [StringComparison]::OrdinalIgnoreCase) -or
    -not $EvidenceDirectory.StartsWith($root, [StringComparison]::OrdinalIgnoreCase)) {
    throw 'Probe and evidence paths must remain inside the WinRouter workspace.'
}
if (-not (Test-Path -LiteralPath $ProbePath -PathType Leaf)) { throw "Probe not found: $ProbePath" }
New-Item -ItemType Directory -Force -Path $EvidenceDirectory | Out-Null

$cases = @(
    @{ Name = 'dual-outbound'; Argument = '--run-dual' },
    @{ Name = 'dual-dns'; Argument = '--run-dns' },
    @{ Name = 'dual-lan'; Argument = '--run-lan' },
    @{ Name = 'ipv6-block'; Argument = '--run-ipv6' }
)
$summary = @()
foreach ($case in $cases) {
    $base = Join-Path $EvidenceDirectory $case.Name
    $etl = "$base.etl"
    $capture = "$base.capture.txt"
    $stdout = "$base.stdout.json"
    $stderr = "$base.stderr.log"
    & pktmon filter remove | Out-Null
    & pktmon start --capture --pkt-size 0 --file-name $etl | Out-Null
    $exitCode = 1
    try {
        $process = Start-Process -FilePath $ProbePath -ArgumentList @($case.Argument, '--stack', 'system', '--core', $corePath) -WorkingDirectory $root -Wait -PassThru -NoNewWindow -RedirectStandardOutput $stdout -RedirectStandardError $stderr
        $exitCode = $process.ExitCode
    } finally {
        & pktmon stop | Out-Null
        & pktmon etl2txt $etl --out $capture | Out-Null
    }
    $summary += [ordered]@{ name = $case.Name; exit_code = $exitCode; stdout = [IO.Path]::GetFileName($stdout); capture = [IO.Path]::GetFileName($capture) }
    if ($exitCode -ne 0) { throw "Validation case $($case.Name) failed; inspect $stderr" }
}
$summary | ConvertTo-Json -Depth 4 | Set-Content -LiteralPath (Join-Path $EvidenceDirectory 'network-validation-summary.json') -Encoding UTF8
} finally {
    Stop-Transcript | Out-Null
}

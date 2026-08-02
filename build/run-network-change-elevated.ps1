$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'
$changeStdout = Join-Path $outputDirectory 'network-change.stdout.json'
$changeStderr = Join-Path $outputDirectory 'network-change.stderr.log'
$recoveryStdout = Join-Path $outputDirectory 'network-recovery.stdout.json'
$recoveryStderr = Join-Path $outputDirectory 'network-recovery.stderr.log'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
Set-Location -LiteralPath $projectRoot
$adapter = Get-NetAdapter -Name 'WLAN' -ErrorAction Stop
if ($adapter.Status -ne 'Up') { throw 'WLAN must be Up before the experiment' }

$probe = Start-Process -FilePath $probePath -ArgumentList @('--run-network-change','--stack','system','--core',$corePath) -RedirectStandardOutput $changeStdout -RedirectStandardError $changeStderr -PassThru -WindowStyle Hidden
try {
    $deadline = (Get-Date).AddSeconds(20)
    do {
        Start-Sleep -Milliseconds 250
        $tun = Get-NetIPAddress -InterfaceAlias 'WinRouter-TUN' -AddressFamily IPv4 -ErrorAction SilentlyContinue
    } while (-not $tun -and (Get-Date) -lt $deadline -and -not $probe.HasExited)
    if (-not $tun) { throw 'TUN did not appear before WLAN change' }
    Disable-NetAdapter -Name 'WLAN' -Confirm:$false
    if (-not $probe.WaitForExit(20000)) { throw 'controller did not stop after WLAN was disabled' }
}
finally {
    Enable-NetAdapter -Name 'WLAN' -Confirm:$false -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 2
    $ErrorActionPreference = 'Continue'
    netsh wlan connect name="iQOO Neo8 Pro" interface="WLAN" | Out-Null
    $ErrorActionPreference = 'Stop'
    if (-not $probe.HasExited) { Stop-Process -Id $probe.Id -Force -ErrorAction SilentlyContinue }
}

$deadline = (Get-Date).AddSeconds(60)
do {
    Start-Sleep -Milliseconds 500
    $adapter = Get-NetAdapter -Name 'WLAN' -ErrorAction SilentlyContinue
    $address = Get-NetIPAddress -InterfaceAlias 'WLAN' -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -notlike '169.254.*' } | Select-Object -First 1
} while (($adapter.Status -ne 'Up' -or -not $address) -and (Get-Date) -lt $deadline)
if ($adapter.Status -ne 'Up' -or -not $address) { throw 'WLAN did not recover in 60 seconds' }

$ErrorActionPreference = 'Continue'
& $probePath --run-dual --stack system --core $corePath 1> $recoveryStdout 2> $recoveryStderr
$exitCode = $LASTEXITCODE
$ErrorActionPreference = 'Stop'
exit $exitCode

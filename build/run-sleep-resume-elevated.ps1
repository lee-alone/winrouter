$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'
$sleepStdout = Join-Path $outputDirectory 'sleep-resume.stdout.json'
$sleepStderr = Join-Path $outputDirectory 'sleep-resume.stderr.log'
$recoveryStdout = Join-Path $outputDirectory 'sleep-recovery.stdout.json'
$recoveryStderr = Join-Path $outputDirectory 'sleep-recovery.stderr.log'
$metadataPath = Join-Path $outputDirectory 'sleep-resume.metadata.json'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
Set-Location -LiteralPath $projectRoot
$probe = Start-Process -FilePath $probePath -ArgumentList @('--run-sleep-resume','--stack','system','--core',$corePath) -RedirectStandardOutput $sleepStdout -RedirectStandardError $sleepStderr -PassThru -WindowStyle Hidden
try {
    $deadline = (Get-Date).AddSeconds(20)
    do {
        Start-Sleep -Milliseconds 250
        $tun = Get-NetIPAddress -InterfaceAlias 'WinRouter-TUN' -AddressFamily IPv4 -ErrorAction SilentlyContinue
    } while (-not $tun -and (Get-Date) -lt $deadline -and -not $probe.HasExited)
    if (-not $tun) { throw 'TUN did not appear before sleep' }

    $sleepRequested = Get-Date
    & rundll32.exe powrprof.dll,SetSuspendState 0,1,0
    $resumed = Get-Date
    if (-not $probe.WaitForExit(30000)) { throw 'controller did not stop within 30 seconds after resume' }

    $deadline = (Get-Date).AddSeconds(60)
    do {
        Start-Sleep -Milliseconds 500
        $wlan = Get-NetAdapter -Name 'WLAN' -ErrorAction SilentlyContinue
        $wlanAddress = Get-NetIPAddress -InterfaceAlias 'WLAN' -AddressFamily IPv4 -ErrorAction SilentlyContinue | Where-Object { $_.IPAddress -notlike '169.254.*' } | Select-Object -First 1
        if ($wlan.Status -ne 'Up' -or -not $wlanAddress) {
            Enable-NetAdapter -Name 'WLAN' -Confirm:$false -ErrorAction SilentlyContinue
            $ErrorActionPreference = 'Continue'
            netsh wlan connect name="iQOO Neo8 Pro" interface="WLAN" | Out-Null
            $ErrorActionPreference = 'Stop'
        }
    } while (($wlan.Status -ne 'Up' -or -not $wlanAddress) -and (Get-Date) -lt $deadline)
    if ($wlan.Status -ne 'Up' -or -not $wlanAddress) { throw 'WLAN did not recover after resume' }

    $metadata = [ordered]@{
        sleep_requested = $sleepRequested.ToString('o')
        command_returned = $resumed.ToString('o')
        elapsed_seconds = [math]::Round(($resumed - $sleepRequested).TotalSeconds, 3)
        wlan_address = $wlanAddress.IPAddress
    }
    $metadata | ConvertTo-Json | Set-Content -LiteralPath $metadataPath -Encoding UTF8

    $ErrorActionPreference = 'Continue'
    & $probePath --run-dual --stack system --core $corePath 1> $recoveryStdout 2> $recoveryStderr
    $exitCode = $LASTEXITCODE
    $ErrorActionPreference = 'Stop'
    exit $exitCode
}
finally {
    if (-not $probe.HasExited) { Stop-Process -Id $probe.Id -Force -ErrorAction SilentlyContinue }
}

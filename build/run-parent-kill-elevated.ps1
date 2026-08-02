$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'
$childStdout = Join-Path $outputDirectory 'parent-kill.child.stdout.log'
$childStderr = Join-Path $outputDirectory 'parent-kill.child.stderr.log'
$resultPath = Join-Path $outputDirectory 'parent-kill.result.json'

New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null
Set-Location -LiteralPath $projectRoot
$probe = Start-Process -FilePath $probePath -ArgumentList @('--run-tun','--hold-seconds','60','--stack','system','--core',$corePath) -RedirectStandardOutput $childStdout -RedirectStandardError $childStderr -PassThru -WindowStyle Hidden
$corePid = $null
try {
    $deadline = (Get-Date).AddSeconds(20)
    do {
        Start-Sleep -Milliseconds 250
        $core = Get-CimInstance Win32_Process -Filter "ParentProcessId=$($probe.Id)" -ErrorAction SilentlyContinue | Where-Object { $_.Name -eq 'sing-box.exe' } | Select-Object -First 1
        $tun = Get-NetIPAddress -InterfaceAlias 'WinRouter-TUN' -AddressFamily IPv4 -ErrorAction SilentlyContinue
    } while ((-not $core -or -not $tun) -and (Get-Date) -lt $deadline -and -not $probe.HasExited)
    if (-not $core -or -not $tun) {
        throw 'controlled process did not create sing-box and TUN in time'
    }
    $corePid = [int]$core.ProcessId
    Stop-Process -Id $probe.Id -Force
    $deadline = (Get-Date).AddSeconds(15)
    do {
        Start-Sleep -Milliseconds 250
        $coreAlive = Get-Process -Id $corePid -ErrorAction SilentlyContinue
        $tunAddresses = @(Get-NetIPAddress -InterfaceAlias 'WinRouter-TUN' -ErrorAction SilentlyContinue)
        $tunRoutes = @(Get-NetRoute -InterfaceAlias 'WinRouter-TUN' -ErrorAction SilentlyContinue)
    } while (($coreAlive -or $tunAddresses.Count -or $tunRoutes.Count) -and (Get-Date) -lt $deadline)
    $result = [ordered]@{
        parent_pid = $probe.Id
        core_pid = $corePid
        parent_forced = $true
        core_exited = -not [bool]$coreAlive
        tun_addresses = $tunAddresses.Count
        tun_routes = $tunRoutes.Count
    }
    $result | ConvertTo-Json | Set-Content -LiteralPath $resultPath -Encoding UTF8
    if ($coreAlive -or $tunAddresses.Count -or $tunRoutes.Count) {
        exit 2
    }
}
finally {
    if (-not $probe.HasExited) { Stop-Process -Id $probe.Id -Force -ErrorAction SilentlyContinue }
    if ($corePid) { Stop-Process -Id $corePid -Force -ErrorAction SilentlyContinue }
}

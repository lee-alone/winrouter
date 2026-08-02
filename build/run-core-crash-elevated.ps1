$ErrorActionPreference = 'Stop'

$projectRoot = Split-Path -Parent $PSScriptRoot
$outputDirectory = Join-Path $PSScriptRoot 'phase0'
$probePath = Join-Path $projectRoot 'bin\winrouter-probe.exe'
$corePath = Join-Path $projectRoot 'resources\core\sing-box.exe'
$childStdout = Join-Path $outputDirectory 'core-crash.child.stdout.log'
$childStderr = Join-Path $outputDirectory 'core-crash.child.stderr.log'
$resultPath = Join-Path $outputDirectory 'core-crash.result.json'

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
    if (-not $core -or -not $tun) { throw 'core and TUN did not appear' }
    $corePid = [int]$core.ProcessId
    Stop-Process -Id $corePid -Force
    if (-not $probe.WaitForExit(15000)) { throw 'controller did not stop after core crash' }
    $probe.WaitForExit()
    $probe.Refresh()
    $controllerExitCode = $probe.ExitCode
    Start-Sleep -Seconds 2
    $restarted = @(Get-CimInstance Win32_Process -Filter "ParentProcessId=$($probe.Id)" -ErrorAction SilentlyContinue | Where-Object { $_.Name -eq 'sing-box.exe' })
    $allCore = @(Get-Process sing-box -ErrorAction SilentlyContinue)
    $tunAddresses = @(Get-NetIPAddress -InterfaceAlias 'WinRouter-TUN' -ErrorAction SilentlyContinue)
    $tunRoutes = @(Get-NetRoute -InterfaceAlias 'WinRouter-TUN' -ErrorAction SilentlyContinue)
    $result = [ordered]@{
        parent_pid = $probe.Id
        crashed_core_pid = $corePid
        controller_exit_code = $controllerExitCode
        restarted_children = $restarted.Count
        remaining_core_processes = $allCore.Count
        tun_addresses = $tunAddresses.Count
        tun_routes = $tunRoutes.Count
    }
    $result | ConvertTo-Json | Set-Content -LiteralPath $resultPath -Encoding UTF8
    if ($controllerExitCode -eq 0 -or $restarted.Count -or $allCore.Count -or $tunAddresses.Count -or $tunRoutes.Count) { exit 2 }
}
finally {
    if (-not $probe.HasExited) { Stop-Process -Id $probe.Id -Force -ErrorAction SilentlyContinue }
    if ($corePid) { Stop-Process -Id $corePid -Force -ErrorAction SilentlyContinue }
}

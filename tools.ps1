param(
    [ValidateSet('all', 'check', 'build')][string]$Task = 'all',
    [string]$Version = '0.1.0-dev'
)

$ErrorActionPreference = 'Stop'
$root = Split-Path -Parent $MyInvocation.MyCommand.Path
if ($Task -eq 'all') {
    & (Join-Path $root 'scripts\check.ps1')
    if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
    & (Join-Path $root 'scripts\build.ps1') -Version $Version
} else {
    if ($Task -eq 'build') {
        & (Join-Path $root 'scripts\build.ps1') -Version $Version
    } else {
        & (Join-Path $root 'scripts\check.ps1')
    }
}

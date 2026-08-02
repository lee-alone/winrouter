param(
    [Parameter(Mandatory = $true)][string]$OutputPath,
    [int]$Seconds = 45
)

$ErrorActionPreference = 'Continue'
for ($i = 0; $i -lt $Seconds; $i++) {
    "===== $([DateTime]::UtcNow.ToString('o')) =====" | Add-Content -Encoding UTF8 -LiteralPath $OutputPath
    route.exe print -4 | Add-Content -Encoding UTF8 -LiteralPath $OutputPath
    '-- processes --' | Add-Content -Encoding UTF8 -LiteralPath $OutputPath
    Get-Process WinRouter, WinRouter-helper, sing-box -ErrorAction SilentlyContinue |
        Select-Object ProcessName, Id, StartTime |
        Format-Table -HideTableHeaders |
        Out-String |
        Add-Content -Encoding UTF8 -LiteralPath $OutputPath
    Start-Sleep -Seconds 1
}

$ErrorActionPreference = 'Stop'

$outputPath = Join-Path $PSScriptRoot 'phase0\dual-interface-baseline.json'
$outputDirectory = Split-Path -Parent $outputPath
New-Item -ItemType Directory -Force -Path $outputDirectory | Out-Null

$sources = @('10.12.85.4', '192.168.1.166')
$diagnostics = foreach ($source in $sources) {
    $result = Test-NetConnection -ComputerName '1.1.1.1' -DiagnoseRouting -ConstrainSourceAddress $source -InformationLevel Detailed
    [pscustomobject]@{
        requested_source = $source
        selected_source = [string]$result.SelectedSourceAddress.IPAddress
        outgoing_interface_index = $result.OutgoingInterfaceIndex
        next_hop = [string]$result.SelectedNetRoute.NextHop
        destination_prefix = [string]$result.SelectedNetRoute.DestinationPrefix
        succeeded = $result.RouteDiagnosticsSucceeded
    }
}

$routes = Get-NetRoute -AddressFamily IPv4 -DestinationPrefix '0.0.0.0/0' | ForEach-Object {
    [pscustomobject]@{
        interface_alias = $_.InterfaceAlias
        interface_index = $_.InterfaceIndex
        next_hop = $_.NextHop
        route_metric = $_.RouteMetric
        state = [string]$_.State
    }
}

[pscustomobject]@{
    captured_at = (Get-Date).ToString('o')
    diagnostics = @($diagnostics)
    default_routes = @($routes)
} | ConvertTo-Json -Depth 5 | Set-Content -LiteralPath $outputPath -Encoding UTF8

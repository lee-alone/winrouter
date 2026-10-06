$ErrorActionPreference = 'Stop'
$files = @(
    @{ name='geosite-cn.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs' },
    @{ name='geoip-cn.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs' },
    @{ name='geosite-github.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-github.srs' },
    @{ name='geosite-cloudflare.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cloudflare.srs' },
    @{ name='geosite-geolocation-!cn.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-geolocation-!cn.srs' },
    @{ name='geosite-openai.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-openai.srs' },
    @{ name='geosite-anthropic.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-anthropic.srs' },
    @{ name='geosite-google.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-google.srs' },
    @{ name='geosite-youtube.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-youtube.srs' },
    @{ name='geosite-telegram.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-telegram.srs' },
    @{ name='geosite-steam.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-steam.srs' },
    @{ name='geosite-microsoft.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-microsoft.srs' },
    @{ name='geosite-apple.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-apple.srs' },
    @{ name='geosite-category-ads-all.srs'; url='https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-category-ads-all.srs' }
)

New-Item -ItemType Directory -Force -Path 'resources\geo' | Out-Null
foreach ($item in $files) {
    $dst = Join-Path 'resources\geo' $item.name
    Write-Host "Downloading $($item.name)..."
    & curl.exe -sSL -m 20 -o $dst $item.url
    if (!(Test-Path $dst) -or (Get-Item $dst).Length -lt 50) {
        throw "Failed to download $($item.name)"
    }
}
Write-Host "All $($files.Count) files downloaded successfully."

package srssets

type Preset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	URL      string `json:"url"`
	Upstream string `json:"upstream"`
	License  string `json:"license"`
}

var presets = []Preset{
	{ID: "sagernet-geosite-cn", Name: "中国域名 (geosite-cn)", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geoip-cn", Name: "中国 IP (geoip-cn)", Kind: "ip", URL: "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs", Upstream: "SagerNet/sing-geoip", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-github", Name: "GitHub", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-github.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-cloudflare", Name: "Cloudflare", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cloudflare.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-geolocation-!cn", Name: "非中国/被墙域名 (geolocation-!cn)", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-geolocation-!cn.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-openai", Name: "OpenAI / ChatGPT", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-openai.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-anthropic", Name: "Claude / Anthropic", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-anthropic.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-google", Name: "Google", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-google.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-youtube", Name: "YouTube", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-youtube.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-telegram", Name: "Telegram", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-telegram.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-steam", Name: "Steam", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-steam.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-microsoft", Name: "Microsoft", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-microsoft.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-apple", Name: "Apple", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-apple.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-category-ads-all", Name: "广告与隐私追踪过滤", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-category-ads-all.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
}

func Presets() []Preset { return append([]Preset(nil), presets...) }

func presetByID(id string) (Preset, bool) {
	for _, preset := range presets {
		if preset.ID == id {
			return preset, true
		}
	}
	return Preset{}, false
}

func defaultSources() []Source {
	result := make([]Source, 0, len(presets))
	for _, preset := range presets {
		action := "a"
		enabled := false
		switch preset.ID {
		case "sagernet-geosite-cn", "sagernet-geoip-cn":
			action = "a"
			enabled = true
		case "sagernet-geosite-github", "sagernet-geosite-cloudflare":
			action = "b"
			enabled = true
		case "sagernet-geosite-geolocation-!cn", "sagernet-geosite-openai":
			action = "c"
			enabled = true
		case "sagernet-geosite-anthropic", "sagernet-geosite-google", "sagernet-geosite-youtube", "sagernet-geosite-telegram":
			action = "c"
			enabled = false
		case "sagernet-geosite-steam", "sagernet-geosite-microsoft":
			action = "b"
			enabled = false
		case "sagernet-geosite-apple":
			action = "a"
			enabled = false
		case "sagernet-geosite-category-ads-all":
			action = "reject"
			enabled = false
		}
		result = append(result, Source{
			ID:       preset.ID,
			Name:     preset.Name,
			Kind:     preset.Kind,
			PresetID: preset.ID,
			URL:      preset.URL,
			Enabled:  enabled,
			Action:   action,
			Upstream: preset.Upstream,
			License:  preset.License,
		})
	}
	return result
}

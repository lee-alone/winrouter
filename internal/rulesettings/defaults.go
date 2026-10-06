package rulesettings

const DefaultPrivateLANRuleID = "default-private-lan"

// DefaultPrivateLANRule returns the built-in LAN IP rule.
func DefaultPrivateLANRule() Rule {
	return Rule{
		ID:   DefaultPrivateLANRuleID,
		Name: "常用内网地址 (局域网)",
		Type: "ip",
		Values: []string{
			"10.0.0.0/8",
			"172.16.0.0/12",
			"192.168.0.0/16",
			"100.64.0.0/10",
			"169.254.0.0/16",
			"fc00::/7",
			"fe80::/10",
		},
		Action:  "a",
		Enabled: true,
	}
}

// DefaultSingleProfile returns the factory default rule configuration for Single-NIC mode.
func DefaultSingleProfile() Profile {
	return Profile{
		DefaultOutbound:    "a",
		RuleUpdateOutbound: "auto",
		Rules: []Rule{
			DefaultPrivateLANRule(),
		},
		RuleOrder: []string{
			DefaultPrivateLANRuleID,
		},
		SRSActions: map[string]string{
			"sagernet-geosite-cn":             "a",
			"sagernet-geoip-cn":               "a",
			"sagernet-geosite-github":         "a", // Single-NIC direct
			"sagernet-geosite-cloudflare":     "a", // Single-NIC direct
			"sagernet-geosite-geolocation-!cn": "c",
			"sagernet-geosite-openai":         "c",
			"sagernet-geosite-anthropic":      "c",
			"sagernet-geosite-google":         "c",
			"sagernet-geosite-youtube":        "c",
			"sagernet-geosite-telegram":       "c",
			"sagernet-geosite-steam":          "a",
			"sagernet-geosite-microsoft":      "a",
			"sagernet-geosite-apple":          "a",
			"sagernet-geosite-category-ads-all": "reject",
		},
		SRSEnabled: map[string]bool{
			"sagernet-geosite-cn":             true,
			"sagernet-geoip-cn":               true,
			"sagernet-geosite-github":         true,
			"sagernet-geosite-cloudflare":     true,
			"sagernet-geosite-geolocation-!cn": false,
			"sagernet-geosite-openai":         false,
			"sagernet-geosite-anthropic":      false,
			"sagernet-geosite-google":         false,
			"sagernet-geosite-youtube":        false,
			"sagernet-geosite-telegram":       false,
			"sagernet-geosite-steam":          false,
			"sagernet-geosite-microsoft":      false,
			"sagernet-geosite-apple":          false,
			"sagernet-geosite-category-ads-all": false,
		},
	}
}

// DefaultDualProfile returns the factory default rule configuration for Dual-NIC mode.
func DefaultDualProfile() Profile {
	return Profile{
		DefaultOutbound:    "b",
		RuleUpdateOutbound: "auto",
		Rules: []Rule{
			DefaultPrivateLANRule(),
		},
		RuleOrder: []string{
			DefaultPrivateLANRuleID,
		},
		SRSActions: map[string]string{
			"sagernet-geosite-cn":             "a",
			"sagernet-geoip-cn":               "a",
			"sagernet-geosite-github":         "b",
			"sagernet-geosite-cloudflare":     "b",
			"sagernet-geosite-geolocation-!cn": "c",
			"sagernet-geosite-openai":         "c",
			"sagernet-geosite-anthropic":      "c",
			"sagernet-geosite-google":         "c",
			"sagernet-geosite-youtube":        "c",
			"sagernet-geosite-telegram":       "c",
			"sagernet-geosite-steam":          "b",
			"sagernet-geosite-microsoft":      "b",
			"sagernet-geosite-apple":          "a",
			"sagernet-geosite-category-ads-all": "reject",
		},
		SRSEnabled: map[string]bool{
			"sagernet-geosite-cn":             true,
			"sagernet-geoip-cn":               true,
			"sagernet-geosite-github":         true,
			"sagernet-geosite-cloudflare":     true,
			"sagernet-geosite-geolocation-!cn": true,
			"sagernet-geosite-openai":         true,
			"sagernet-geosite-anthropic":      false,
			"sagernet-geosite-google":         false,
			"sagernet-geosite-youtube":        false,
			"sagernet-geosite-telegram":       false,
			"sagernet-geosite-steam":          false,
			"sagernet-geosite-microsoft":      false,
			"sagernet-geosite-apple":          false,
			"sagernet-geosite-category-ads-all": false,
		},
	}
}

// Defaults returns default settings populated with both Single and Dual profiles.
func Defaults() Settings {
	single := DefaultSingleProfile()
	dual := DefaultDualProfile()
	return Settings{
		SchemaVersion:      SchemaVersion,
		Initialized:        false,
		ActiveMode:         ModeDual,
		Profiles: map[string]Profile{
			ModeSingle: single,
			ModeDual:   dual,
		},
		DefaultOutbound:    dual.DefaultOutbound,
		RuleUpdateOutbound: dual.RuleUpdateOutbound,
		Rules:              append([]Rule(nil), dual.Rules...),
		RuleOrder:          append([]string(nil), dual.RuleOrder...),
	}
}

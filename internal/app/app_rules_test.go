package app

import (
	"testing"

	"winrouter/internal/config"
	"winrouter/internal/rulesettings"
)

func TestApplyRuleSettingsExpandsMultiValueGroupsInOrder(t *testing.T) {
	input := config.MVPConfig{DefaultOutbound: "a", CustomRules: []config.MVPCustomRule{{ID: "old"}}}
	settings := rulesettings.Settings{
		SchemaVersion: rulesettings.SchemaVersion, Initialized: true, DefaultOutbound: "b",
		Rules: []rulesettings.Rule{
			{ID: "domains", Name: "Domains", Type: "domain-suffix", Values: []string{"one.example", "two.example"}, Action: "b", Enabled: true},
			{ID: "disabled", Name: "Disabled", Type: "ip", Values: []string{"203.0.113.0/24"}, Action: "a", Enabled: false},
		},
		RuleOrder: []string{"srs:sagernet-geosite-cn", "domains"},
	}
	result := applyRuleSettings(input, settings)
	if result.DefaultOutbound != "b" || len(result.CustomRules) != 2 || result.CustomRules[0].Value != "one.example" || result.CustomRules[1].Value != "two.example" {
		t.Fatalf("applied settings = %#v", result)
	}
	if result.CustomRules[0].ID != "domains" || result.CustomRules[1].Action != "b" {
		t.Fatalf("group metadata was not preserved: %#v", result.CustomRules)
	}
}

func TestApplyRuleSettingsPreservesInputWhenUninitialized(t *testing.T) {
	input := config.MVPConfig{DefaultOutbound: "a", CustomRules: []config.MVPCustomRule{{ID: "existing"}}}
	result := applyRuleSettings(input, rulesettings.Defaults())
	if result.DefaultOutbound != "a" || len(result.CustomRules) != 1 || result.CustomRules[0].ID != "existing" {
		t.Fatalf("uninitialized settings changed input: %#v", result)
	}
}

func TestApplyProfileToConfigSingleNICMapsBToAAndSetsDefaultA(t *testing.T) {
	input := config.MVPConfig{Mode: "single"}
	profile := rulesettings.Profile{
		DefaultOutbound: "b", // Even if profile had 'b', single-NIC must normalize to 'a'
		Rules: []rulesettings.Rule{
			{ID: "legacy-b", Name: "Legacy B", Type: "domain", Values: []string{"example.com"}, Action: "b", Enabled: true},
			{ID: "direct-a", Name: "Direct A", Type: "domain", Values: []string{"local.cn"}, Action: "a", Enabled: true},
		},
		RuleOrder: []string{"legacy-b", "direct-a"},
	}
	result := applyProfileToConfig(input, profile, "single")
	if result.DefaultOutbound != "a" {
		t.Fatalf("expected single-NIC default outbound to be 'a', got %q", result.DefaultOutbound)
	}
	if len(result.CustomRules) != 2 {
		t.Fatalf("expected 2 custom rules, got %d", len(result.CustomRules))
	}
	if result.CustomRules[0].Action != "a" {
		t.Fatalf("expected rule legacy-b action 'b' to be mapped to 'a' in single mode, got %q", result.CustomRules[0].Action)
	}
	if result.CustomRules[1].Action != "a" {
		t.Fatalf("expected rule direct-a action to be 'a', got %q", result.CustomRules[1].Action)
	}
}

func TestApplyProfileToConfigDualNICPreservesOutboundB(t *testing.T) {
	input := config.MVPConfig{Mode: "dual"}
	profile := rulesettings.Profile{
		DefaultOutbound: "b",
		Rules: []rulesettings.Rule{
			{ID: "dual-b", Name: "Dual B", Type: "domain", Values: []string{"example.com"}, Action: "b", Enabled: true},
		},
		RuleOrder: []string{"dual-b"},
	}
	result := applyProfileToConfig(input, profile, "dual")
	if result.DefaultOutbound != "b" {
		t.Fatalf("expected dual-NIC default outbound to be 'b', got %q", result.DefaultOutbound)
	}
	if len(result.CustomRules) != 1 || result.CustomRules[0].Action != "b" {
		t.Fatalf("expected dual-NIC to preserve action 'b': %#v", result.CustomRules)
	}
}


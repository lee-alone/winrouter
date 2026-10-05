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

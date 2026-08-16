package main

import (
	"path/filepath"
	"testing"

	"winrouter/internal/config"
	"winrouter/internal/rulesettings"
)

func TestApplyRuleSettingsUsesInitializedStore(t *testing.T) {
	input := config.MVPConfig{DefaultOutbound: "a", CustomRules: []config.MVPCustomRule{{ID: "old", Name: "Old", Type: "domain", Value: "old.example", Action: "a"}}, RuleOrder: []string{"old"}}
	settings := rulesettings.Settings{
		SchemaVersion:   rulesettings.SchemaVersion,
		Initialized:     true,
		DefaultOutbound: "b",
		Rules: []rulesettings.Rule{
			{ID: "enabled", Name: "Enabled", Type: "domain", Value: "example.com", Action: "b", Enabled: true},
			{ID: "disabled", Name: "Disabled", Type: "ip", Value: "203.0.113.0/24", Action: "a", Enabled: false},
		},
		RuleOrder: []string{"srs:sagernet-geosite-cn", "enabled"},
	}

	result := applyRuleSettings(input, settings)
	if result.DefaultOutbound != "b" || len(result.CustomRules) != 1 || result.CustomRules[0].ID != "enabled" || len(result.RuleOrder) != 2 {
		t.Fatalf("applied settings = %#v", result)
	}
	settings.RuleOrder[0] = "mutated"
	if result.RuleOrder[0] != "srs:sagernet-geosite-cn" {
		t.Fatal("rule order was not cloned")
	}
}

func TestApplyRuleSettingsPreservesInputBeforeMigration(t *testing.T) {
	input := config.MVPConfig{DefaultOutbound: "a", CustomRules: []config.MVPCustomRule{{ID: "legacy"}}}
	result := applyRuleSettings(input, rulesettings.Defaults())
	if result.DefaultOutbound != "a" || len(result.CustomRules) != 1 || result.CustomRules[0].ID != "legacy" {
		t.Fatalf("uninitialized settings changed input: %#v", result)
	}
}

func TestWithRemoteRulesSkipsMigratedLegacySource(t *testing.T) {
	manager, err := rulesettings.New(filepath.Join(t.TempDir(), "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	settings := rulesettings.Defaults()
	settings.LegacyRemoteMigrated = true
	if _, err := manager.Configure(settings); err != nil {
		t.Fatal(err)
	}
	app := &App{ruleSettings: manager}
	input := config.MVPConfig{CustomRules: []config.MVPCustomRule{{ID: "kept"}}}
	result, err := app.withRemoteRules(input)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.CustomRules) != 1 || result.CustomRules[0].ID != "kept" {
		t.Fatalf("migrated legacy source changed input: %#v", result)
	}
}

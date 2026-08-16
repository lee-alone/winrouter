package rulesettings

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestConfigurePersistsMultiValueRulesAndClones(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	m, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := Defaults()
	settings.Rules = []Rule{{ID: "browser", Name: "Browser", Type: "process-name", Values: []string{"browser.exe", "helper.exe"}, Action: "b", Enabled: true}}
	settings.RuleOrder = []string{"srs:sagernet-geosite-cn", "browser"}
	stored, err := m.Configure(settings)
	if err != nil {
		t.Fatal(err)
	}
	stored.Rules[0].Values[0] = "mutated.exe"
	reloaded, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	got := reloaded.Get()
	if !got.Initialized || len(got.Rules[0].Values) != 2 || got.Rules[0].Values[0] != "browser.exe" {
		t.Fatalf("reloaded settings = %#v", got)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"value"`) || !strings.Contains(string(data), `"values"`) {
		t.Fatalf("unexpected persisted schema: %s", data)
	}
}

func TestNewRejectsLegacyValueField(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	data := `{"schema_version":2,"initialized":true,"default_outbound":"b","rules":[{"id":"legacy","name":"Legacy","type":"domain","value":"example.com","action":"a","enabled":true}],"rule_order":[]}`
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path); err == nil || !strings.Contains(err.Error(), "legacy rule settings format") {
		t.Fatalf("legacy value field error = %v", err)
	}
}

func TestValidationRejectsDuplicateValuesAndActionConflicts(t *testing.T) {
	duplicate := Defaults()
	duplicate.Rules = []Rule{{ID: "same", Name: "Same", Type: "domain-suffix", Values: []string{"Example.COM.", "example.com"}, Action: "a", Enabled: true}}
	if err := Validate(duplicate); err == nil {
		t.Fatal("semantic duplicate accepted")
	}
	conflict := Defaults()
	conflict.Rules = []Rule{
		{ID: "one", Name: "One", Type: "domain", Values: []string{"example.com"}, Action: "a", Enabled: true},
		{ID: "two", Name: "Two", Type: "domain", Values: []string{"Example.COM."}, Action: "b", Enabled: true},
	}
	if err := Validate(conflict); err == nil {
		t.Fatal("conflicting actions accepted")
	}
}

func TestGetAlwaysReturnsArrayBackedCollections(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := m.Get()
	if got.Rules == nil || got.RuleOrder == nil {
		t.Fatalf("empty collections must serialize as arrays: %#v", got)
	}
}

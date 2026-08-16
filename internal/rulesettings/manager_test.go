package rulesettings

import (
	"fmt"
	"path/filepath"
	"testing"
)

func TestConfigurePersistsAndClones(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	m, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Get().Initialized {
		t.Fatal("new settings unexpectedly initialized")
	}
	settings := Defaults()
	settings.Rules = []Rule{{ID: "browser", Name: "Browser", Type: "process-name", Value: "browser.exe", Action: "b", Enabled: true}}
	settings.RuleOrder = []string{"srs:sagernet-geosite-cn", "browser"}
	stored, err := m.Configure(settings)
	if err != nil {
		t.Fatal(err)
	}
	stored.Rules[0].Name = "mutated"
	reloaded, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get(); !got.Initialized || got.Rules[0].Name != "Browser" || got.RuleOrder[0] != "srs:sagernet-geosite-cn" {
		t.Fatalf("reloaded settings = %#v", got)
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

func TestValidationRejectsInvalidAndDuplicateValues(t *testing.T) {
	tests := []Settings{Defaults(), Defaults(), Defaults(), Defaults()}
	tests[0].DefaultOutbound = "proxy"
	tests[1].Rules = []Rule{{ID: "bad id", Name: "Bad", Type: "domain", Value: "example.com", Action: "a"}}
	tests[2].Rules = []Rule{{ID: "same", Name: "One", Type: "domain", Value: "one.example", Action: "a"}, {ID: "same", Name: "Two", Type: "domain", Value: "two.example", Action: "b"}}
	tests[3].RuleOrder = []string{"same", "same"}
	for index, value := range tests {
		if err := Validate(value); err == nil {
			t.Fatalf("case %d accepted", index)
		}
	}
}

func TestMigrateLegacyRemoteIsAtomicAndIdempotent(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	first, err := m.MigrateLegacyRemote([]Rule{{ID: "legacy-remote-001", Name: "Imported rule 1", Type: "domain", Value: "example.com", Action: "b", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	second, err := m.MigrateLegacyRemote([]Rule{{ID: "legacy-remote-002", Name: "Imported rule 2", Type: "ip", Value: "203.0.113.0/24", Action: "a", Enabled: true}})
	if err != nil {
		t.Fatal(err)
	}
	if !first.LegacyRemoteMigrated || len(second.Rules) != 1 || second.Rules[0].ID != "legacy-remote-001" {
		t.Fatalf("migration was not idempotent: %#v", second)
	}
}

func TestMigrateLegacyRemoteRejectsOverflowWithoutChangingSettings(t *testing.T) {
	m, _ := New(filepath.Join(t.TempDir(), "rules.json"))
	settings := Defaults()
	for index := 0; index < 200; index++ {
		settings.Rules = append(settings.Rules, Rule{ID: fmt.Sprintf("rule-%03d", index), Name: "Existing", Type: "domain", Value: fmt.Sprintf("%d.example", index), Action: "a", Enabled: true})
	}
	if _, err := m.Configure(settings); err != nil {
		t.Fatal(err)
	}
	if _, err := m.MigrateLegacyRemote([]Rule{{ID: "legacy-remote-001", Name: "Imported", Type: "domain", Value: "example.com", Action: "b", Enabled: true}}); err == nil {
		t.Fatal("overflow migration accepted")
	}
	if got := m.Get(); got.LegacyRemoteMigrated || len(got.Rules) != 200 {
		t.Fatalf("failed migration changed settings: %#v", got)
	}
}

func TestMigrationRejectsSemanticConflictWithoutChangingSettings(t *testing.T) {
	m, _ := New(filepath.Join(t.TempDir(), "rules.json"))
	settings := Defaults()
	settings.Rules = []Rule{{ID: "existing", Name: "Existing", Type: "domain", Value: "Example.COM.", Action: "a", Enabled: true}}
	if _, err := m.Configure(settings); err != nil {
		t.Fatal(err)
	}
	_, err := m.MigrateLegacyRemote([]Rule{{ID: "legacy-remote-001", Name: "Imported", Type: "domain", Value: "example.com", Action: "b", Enabled: true}})
	if err == nil {
		t.Fatal("semantic conflict accepted")
	}
	if got := m.Get(); got.LegacyRemoteMigrated || len(got.Rules) != 1 {
		t.Fatalf("failed conflict migration changed settings: %#v", got)
	}
}

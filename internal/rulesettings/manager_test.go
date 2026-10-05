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

func TestNormalizeIPAndCIDRValues(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		{"1.1.1.1", "1.1.1.1/32", false},
		{"192.168.1.0/24", "192.168.1.0/24", false},
		{"192.168.1.5/24", "192.168.1.0/24", false},
		{"2001:db8::1", "2001:db8::1/128", false},
		{"2001:db8::/32", "2001:db8::/32", false},
		{"2001:db8:1234::1/64", "2001:db8:1234::/64", false},
		{"0.0.0.0", "", true},
		{"::", "", true},
		{"224.0.0.1", "", true},
		{"ff02::1", "", true},
		{"not-an-ip", "", true},
	}
	for _, tc := range tests {
		got, err := normalizeValue("ip", tc.input)
		if (err != nil) != tc.wantErr {
			t.Errorf("normalizeValue(ip, %q) err = %v, wantErr = %v", tc.input, err, tc.wantErr)
			continue
		}
		if !tc.wantErr && got != tc.want {
			t.Errorf("normalizeValue(ip, %q) = %q, want %q", tc.input, got, tc.want)
		}
	}
}

func TestConfigureNormalizesIPv6Rules(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	m, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := Defaults()
	settings.Rules = []Rule{
		{
			ID: "v6-rule", Name: "IPv6 Rule", Type: "ip",
			Values: []string{"2001:db8::1", "2001:db8:abcd::1/48"},
			Action: "b", Enabled: true,
		},
	}
	stored, err := m.Configure(settings)
	if err != nil {
		t.Fatalf("Configure() error: %v", err)
	}
	if len(stored.Rules[0].Values) != 2 || stored.Rules[0].Values[0] != "2001:db8::1/128" || stored.Rules[0].Values[1] != "2001:db8:abcd::/48" {
		t.Fatalf("unexpected normalized values: %#v", stored.Rules[0].Values)
	}
}

func TestDefaultsIncludePrivateLANAndRuleUpdateOutbound(t *testing.T) {
	d := Defaults()
	if d.RuleUpdateOutbound != "auto" {
		t.Fatalf("default RuleUpdateOutbound = %q, want auto", d.RuleUpdateOutbound)
	}
	if len(d.Rules) == 0 || d.Rules[0].ID != "default-private-lan" {
		t.Fatalf("defaults must include default-private-lan rule, got %#v", d.Rules)
	}
	if d.Rules[0].Action != "a" || len(d.Rules[0].Values) != 2 {
		t.Fatalf("default private lan rule action/values = %#v", d.Rules[0])
	}
	m, err := New(filepath.Join(t.TempDir(), "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := m.Get()
	if len(got.Rules) == 0 || got.Rules[0].ID != "default-private-lan" {
		t.Fatalf("new manager must initialize with default-private-lan, got %#v", got.Rules)
	}
}

func TestRuleOrderSupportsExclamationMarkSRSKeys(t *testing.T) {
	settings := Defaults()
	settings.RuleOrder = []string{"srs:sagernet-geosite-geolocation-!cn", "default-private-lan"}
	if err := Validate(settings); err != nil {
		t.Fatalf("Validate failed for order key with exclamation mark: %v", err)
	}

	m, err := New(filepath.Join(t.TempDir(), "rules.json"))
	if err != nil {
		t.Fatal(err)
	}
	stored, err := m.Configure(settings)
	if err != nil {
		t.Fatalf("Configure failed: %v", err)
	}
	if len(stored.RuleOrder) != 2 || stored.RuleOrder[0] != "srs:sagernet-geosite-geolocation-!cn" {
		t.Fatalf("unexpected stored rule order: %#v", stored.RuleOrder)
	}
}


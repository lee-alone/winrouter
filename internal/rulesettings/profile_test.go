package rulesettings

import (
	"path/filepath"
	"testing"
)

func TestProfileIndependenceAndValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "rules.json")
	m, err := New(path)
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}

	single := m.GetProfile(ModeSingle)
	if single.DefaultOutbound != "a" {
		t.Fatalf("expected single profile default outbound 'a', got %q", single.DefaultOutbound)
	}

	dual := m.GetProfile(ModeDual)
	if dual.DefaultOutbound != "b" {
		t.Fatalf("expected dual profile default outbound 'b', got %q", dual.DefaultOutbound)
	}

	// Single profile rejects action 'b'
	invalidSingle := single.Clone()
	invalidSingle.Rules = append(invalidSingle.Rules, Rule{
		ID:      "test-b",
		Name:    "Test B",
		Type:    "domain",
		Values:  []string{"example.com"},
		Action:  "b",
		Enabled: true,
	})
	if err := invalidSingle.Validate(ModeSingle); err == nil {
		t.Fatal("expected single profile to reject rule with action 'b'")
	}

	// Dual profile accepts action 'b'
	if err := invalidSingle.Validate(ModeDual); err != nil {
		t.Fatalf("expected dual profile to accept rule with action 'b', got: %v", err)
	}

	// Updating single profile does not affect dual profile
	validSingle := single.Clone()
	validSingle.DefaultOutbound = "c"
	validSingle.Rules = append(validSingle.Rules, Rule{
		ID:      "test-c",
		Name:    "Test C",
		Type:    "domain",
		Values:  []string{"proxy.example.com"},
		Action:  "c",
		Enabled: true,
	})
	if _, err := m.SetProfile(ModeSingle, validSingle); err != nil {
		t.Fatalf("SetProfile(single) failed: %v", err)
	}

	reloaded, err := New(path)
	if err != nil {
		t.Fatalf("reloaded New() failed: %v", err)
	}
	reloadedSingle := reloaded.GetProfile(ModeSingle)
	if reloadedSingle.DefaultOutbound != "c" || len(reloadedSingle.Rules) != 2 {
		t.Fatalf("single profile was not persisted independently: %#v", reloadedSingle)
	}

	reloadedDual := reloaded.GetProfile(ModeDual)
	if reloadedDual.DefaultOutbound != "b" || len(reloadedDual.Rules) != 1 {
		t.Fatalf("dual profile was mutated unexpectedly: %#v", reloadedDual)
	}
}

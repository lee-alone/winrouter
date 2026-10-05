package app

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"winrouter/internal/dnssettings"
	"winrouter/internal/interfacemanager"
	"winrouter/internal/rulesettings"
)

func TestRebuildApplicationSettingsReplacesInvalidFiles(t *testing.T) {
	directory := t.TempDir()
	for _, name := range []string{"interfaces.json", "rule-settings.json", "dns-settings.json"} {
		if err := os.WriteFile(filepath.Join(directory, name), []byte("not json"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	if err := rebuildApplicationSettings(directory); err != nil {
		t.Fatal(err)
	}
	state, err := interfacemanager.LoadState(filepath.Join(directory, "interfaces.json"))
	if err != nil || state.InterfaceA.GUID != "" || state.InterfaceB.GUID != "" || state.IPv6Policy != interfacemanager.IPv6PolicyBlock {
		t.Fatalf("interface defaults = %#v, %v", state, err)
	}
	expectedRules := rulesettings.Defaults()
	expectedRules.Initialized = true
	rules, err := rulesettings.New(filepath.Join(directory, "rule-settings.json"))
	if err != nil || !reflect.DeepEqual(rules.Get(), expectedRules) {
		t.Fatalf("rule defaults were not rebuilt: %#v, %v", rules, err)
	}
	dns, err := dnssettings.New(filepath.Join(directory, "dns-settings.json"))
	if err != nil || !reflect.DeepEqual(dns.Get(), dnssettings.Defaults()) {
		t.Fatalf("DNS defaults = %#v, %v", dns, err)
	}
}

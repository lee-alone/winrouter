package tunprefix

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "tun-prefix.json")
	if err := SaveState(path, "172.19.0.1/30"); err != nil {
		t.Fatal(err)
	}
	state, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.Prefix != "172.19.0.0/30" {
		t.Fatalf("prefix = %q", state.Prefix)
	}
	if err := SaveState(path, "172.19.0.4/30"); err != nil {
		t.Fatal(err)
	}
	updated, err := LoadState(path)
	if err != nil || updated.Prefix != "172.19.0.4/30" {
		t.Fatalf("updated state = %#v, %v", updated, err)
	}
	if info, err := os.Stat(path); err != nil || info.IsDir() {
		t.Fatalf("state file missing: %v", err)
	}
}

func TestLoadStateHandlesMissingAndInvalidFiles(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.json")
	if state, err := LoadState(missing); err != nil || state.Prefix != "" {
		t.Fatalf("LoadState(missing) = %#v, %v", state, err)
	}
	invalid := filepath.Join(t.TempDir(), "invalid.json")
	if err := os.WriteFile(invalid, []byte(`{"prefix":"8.8.8.0/24"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(invalid); err == nil {
		t.Fatal("LoadState(invalid) succeeded")
	}
}

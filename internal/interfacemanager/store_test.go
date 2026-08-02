package interfacemanager

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"winrouter/internal/interfaces"
)

func TestStateRoundTripAndAtomicReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state", "interfaces.json")
	first := State{InterfaceA: interfaces.Identity{GUID: "{A}"}, TUNPrefix: "172.19.0.0/30"}
	if err := SaveState(path, first); err != nil {
		t.Fatal(err)
	}
	second := State{InterfaceA: interfaces.Identity{GUID: "{B}"}, TUNPrefix: "172.19.0.4/30"}
	if err := SaveState(path, second); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.SchemaVersion != StateSchemaVersion || loaded.InterfaceA.GUID != "{B}" || loaded.TUNPrefix != second.TUNPrefix {
		t.Fatalf("loaded = %#v", loaded)
	}
}

func TestLoadStateRejectsUnknownSchemaAndMalformedJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":99}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("LoadState() accepted unknown schema")
	}
	if err := os.WriteFile(path, []byte(`{`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("LoadState() accepted malformed JSON")
	}
}

func TestLoadStateMigratesVersionOneOnlyAfterValidation(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interfaces.json")
	legacy := []byte(`{"schema_version":1,"interface_a":{"guid":" {A} "},"interface_b":{"guid":"{B}"},"tun_prefix":"172.19.0.0/30"}`)
	if err := os.WriteFile(path, legacy, 0o600); err != nil {
		t.Fatal(err)
	}
	state, err := LoadState(path)
	if err != nil {
		t.Fatal(err)
	}
	if state.SchemaVersion != StateSchemaVersion || state.InterfaceA.GUID != "{A}" {
		t.Fatalf("migrated state = %#v", state)
	}
	committed, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var envelope struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(committed, &envelope); err != nil || envelope.SchemaVersion != StateSchemaVersion {
		t.Fatalf("committed migration = %s, %v", committed, err)
	}
}

func TestFailedMigrationPreservesOriginalAndPreventsManagerWrites(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interfaces.json")
	original := []byte(`{"schema_version":1,"interface_a":{"guid":"{A}"},"interface_b":{"guid":"{B}"},"tun_prefix":"not-a-prefix"}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(Options{StatePath: path}); err == nil {
		t.Fatal("New() accepted a failed state migration")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("failed migration overwrote original: %s", after)
	}
}

func TestUnknownSchemaPreservesOriginal(t *testing.T) {
	path := filepath.Join(t.TempDir(), "interfaces.json")
	original := []byte(`{"schema_version":99,"future":"value"}`)
	if err := os.WriteFile(path, original, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadState(path); err == nil {
		t.Fatal("LoadState() accepted an unknown schema")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, original) {
		t.Fatalf("unknown schema was overwritten: %s", after)
	}
}

package dnssettings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsPersistAndReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dns.json")
	m, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	settings := m.Get()
	settings.Global = Server{PresetID: "cloudflare-doh", Type: "https", Server: "1.1.1.1", Port: 443, ServerName: "cloudflare-dns.com"}
	if _, err = m.Configure(settings); err != nil {
		t.Fatal(err)
	}
	settings.Global = Server{PresetID: "google-udp", Type: "udp", Server: "8.8.8.8", Port: 53}
	if _, err = m.Configure(settings); err != nil {
		t.Fatalf("replace existing settings: %v", err)
	}
	reloaded, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Get().Global; got.Type != "udp" || got.Server != "8.8.8.8" {
		t.Fatalf("global = %#v", got)
	}
	if info, _ := os.Stat(path); info == nil {
		t.Fatal("settings file not written")
	}
}

func TestValidationBoundaries(t *testing.T) {
	tests := []Settings{Defaults(), Defaults(), Defaults(), Defaults()}
	tests[0].Domestic.Server = "dns.example"
	tests[1].Domestic.Port = 0
	tests[2].Global = tests[2].Domestic
	tests[3].Global = Server{Type: "https", Server: "1.1.1.1", Port: 443, ServerName: "https://bad/name"}
	for i, value := range tests {
		if err := Validate(value); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
}

func TestSchemaZeroMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dns.json")
	if err := os.WriteFile(path, []byte(`{"domestic":{"type":"udp","server":"223.5.5.5","port":53},"global":{"type":"udp","server":"8.8.8.8","port":53}}`), 0600); err != nil {
		t.Fatal(err)
	}
	m, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	if m.Get().SchemaVersion != SchemaVersion {
		t.Fatal("schema was not migrated")
	}
}

package nodes

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

type testProtector struct{}

func (testProtector) Protect(value []byte) ([]byte, error) {
	return append([]byte("protected:"), value...), nil
}
func (testProtector) Unprotect(value []byte) ([]byte, error) {
	return bytes.TrimPrefix(value, []byte("protected:")), nil
}

func TestStoreCRUDSelectionAndSecretRedaction(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.json")
	store, err := New(path, testProtector{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.Add(Input{Name: "Primary", Type: TypeHTTP, Server: "203.0.113.10", Port: 8080, Username: "alice", Password: "secret-value"})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Selected || !first.HasPassword {
		t.Fatalf("first node = %#v", first)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("secret-value")) {
		t.Fatalf("plaintext password persisted: %s", data)
	}
	second, err := store.Add(Input{Name: "Backup", Type: TypeHTTP, Server: "198.51.100.20", Port: 3128})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Select(second.ID); err != nil {
		t.Fatal(err)
	}
	updated, err := store.Update(Input{ID: second.ID, Name: "Backup edited", Type: TypeHTTP, Server: second.Server, Port: second.Port})
	if err != nil || updated.Name != "Backup edited" || !updated.Selected {
		t.Fatalf("updated = %#v, %v", updated, err)
	}
	cleared, err := store.Update(Input{ID: first.ID, Name: first.Name, Type: first.Type, Server: first.Server, Port: first.Port, Username: first.Username, ClearPassword: true})
	if err != nil || cleared.HasPassword {
		t.Fatalf("cleared password = %#v, %v", cleared, err)
	}
	if _, err := store.Update(Input{ID: first.ID, Name: first.Name, Type: first.Type, Server: first.Server, Port: first.Port, Password: "new", ClearPassword: true}); err == nil {
		t.Fatal("password and clear_password accepted together")
	}
	if _, err := store.Update(Input{ID: first.ID, Name: first.Name, Type: first.Type, Server: first.Server, Port: first.Port, Username: first.Username, Password: "secret-value"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Delete(second.ID); err != nil {
		t.Fatal(err)
	}
	items := store.List()
	if len(items) != 1 || items[0].ID != first.ID || !items[0].Selected {
		t.Fatalf("items = %#v", items)
	}
	node, credentials, err := store.SelectedCredentials()
	if err != nil || node.ID != first.ID || credentials.Username != "alice" || credentials.Password != "secret-value" {
		t.Fatalf("selected = %#v, %#v, %v", node, credentials, err)
	}
}

func TestStoreRejectsInvalidNodesAndCorruptSelection(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "nodes.json"), testProtector{})
	if err != nil {
		t.Fatal(err)
	}
	for index, input := range []Input{
		{Name: "", Type: TypeHTTP, Server: "203.0.113.10", Port: 80},
		{Name: "bad type", Type: "socks", Server: "203.0.113.10", Port: 80},
		{Name: "bad domain", Type: TypeHTTP, Server: "not a domain", Port: 80},
		{Name: "loopback", Type: TypeHTTP, Server: "127.0.0.1", Port: 80},
	} {
		if _, err := store.Add(input); err == nil {
			t.Fatalf("invalid case %d accepted", index)
		}
	}
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":1,"selected_id":"missing","nodes":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := New(path, testProtector{}); err == nil {
		t.Fatal("corrupt selected id accepted")
	}
}

func TestStoreCommitsResolvedIPAtomicallyAndPreservesItOnMetadataEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.json")
	store, err := New(path, testProtector{})
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.Add(Input{Name: "Domain", Type: TypeHTTP, Server: "proxy.example.test", Port: 8080})
	if err != nil {
		t.Fatal(err)
	}
	if item.ResolvedIP != "" {
		t.Fatalf("initial resolved IP = %q", item.ResolvedIP)
	}
	item, err = store.CommitResolvedIP(item.ID, "203.0.113.44")
	if err != nil || item.ResolvedIP != "203.0.113.44" {
		t.Fatalf("commit = %#v, %v", item, err)
	}
	item, err = store.Update(Input{ID: item.ID, Name: "Renamed", Type: TypeHTTP, Server: "proxy.example.test", Port: 8080})
	if err != nil || item.ResolvedIP != "203.0.113.44" {
		t.Fatalf("metadata update = %#v, %v", item, err)
	}
	if _, err := store.CommitResolvedIP(item.ID, "127.0.0.1"); err == nil {
		t.Fatal("loopback resolution accepted")
	}
	loaded, err := New(path, testProtector{})
	if err != nil || loaded.List()[0].ResolvedIP != "203.0.113.44" {
		t.Fatalf("reloaded = %#v, %v", loaded.List(), err)
	}
}

func TestReplaceSubscriptionIsAllOrNothing(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "nodes.json"), testProtector{})
	if err != nil {
		t.Fatal(err)
	}
	first, err := store.ReplaceSubscription("sub-1", []Input{{Name: "One", Type: TypeHTTP, Server: "203.0.113.1", Port: 80, Password: "first"}})
	if err != nil || len(first) != 1 {
		t.Fatalf("first = %#v, %v", first, err)
	}
	if _, err := store.SetFavorite(first[0].ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ReplaceSubscription("sub-1", []Input{{Name: "Invalid", Type: "command", Server: "203.0.113.2", Port: 80}}); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	items := store.List()
	if len(items) != 1 || items[0].Name != "One" || items[0].SubscriptionID != "sub-1" {
		t.Fatalf("rollback items = %#v", items)
	}
	second, err := store.ReplaceSubscription("sub-1", []Input{{Name: "Two", Type: TypeHTTP, Server: "proxy.example.test", Port: 8080}})
	if err != nil || len(second) != 1 || second[0].Name != "Two" || !second[0].Selected || second[0].Favorite {
		t.Fatalf("second = %#v, %v", second, err)
	}
}

func TestFavoritePersistsAndSubscriptionRefreshPreservesEndpoint(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.json")
	store, err := New(path, testProtector{})
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.ReplaceSubscription("sub", []Input{{Name: "First name", Type: TypeHTTP, Server: "203.0.113.8", Port: 8080}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.SetFavorite(items[0].ID, true); err != nil {
		t.Fatal(err)
	}
	items, err = store.ReplaceSubscription("sub", []Input{{Name: "Renamed", Type: TypeHTTP, Server: "203.0.113.8", Port: 8080}})
	if err != nil || !items[0].Favorite {
		t.Fatalf("refreshed favorite = %#v, %v", items, err)
	}
	loaded, err := New(path, testProtector{})
	if err != nil || !loaded.List()[0].Favorite {
		t.Fatalf("loaded favorite = %#v, %v", loaded.List(), err)
	}
}

func TestNodeSchemaOneMigratesWithoutLosingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.json")
	data := []byte(`{"schema_version":1,"selected_id":"one","nodes":[{"id":"one","name":"Legacy","type":"http","server":"203.0.113.9","port":8080}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(path, testProtector{})
	if err != nil || len(store.List()) != 1 {
		t.Fatalf("migration = %#v, %v", store, err)
	}
	migrated, _ := os.ReadFile(path)
	if !bytes.Contains(migrated, []byte(`"schema_version": 2`)) {
		t.Fatalf("migrated state = %s", migrated)
	}
}

func TestDPAPIRoundTrip(t *testing.T) {
	protector := DPAPIProtector{}
	ciphertext, err := protector.Protect([]byte("winrouter-node-secret"))
	if err != nil {
		t.Skipf("DPAPI unavailable: %v", err)
	}
	plaintext, err := protector.Unprotect(ciphertext)
	if err != nil || string(plaintext) != "winrouter-node-secret" {
		t.Fatalf("DPAPI round trip = %q, %v", plaintext, err)
	}
}

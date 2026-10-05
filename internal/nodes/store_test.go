package nodes

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
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
	first, err := store.Add(Input{
		Name:           "Primary",
		Type:           TypeHTTP,
		Server:         "203.0.113.10",
		Port:           8080,
		Egress:         EgressA,
		Authentication: AuthenticationInput{Username: "alice", Password: "secret-value"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !first.Selected || !first.HasSecret || first.Egress != EgressA {
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
	if second.Egress != EgressB {
		t.Fatalf("default egress expected b, got %s", second.Egress)
	}
	if _, err := store.Select(second.ID); err != nil {
		t.Fatal(err)
	}
	updated, err := store.Update(Input{ID: second.ID, Name: "Backup edited", Type: TypeHTTP, Server: second.Server, Port: second.Port, Egress: EgressB})
	if err != nil || updated.Name != "Backup edited" || !updated.Selected {
		t.Fatalf("updated = %#v, %v", updated, err)
	}
	cleared, err := store.Update(Input{ID: first.ID, Name: first.Name, Type: first.Type, Server: first.Server, Port: first.Port, Authentication: AuthenticationInput{Username: first.Username}, ClearSecret: true})
	if err != nil || cleared.HasSecret {
		t.Fatalf("cleared secret = %#v, %v", cleared, err)
	}
	if _, err := store.Update(Input{ID: first.ID, Name: first.Name, Type: first.Type, Server: first.Server, Port: first.Port, Authentication: AuthenticationInput{Password: "new"}, ClearSecret: true}); err == nil {
		t.Fatal("password and clear_secret accepted together")
	}
	if _, err := store.Update(Input{ID: first.ID, Name: first.Name, Type: first.Type, Server: first.Server, Port: first.Port, Authentication: AuthenticationInput{Username: "alice", Password: "secret-value"}}); err != nil {
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

func TestStoreMultiProtocolInputs(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.json")
	store, err := New(path, testProtector{})
	if err != nil {
		t.Fatal(err)
	}

	// VMess node
	vmessNode, err := store.Add(Input{
		Name:           "VMess-1",
		Type:           TypeVMess,
		Server:         "203.0.113.15",
		Port:           443,
		Authentication: AuthenticationInput{UUID: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"},
		TLS:            &TLSInput{Enabled: true, ServerName: "example.com"},
		Transport:      &TransportInput{Type: "ws", Path: "/chat"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !vmessNode.HasSecret || vmessNode.TLS == nil || !vmessNode.TLS.Enabled || vmessNode.Transport == nil || vmessNode.Transport.Type != "ws" {
		t.Fatalf("vmessNode = %#v", vmessNode)
	}
	_, creds, err := store.Credentials(vmessNode.ID)
	if err != nil || creds.UUID != "a8e678c0-8903-4402-8e99-20aadf1a7cd1" {
		t.Fatalf("vmess creds = %#v, %v", creds, err)
	}

	// Shadowsocks node
	ssNode, err := store.Add(Input{
		Name:           "SS-1",
		Type:           TypeShadowsocks,
		Server:         "203.0.113.16",
		Port:           8388,
		Authentication: AuthenticationInput{Method: "aes-256-gcm", Password: "sspassword"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ssNode.Authentication.Method != "aes-256-gcm" || ssNode.Username != "aes-256-gcm" {
		t.Fatalf("ssNode = %#v", ssNode)
	}
	_, ssCreds, err := store.Credentials(ssNode.ID)
	if err != nil || ssCreds.Method != "aes-256-gcm" || ssCreds.Password != "sspassword" {
		t.Fatalf("ss creds = %#v, %v", ssCreds, err)
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
		{Name: "bad egress", Type: TypeHTTP, Server: "203.0.113.10", Port: 80, Egress: "c"},
		{Name: "bad domain", Type: TypeHTTP, Server: "not a domain", Port: 80},
		{Name: "loopback", Type: TypeHTTP, Server: "127.0.0.1", Port: 80},
		{Name: "bad ss method", Type: TypeShadowsocks, Server: "203.0.113.10", Port: 80, Authentication: AuthenticationInput{Method: "plain", Password: "p"}},
	} {
		if _, err := store.Add(input); err == nil {
			t.Fatalf("invalid case %d accepted", index)
		}
	}
	path := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(path, []byte(`{"schema_version":3,"selected_id":"missing","nodes":[]}`), 0o600); err != nil {
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
	if _, err := store.CommitResolvedIP(item.ID, "2606:4700::44"); err != nil {
		t.Fatalf("IPv6 resolution rejected: %v", err)
	}
	loaded, err := New(path, testProtector{})
	if err != nil || loaded.List()[0].ResolvedIP != "2606:4700::44" {
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

func TestSubscriptionRefreshPreservesSelectedNode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.json")
	store, err := New(path, testProtector{})
	if err != nil {
		t.Fatal(err)
	}
	items, err := store.ReplaceSubscription("sub-select", []Input{
		{Name: "Node-1", Type: TypeHTTP, Server: "203.0.113.10", Port: 8080},
		{Name: "Node-2", Type: TypeShadowsocks, Server: "203.0.113.11", Port: 8388, Authentication: AuthenticationInput{Method: "aes-256-gcm", Password: "p"}},
	})
	if err != nil || len(items) != 2 {
		t.Fatal(err)
	}

	// Select Node-2
	node2, err := store.Select(items[1].ID)
	if err != nil || !node2.Selected {
		t.Fatal(err)
	}

	// Refresh subscription with same nodes
	refreshed, err := store.ReplaceSubscription("sub-select", []Input{
		{Name: "Node-1", Type: TypeHTTP, Server: "203.0.113.10", Port: 8080},
		{Name: "Node-2", Type: TypeShadowsocks, Server: "203.0.113.11", Port: 8388, Authentication: AuthenticationInput{Method: "aes-256-gcm", Password: "p"}},
	})
	if err != nil || len(refreshed) != 2 {
		t.Fatal(err)
	}

	// Node-2 must STILL be selected!
	if !refreshed[1].Selected {
		t.Fatalf("expected Node-2 to remain selected after refresh, got %#v", refreshed)
	}
	if refreshed[0].Selected {
		t.Fatalf("Node-1 should not be selected, got %#v", refreshed)
	}
}

func TestNodeIdentityPreservesCase(t *testing.T) {
	input1 := Input{
		Name:           "Node1",
		Type:           TypeShadowsocks,
		Server:         "Server.Example.COM",
		Port:           8388,
		Authentication: AuthenticationInput{Method: "AES-256-GCM", Password: "SecretPassword123"},
	}
	input2 := Input{
		Name:           "Node2",
		Type:           TypeShadowsocks,
		Server:         "server.example.com",
		Port:           8388,
		Authentication: AuthenticationInput{Method: "aes-256-gcm", Password: "secretpassword123"},
	}

	id1 := inputIdentity(input1)
	id2 := inputIdentity(input2)
	if id1 == id2 {
		t.Fatalf("expected different identities for case-sensitive passwords, got %q vs %q", id1, id2)
	}
}

func TestStoreStrictValidation(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "nodes.json"), testProtector{})
	if err != nil {
		t.Fatal(err)
	}

	// Invalid UUID format on VMess
	if _, err := store.Add(Input{Name: "Bad-VMess", Type: TypeVMess, Server: "1.2.3.4", Port: 443, Authentication: AuthenticationInput{UUID: "not-a-uuid"}}); err == nil {
		t.Fatal("expected error for invalid VMess UUID")
	}

	// Invalid UUID format on VLESS
	if _, err := store.Add(Input{Name: "Bad-VLESS", Type: TypeVLESS, Server: "1.2.3.4", Port: 443, Authentication: AuthenticationInput{UUID: "1234"}}); err == nil {
		t.Fatal("expected error for invalid VLESS UUID")
	}

	// Invalid flow on VLESS
	if _, err := store.Add(Input{Name: "Bad-Flow", Type: TypeVLESS, Server: "1.2.3.4", Port: 443, Authentication: AuthenticationInput{UUID: "a8e678c0-8903-4402-8e99-20aadf1a7cd1", Flow: "invalid-flow"}}); err == nil {
		t.Fatal("expected error for invalid VLESS flow")
	}

	// ClearSecret on VLESS
	if _, err := store.Add(Input{Name: "Clear-VLESS", Type: TypeVLESS, Server: "1.2.3.4", Port: 443, ClearSecret: true}); err == nil {
		t.Fatal("expected error for clear_secret on VLESS")
	}
}

func TestNodeSchemaTwoMigratesWithoutLosingData(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.json")
	data := []byte(`{"schema_version":2,"selected_id":"one","nodes":[{"id":"one","name":"Legacy SS","type":"shadowsocks","server":"203.0.113.9","port":8080,"username":"aes-256-gcm","protected_password":"cHJvdGVjdGVkOnNlY3JldA==","favorite":true}]}`)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	store, err := New(path, testProtector{})
	if err != nil || len(store.List()) != 1 {
		t.Fatalf("migration = %#v, %v", store, err)
	}
	migratedList := store.List()
	if migratedList[0].Egress != EgressB || migratedList[0].Authentication.Method != "aes-256-gcm" || !migratedList[0].Favorite || !migratedList[0].Selected {
		t.Fatalf("migrated node = %#v", migratedList[0])
	}
	migrated, _ := os.ReadFile(path)
	if !bytes.Contains(migrated, []byte(`"schema_version": 3`)) {
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

func TestStoreRejectsULAAndPrivateNodeServer(t *testing.T) {
	store, err := New(filepath.Join(t.TempDir(), "nodes.json"), testProtector{})
	if err != nil {
		t.Fatal(err)
	}
	for _, ip := range []string{"fc00::1", "fd00::1", "fe80::1", "::", "::1", "0.0.0.0", "127.0.0.1"} {
		if _, err := store.Add(Input{Name: "Bad-Node", Type: TypeHTTP, Server: ip, Port: 8080}); err == nil {
			t.Fatalf("store.Add succeeded for invalid/ULA/private IP %q, want error", ip)
		}
	}
}

func TestStoreProxyChainSelectionAndMode(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nodes.json")
	store, err := New(path, testProtector{})
	if err != nil {
		t.Fatal(err)
	}

	n1, err := store.Add(Input{
		Name:           "Hop1",
		Type:           TypeShadowsocks,
		Server:         "198.51.100.10",
		Port:           8388,
		Authentication: AuthenticationInput{Method: "aes-128-gcm", Password: "secret-1"},
	})
	if err != nil {
		t.Fatal(err)
	}

	n2, err := store.Add(Input{
		Name:           "Hop2",
		Type:           TypeVMess,
		Server:         "198.51.100.20",
		Port:           443,
		Authentication: AuthenticationInput{UUID: "a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d"},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Initial selection default to single
	sel := store.GetSelection()
	if sel.Mode != ProxyModeSingle || sel.SelectedID != n1.ID {
		t.Fatalf("unexpected initial selection: %#v", sel)
	}

	// Select chain
	if err := store.SelectChain([]string{n1.ID, n2.ID}); err != nil {
		t.Fatalf("SelectChain failed: %v", err)
	}

	sel2 := store.GetSelection()
	if sel2.Mode != ProxyModeChain || len(sel2.SelectedChain) != 2 || sel2.SelectedChain[0] != n1.ID || sel2.SelectedChain[1] != n2.ID {
		t.Fatalf("unexpected chain selection: %#v", sel2)
	}

	// Verify List() reflects chain positions
	list := store.List()
	if len(list) != 2 {
		t.Fatalf("expected 2 nodes, got %d", len(list))
	}
	var node1, node2 Node
	for _, n := range list {
		if n.ID == n1.ID {
			node1 = n
		}
		if n.ID == n2.ID {
			node2 = n
		}
	}
	if node1.ChainPosition != 1 || !node1.Selected {
		t.Errorf("node 1 chain pos = %d, selected = %v, want pos=1, selected=true", node1.ChainPosition, node1.Selected)
	}
	if node2.ChainPosition != 2 || !node2.Selected {
		t.Errorf("node 2 chain pos = %d, selected = %v, want pos=2, selected=true", node2.ChainPosition, node2.Selected)
	}

	// Verify SelectedChainCredentials()
	chainNodes, chainCreds, err := store.SelectedChainCredentials()
	if err != nil {
		t.Fatalf("SelectedChainCredentials failed: %v", err)
	}
	if len(chainNodes) != 2 || len(chainCreds) != 2 {
		t.Fatalf("expected 2 items, got nodes=%d creds=%d", len(chainNodes), len(chainCreds))
	}
	if chainCreds[0].Password != "secret-1" || chainCreds[1].UUID != "a1b2c3d4-e5f6-7a8b-9c0d-1e2f3a4b5c6d" {
		t.Errorf("credentials mismatch: %#v, %#v", chainCreds[0], chainCreds[1])
	}

	// Test deleting n1 removes it from chain
	if err := store.Delete(n1.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	sel3 := store.GetSelection()
	if len(sel3.SelectedChain) != 1 || sel3.SelectedChain[0] != n2.ID {
		t.Errorf("expected chain after delete to contain only n2, got %#v", sel3.SelectedChain)
	}

	// Test Select single node switches mode back to single
	if _, err := store.Select(n2.ID); err != nil {
		t.Fatalf("Select single failed: %v", err)
	}
	sel4 := store.GetSelection()
	if sel4.Mode != ProxyModeSingle || sel4.SelectedID != n2.ID {
		t.Errorf("expected single mode with n2 selected, got %#v", sel4)
	}
}

func TestStoreChainModeEmptyDefensive(t *testing.T) {
	tempPath := filepath.Join(t.TempDir(), "empty_nodes.json")
	store, err := New(tempPath, testProtector{})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}

	// 1. SetMode chain on empty store must fail
	if err := store.SetMode(ProxyModeChain); err == nil {
		t.Fatal("expected error when setting chain mode on empty store, got nil")
	}

	// 2. GetSelection must return single mode and non-nil empty slice for chain
	sel := store.GetSelection()
	if sel.Mode != ProxyModeSingle {
		t.Errorf("expected mode single, got %s", sel.Mode)
	}
	if sel.SelectedChain == nil || len(sel.SelectedChain) != 0 {
		t.Errorf("expected non-nil empty slice for SelectedChain, got %#v", sel.SelectedChain)
	}

	// 3. JSON serialization of ProxySelection must serialize SelectedChain as [] not null
	data, err := json.Marshal(sel)
	if err != nil {
		t.Fatalf("marshal selection failed: %v", err)
	}
	if !strings.Contains(string(data), `"selected_chain":[]`) {
		t.Errorf("expected JSON to contain '\"selected_chain\":[]', got %s", string(data))
	}

	// 4. File on disk with mode: chain and nodes: [] should auto-heal to single mode on load
	chainOnDiskPath := filepath.Join(t.TempDir(), "chain_disk.json")
	if err := os.WriteFile(chainOnDiskPath, []byte(`{"schema_version":3,"mode":"chain","selected_chain":[],"nodes":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	loadedStore, err := New(chainOnDiskPath, testProtector{})
	if err != nil {
		t.Fatalf("loading store with chain mode on 0 nodes failed: %v", err)
	}
	loadedSel := loadedStore.GetSelection()
	if loadedSel.Mode != ProxyModeSingle {
		t.Errorf("expected auto-healed mode single, got %s", loadedSel.Mode)
	}

	// 5. Deleting all nodes from an active chain auto-resets mode to single
	n, err := loadedStore.Add(Input{Name: "Node1", Type: TypeHTTP, Server: "203.0.113.1", Port: 8080})
	if err != nil {
		t.Fatalf("add node failed: %v", err)
	}
	if err := loadedStore.SetMode(ProxyModeChain); err != nil {
		t.Fatalf("SetMode chain failed with 1 node: %v", err)
	}
	if err := loadedStore.SelectChain([]string{n.ID}); err != nil {
		t.Fatalf("SelectChain failed: %v", err)
	}
	if err := loadedStore.Delete(n.ID); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	afterDeleteSel := loadedStore.GetSelection()
	if afterDeleteSel.Mode != ProxyModeSingle {
		t.Errorf("expected mode single after deleting last node, got %s", afterDeleteSel.Mode)
	}
	if afterDeleteSel.SelectedChain == nil || len(afterDeleteSel.SelectedChain) != 0 {
		t.Errorf("expected empty SelectedChain after deleting last node, got %#v", afterDeleteSel.SelectedChain)
	}
}


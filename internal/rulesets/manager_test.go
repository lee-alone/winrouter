package rulesets

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestUpdateVerifiesHashAndKeepsLastValidVersion(t *testing.T) {
	valid := []byte(`{"version":"2026.08.01","rules":[{"type":"domain","value":"example.com","action":"reject"}]}`)
	current := valid
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(current) }))
	defer server.Close()
	client := server.Client()
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // test server only
	manager, err := newManager(filepath.Join(t.TempDir(), "ruleset.json"), client)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(valid)
	if _, err = manager.Configure(Source{Name: "fixed", URL: server.URL, ExpectedSHA256: hex.EncodeToString(digest[:])}); err != nil {
		t.Fatal(err)
	}
	updated, err := manager.Update(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if updated.RuleCount != 1 || updated.AppliedSHA256 == "" {
		t.Fatalf("updated = %#v", updated)
	}
	current = []byte(`{"version":"bad","rules":[]}`)
	badDigest := sha256.Sum256(current)
	if _, err = manager.Configure(Source{Name: "fixed-next", URL: server.URL, ExpectedSHA256: hex.EncodeToString(badDigest[:])}); err != nil {
		t.Fatal(err)
	}
	if _, err = manager.Update(context.Background()); err == nil {
		t.Fatal("invalid replacement accepted")
	}
	rules, err := manager.Rules()
	if err != nil || len(rules) != 1 || rules[0].Value != "example.com" {
		t.Fatalf("last valid rules = %#v, %v", rules, err)
	}
}

func TestConfigureRejectsUnsafeSource(t *testing.T) {
	manager, _ := New(filepath.Join(t.TempDir(), "ruleset.json"))
	for _, source := range []Source{{Name: "x", URL: "http://example.com/a", ExpectedSHA256: string(make([]byte, 64))}, {Name: "x", URL: "https://user@example.com/a", ExpectedSHA256: string(make([]byte, 64))}, {Name: "x", URL: "https://example.com/a?token=value", ExpectedSHA256: string(make([]byte, 64))}} {
		if _, err := manager.Configure(source); err == nil {
			t.Fatalf("unsafe source accepted: %#v", source)
		}
	}
}

package srssets

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultsExposeChinaDomainAndIPSources(t *testing.T) {
	manager, err := newManager(filepath.Join(t.TempDir(), "sources.json"), func(context.Context, string) error { return nil }, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	sources := manager.List()
	if len(sources) != 14 || sources[0].Kind != "domain" || sources[1].Kind != "ip" || sources[2].ID != "sagernet-geosite-github" || sources[2].Action != "b" || sources[3].ID != "sagernet-geosite-cloudflare" || sources[3].Action != "b" || sources[0].URL == "" || sources[1].URL == "" {
		t.Fatalf("defaults = %#v", sources)
	}
	// Verify that embedded default rules were automatically seeded with non-empty AppliedSHA256
	if sources[0].AppliedSHA256 == "" || sources[1].AppliedSHA256 == "" {
		t.Fatalf("default sources must be seeded with non-empty applied hash: %#v", sources[0])
	}
}

func TestUpdateValidatesAndKeepsLastGoodSRS(t *testing.T) {
	valid := []byte("SRS-valid-test-data")
	current := valid
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(current) }))
	defer server.Close()
	client := server.Client()
	client.Transport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // test server only
	validated := 0
	manager, err := newManager(filepath.Join(t.TempDir(), "sources.json"), func(context.Context, string) error { validated++; return nil }, client)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(valid)
	source, err := manager.Configure(Source{Name: "custom", Kind: "domain", URL: server.URL, Enabled: true, Action: "a"})
	if err != nil {
		t.Fatal(err)
	}
	updated, err := manager.Update(context.Background(), source.ID)
	if err != nil || validated != 1 || updated.AppliedSHA256 == "" {
		t.Fatalf("update = %#v, validation=%d, err=%v", updated, validated, err)
	}
	if updated.AppliedSHA256 != hex.EncodeToString(digest[:]) {
		t.Fatalf("applied hash = %q, want %x", updated.AppliedSHA256, digest)
	}
	current = []byte("not-an-srs")
	if _, err := manager.Update(context.Background(), source.ID); err == nil {
		t.Fatal("invalid update accepted")
	}
	active, err := manager.Active()
	if err != nil {
		t.Fatalf("active err = %v", err)
	}
	foundCustom := false
	for _, item := range active {
		if item.Tag == "winrouter-"+source.ID && item.Path != "" {
			foundCustom = true
			break
		}
	}
	if !foundCustom {
		t.Fatalf("active does not contain custom rule: %#v", active)
	}
}

func TestCustomSourceAllowsRollingUpdatesAndRejectsValidatorFailure(t *testing.T) {
	manager, err := newManager(filepath.Join(t.TempDir(), "sources.json"), func(context.Context, string) error { return errors.New("bad SRS") }, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Configure(Source{Name: "custom", Kind: "ip", URL: "https://example.com/rules.srs", Enabled: true, Action: "b"}); err != nil {
		t.Fatalf("rolling custom source rejected: %v", err)
	}
	if _, err := manager.Configure(Source{Name: "locked", Kind: "ip", URL: "https://example.com/locked.srs", ExpectedSHA256: "bad", Enabled: true, Action: "b"}); err == nil {
		t.Fatal("invalid fixed SHA-256 accepted")
	}
}

func TestSeedDefaultsAndOfflineRecovery(t *testing.T) {
	tempDir := t.TempDir()
	sourcesPath := filepath.Join(tempDir, "sources.json")
	manager, err := newManager(sourcesPath, func(context.Context, string) error { return nil }, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	geoDir := filepath.Join(tempDir, "geo")
	entries, err := os.ReadDir(geoDir)
	if err != nil {
		t.Fatalf("read geo dir: %v", err)
	}
	if len(entries) != 14 {
		t.Fatalf("expected 14 seeded geo files, got %d", len(entries))
	}
	// Verify that all sources have AppliedSHA256 set
	for _, source := range manager.List() {
		if source.AppliedSHA256 == "" || source.Size <= 0 {
			t.Fatalf("source %s not seeded with hash and size: %#v", source.ID, source)
		}
	}
	// Delete one file and verify self-healing on next initialization
	testFile := filepath.Join(geoDir, "sagernet-geosite-openai.srs")
	if err := os.Remove(testFile); err != nil {
		t.Fatal(err)
	}
	// Create another manager on the same path
	reloaded, err := newManager(sourcesPath, func(context.Context, string) error { return nil }, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(testFile); err != nil {
		t.Fatalf("self-healing failed to restore deleted file: %v", err)
	}
	active, err := reloaded.Active()
	if err != nil || len(active) == 0 {
		t.Fatalf("reloaded manager active failed: %v", err)
	}
}

package srssets

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

func TestDefaultsExposeChinaDomainAndIPSources(t *testing.T) {
	manager, err := newManager(filepath.Join(t.TempDir(), "sources.json"), func(context.Context, string) error { return nil }, http.DefaultClient)
	if err != nil {
		t.Fatal(err)
	}
	sources := manager.List()
	if len(sources) != 4 || sources[0].Kind != "domain" || sources[1].Kind != "ip" || sources[2].ID != "sagernet-geosite-github" || sources[2].Action != "b" || sources[3].ID != "sagernet-geosite-cloudflare" || sources[3].Action != "b" || sources[0].URL == "" || sources[1].URL == "" {
		t.Fatalf("defaults = %#v", sources)
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
	if err != nil || len(active) != 1 || active[0].Path == "" {
		t.Fatalf("active = %#v, err=%v", active, err)
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

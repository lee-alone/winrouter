package subscriptions

import (
	"bytes"
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"winrouter/internal/nodes"
)

type testProtector struct{}

func (testProtector) Protect(value []byte) ([]byte, error) {
	return append([]byte("protected:"), value...), nil
}

func TestParseBase64ShadowsocksSubscription(t *testing.T) {
	plain := "ss://YWVzLTEyOC1nY206c2hhZG93c29ja3M=@37.19.198.244:443#US-0088\n" +
		"ss://YWVzLTI1Ni1nY206c2VjcmV0@37.19.198.160:8443#US-0089\n"
	items, err := Parse([]byte(base64.StdEncoding.EncodeToString([]byte(plain))))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 || items[0].Type != nodes.TypeShadowsocks || items[0].Username != "aes-128-gcm" || items[0].Password != "shadowsocks" || items[0].Server != "37.19.198.244" || items[0].Port != 443 || items[0].Name != "US-0088" {
		t.Fatalf("parsed nodes = %#v", items)
	}
}
func (testProtector) Unprotect(value []byte) ([]byte, error) {
	return bytes.TrimPrefix(value, []byte("protected:")), nil
}

func TestParseRejectsUnknownFieldsCommandsAndTrailingJSON(t *testing.T) {
	valid := []byte(`{"version":1,"nodes":[{"name":"one","type":"http","server":"proxy.example.test","port":8080}]}`)
	items, err := Parse(valid)
	if err != nil || len(items) != 1 {
		t.Fatalf("valid parse = %#v, %v", items, err)
	}
	for index, data := range [][]byte{
		[]byte(`{"version":1,"command":"calc","nodes":[{"name":"one","type":"http","server":"203.0.113.1","port":80}]}`),
		append(valid, []byte(` {}`)...),
		[]byte(`{"version":2,"nodes":[{"name":"one","type":"http","server":"203.0.113.1","port":80}]}`),
	} {
		if _, err := Parse(data); err == nil {
			t.Fatalf("invalid document %d accepted", index)
		}
	}
}

func TestValidateSubscriptionURLAllowsPrivateHTTPOnly(t *testing.T) {
	for _, value := range []string{
		"https://example.com/nodes.json",
		"http://192.168.1.50:3001/1/download/collection/ss",
		"http://127.0.0.1:8080/nodes.json",
		"http://[::1]:8080/nodes.json",
	} {
		if _, err := validateSubscriptionURL(value); err != nil {
			t.Errorf("valid URL %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{
		"http://example.com/nodes.json",
		"http://8.8.8.8/nodes.json",
		"ftp://192.168.1.50/nodes.json",
		"http://user:password@192.168.1.50/nodes.json",
	} {
		if _, err := validateSubscriptionURL(value); err == nil {
			t.Errorf("unsafe URL %q accepted", value)
		}
	}
}

func TestRefreshStagesNodesAndRollsBackInvalidUpdate(t *testing.T) {
	document := `{"version":1,"nodes":[{"name":"one","type":"http","server":"203.0.113.10","port":8080,"username":"alice","password":"secret"}]}`
	server := httptest.NewTLSServer(http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) { _, _ = response.Write([]byte(document)) }))
	defer server.Close()
	directory := t.TempDir()
	nodeStore, err := nodes.New(filepath.Join(directory, "nodes.json"), testProtector{})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(directory, "subscriptions.json"), testProtector{}, nodeStore)
	if err != nil {
		t.Fatal(err)
	}
	manager.client = server.Client()
	subscription, err := manager.Add(Input{Name: "test", URL: server.URL + "/nodes?token=sensitive"})
	if err != nil {
		t.Fatal(err)
	}
	stored, err := os.ReadFile(filepath.Join(directory, "subscriptions.json"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("token=sensitive")) {
		t.Fatalf("subscription credential stored in plaintext: %s", stored)
	}
	subscription, err = manager.Refresh(context.Background(), subscription.ID)
	if err != nil || subscription.NodeCount != 1 {
		t.Fatalf("refresh = %#v, %v", subscription, err)
	}
	items := nodeStore.List()
	if len(items) != 1 || items[0].SubscriptionID != subscription.ID || !items[0].HasPassword {
		t.Fatalf("nodes = %#v", items)
	}
	document = `{"version":1,"nodes":[{"name":"bad","type":"command","server":"203.0.113.11","port":80}]}`
	if _, err := manager.Refresh(context.Background(), subscription.ID); err == nil {
		t.Fatal("invalid refresh succeeded")
	}
	items = nodeStore.List()
	if len(items) != 1 || items[0].Name != "one" {
		t.Fatalf("failed refresh replaced nodes: %#v", items)
	}
}

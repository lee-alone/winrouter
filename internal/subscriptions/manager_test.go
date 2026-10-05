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

func TestValidateSubscriptionURL(t *testing.T) {
	for _, value := range []string{
		"https://example.com/nodes.json",
		"https://sub.airport.net/api/v1/client/subscribe?token=abc",
		"https://127.0.0.1:8080/nodes.json",
		"http://192.168.1.50:3001/1/download/collection/ss",
		"http://127.0.0.1:8080/nodes.json",
		"http://10.0.0.1:8080/nodes.json",
		"http://172.16.0.1:8080/nodes.json",
		"http://[::1]:8080/nodes.json",
	} {
		if _, err := validateSubscriptionURL(value); err != nil {
			t.Errorf("valid URL %q rejected: %v", value, err)
		}
	}
	for _, value := range []string{
		"http://example.com/nodes.json",                // public HTTP not allowed
		"http://8.8.8.8/nodes.json",                    // public IP over HTTP not allowed
		"http://169.254.169.254/meta-data",             // link-local / cloud metadata over HTTP
		"https://169.254.169.254/meta-data",            // link-local / cloud metadata over HTTPS
		"ftp://192.168.1.50/nodes.json",                // unsupported scheme
		"http://user:password@192.168.1.50/nodes.json", // userinfo not allowed
		"http:///nodes.json",
		"invalid-url",
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

func TestRefreshRedirectSafety(t *testing.T) {
	document := `proxies:
  - name: Redirect-Node
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: secret
`
	// 1. Valid redirect within private HTTP
	targetServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(document))
	}))
	defer targetServer.Close()

	redirectServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, targetServer.URL, http.StatusFound)
	}))
	defer redirectServer.Close()

	directory := t.TempDir()
	nodeStore, err := nodes.New(filepath.Join(directory, "nodes.json"), testProtector{})
	if err != nil {
		t.Fatal(err)
	}
	manager, err := New(filepath.Join(directory, "subscriptions.json"), testProtector{}, nodeStore)
	if err != nil {
		t.Fatal(err)
	}
	sub, err := manager.Add(Input{Name: "redirect-test", URL: redirectServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	sub, err = manager.Refresh(context.Background(), sub.ID)
	if err != nil {
		t.Fatalf("refresh with valid redirect failed: %v", err)
	}
	if sub.NodeCount != 1 {
		t.Fatalf("expected 1 node, got %d", sub.NodeCount)
	}

	// 2. HTTPS to HTTP downgrade redirection is rejected
	httpTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(document))
	}))
	defer httpTarget.Close()

	httpsDowngradeServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, httpTarget.URL, http.StatusFound)
	}))
	defer httpsDowngradeServer.Close()

	manager.client = httpsDowngradeServer.Client()
	manager.client.CheckRedirect = createSubscriptionHTTPClient().CheckRedirect

	subDowngrade, err := manager.Add(Input{Name: "downgrade-test", URL: httpsDowngradeServer.URL})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Refresh(context.Background(), subDowngrade.ID); err == nil {
		t.Fatal("expected error on HTTPS to HTTP downgrade redirect")
	}

	// 3. Public HTTPS redirect to private/loopback IP is rejected
	fakePublicReq, _ := http.NewRequest("GET", "https://airport.example.com/sub", nil)
	privateRedirectReq, _ := http.NewRequest("GET", "https://127.0.0.1:8443/sub", nil)
	checkRedirect := createSubscriptionHTTPClient().CheckRedirect
	if err := checkRedirect(privateRedirectReq, []*http.Request{fakePublicReq}); err == nil {
		t.Fatal("expected error on public HTTPS redirect to private IP")
	}

	// 4. Public HTTPS redirect to domain resolving to loopback (localhost) is rejected
	localhostRedirectReq, _ := http.NewRequest("GET", "https://localhost:8443/sub", nil)
	if err := checkRedirect(localhostRedirectReq, []*http.Request{fakePublicReq}); err == nil {
		t.Fatal("expected error on public HTTPS redirect to localhost (DNS rebinding defense)")
	}
}

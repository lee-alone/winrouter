package clashapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestListConnections(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Error("missing authorization")
		}
		_, _ = w.Write([]byte(`{"connections":[{"id":"1","rule":"geosite-cn","chains":["domestic-direct"],"metadata":{"network":"tcp","type":"TUN","host":"a.example","destinationIP":"1.2.3.4","destinationPort":"443"}}]}`))
	}))
	defer server.Close()
	result, err := (Client{BaseURL: server.URL, Secret: "secret"}).ListConnections(context.Background())
	if err != nil || len(result.Connections) != 1 || result.Connections[0].Rule != "geosite-cn" {
		t.Fatalf("result=%#v err=%v", result, err)
	}
}

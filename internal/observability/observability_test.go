package observability

import (
	"archive/zip"
	"context"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"winrouter/internal/interfaces"
)

func TestStoreKeepsBoundedStructuredHistory(t *testing.T) {
	store := NewStore(2)
	store.Log(LevelInfo, "app", "one", "", nil)
	store.Log(LevelWarning, "core", "two", "WR-2", map[string]any{"attempt": 1})
	store.Log(LevelError, "core", "three", "WR-3", nil)
	logs := store.Logs()
	if len(logs) != 2 || logs[0].Message != "two" || logs[1].CorrelationID != "WR-3" {
		t.Fatalf("unexpected bounded logs: %#v", logs)
	}
}

func TestNewStoreSerializesEmptyCollectionsAsArrays(t *testing.T) {
	store := NewStore(2)
	value := struct {
		Logs     []LogEntry        `json:"logs"`
		Probes   []ProbeResult     `json:"probes"`
		RuleSets []RuleSetMetadata `json:"rule_sets"`
	}{store.Logs(), store.Probes(), store.RuleSets()}
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "null") {
		t.Fatalf("empty collections serialized as null: %s", data)
	}
}

func TestInterfaceForSourceUsesEnumeratedAddress(t *testing.T) {
	adapters := []interfaces.Adapter{{GUID: "{A}", Addresses: []interfaces.Address{{IP: "192.0.2.4"}}}}
	if got := interfaceForSource("192.0.2.4", adapters); got != "{A}" {
		t.Fatalf("got %q", got)
	}
}

func TestSourceForInterfaceSelectsIPv4ByGUID(t *testing.T) {
	adapters := []interfaces.Adapter{{GUID: "{A}", Addresses: []interfaces.Address{{IP: "2001:db8::1"}, {IP: "169.254.1.2"}, {IP: "192.0.2.4"}}}}
	got, err := sourceForInterface("a", adapters)
	if err != nil || got != "192.0.2.4" {
		t.Fatalf("source = %q, %v", got, err)
	}
}

func TestRunProbeRejectsUnavailableExpectedInterface(t *testing.T) {
	result := RunProbe(context.Background(), ProbeRequest{Protocol: "tcp", Target: "127.0.0.1:1", ExpectedInterface: "{missing}"}, nil)
	if result.Success || !strings.Contains(result.Error, "is not available") {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestRunProbeBindsTCPToExpectedInterfaceAddress(t *testing.T) {
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan struct{})
	go func() {
		connection, acceptErr := listener.Accept()
		if acceptErr == nil {
			connection.Close()
		}
		close(accepted)
	}()
	adapters := []interfaces.Adapter{{GUID: "{LOOPBACK}", Addresses: []interfaces.Address{{IP: "127.0.0.1"}}}}
	result := RunProbe(context.Background(), ProbeRequest{Protocol: "tcp", Target: listener.Addr().String(), ExpectedInterface: "loopback", TimeoutMS: 1000}, adapters)
	<-accepted
	if !result.Success || result.SourceAddress != "127.0.0.1" || result.ActualInterface != "{LOOPBACK}" {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseRuleHitsCountsConfirmedOutboundLines(t *testing.T) {
	log := "outbound/direct[domestic-direct]: outbound connection to 1.1.1.1:443\n" +
		"outbound/direct[foreign-direct]: outbound packet connection to 8.8.8.8:53\n" +
		"outbound/direct[b-direct]: outbound connection to 9.9.9.9:443\n"
	hits := ParseRuleHits(log)
	if len(hits) != 2 || hits[0].Count != 1 || hits[1].Count != 2 {
		t.Fatalf("unexpected hits: %#v", hits)
	}
}

func TestWriteBundleRedactsSecretsAndExcludesRuleSetBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "diagnostic.zip")
	input := BundleInput{Application: map[string]any{"password": "open", "endpoint": "https://example.test/x?token=open&mode=1"}, Logs: []LogEntry{{Message: "password=open UUID 550e8400-e29b-41d4-a716-446655440000"}}, RuleSets: []RuleSetMetadata{{Name: "geo", SHA256: "abc", Source: "https://example.test/rules?api_key=open"}}}
	if err := WriteBundle(path, input); err != nil {
		t.Fatal(err)
	}
	archive, err := zip.OpenReader(path)
	if err != nil {
		t.Fatal(err)
	}
	defer archive.Close()
	if len(archive.File) != len(Preview(input).Files) {
		t.Fatalf("unexpected files: %d", len(archive.File))
	}
	for _, file := range archive.File {
		reader, err := file.Open()
		if err != nil {
			t.Fatal(err)
		}
		var value any
		err = json.NewDecoder(reader).Decode(&value)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := json.Marshal(value)
		text := string(raw)
		if strings.Contains(text, "open") {
			t.Fatalf("secret leaked in %s: %s", file.Name, text)
		}
		if strings.Contains(text, "550e8400-e29b-41d4-a716-446655440000") {
			t.Fatalf("UUID leaked in %s", file.Name)
		}
		if strings.Contains(strings.ToLower(file.Name), "binary") {
			t.Fatalf("unexpected rule-set binary: %s", file.Name)
		}
	}
	if info, err := os.Stat(path); err != nil || info.Size() == 0 {
		t.Fatalf("bundle not written: %v", err)
	}
}

package main

import (
	"archive/zip"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"winrouter/internal/observability"
)

type check struct {
	Name   string `json:"name"`
	Passed bool   `json:"passed"`
	Detail string `json:"detail,omitempty"`
}

type report struct {
	Bundle string   `json:"bundle"`
	Files  []string `json:"files"`
	Checks []check  `json:"checks"`
	Passed bool     `json:"passed"`
}

func main() {
	output := flag.String("output", filepath.Join("build", "phase1-diagnostics", "diagnostic-sentinel.zip"), "diagnostic ZIP output path")
	reportPath := flag.String("report", filepath.Join("build", "phase1-diagnostics", "diagnostic-check-summary.json"), "JSON report output path")
	flag.Parse()
	if err := os.MkdirAll(filepath.Dir(*output), 0o755); err != nil {
		fail(err)
	}
	input := sentinelInput()
	if err := observability.WriteBundle(*output, input); err != nil {
		fail(err)
	}
	result, err := inspect(*output, observability.Preview(input).Files)
	if err != nil {
		fail(err)
	}
	data, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		fail(err)
	}
	data = append(data, '\n')
	if err = os.MkdirAll(filepath.Dir(*reportPath), 0o755); err != nil {
		fail(err)
	}
	if err = os.WriteFile(*reportPath, data, 0o600); err != nil {
		fail(err)
	}
	fmt.Print(string(data))
	if !result.Passed {
		os.Exit(2)
	}
}

func sentinelInput() observability.BundleInput {
	return observability.BundleInput{
		Application: map[string]any{
			"name": "WinRouter", "password": "SENTINEL_PASSWORD", "token": "SENTINEL_TOKEN",
			"endpoint": "https://example.test/api?api_key=SENTINEL_API_KEY&mode=check",
		},
		Interfaces: map[string]any{"selected_guid": "550e8400-e29b-41d4-a716-446655440000", "name": "WLAN"},
		Core:       map[string]any{"state": "stopped", "authorization": "Bearer SENTINEL_BEARER"},
		Logs: []observability.LogEntry{{
			Time: time.Now().UTC(), Level: observability.LevelError, Component: "diagnostic",
			Message: "password=SENTINEL_INLINE uuid=550e8400-e29b-41d4-a716-446655440000 Bearer SENTINEL_BEARER",
			Fields:  map[string]any{"subscription": "SENTINEL_SUBSCRIPTION"},
		}},
		Probes:   []observability.ProbeResult{{Time: time.Now().UTC(), Protocol: "tcp", Target: "1.1.1.1:443", Success: true}},
		Counters: []observability.InterfaceCounter{{GUID: "550e8400-e29b-41d4-a716-446655440000", Name: "WLAN", Received: 100, Transmitted: 200, SampledAt: time.Now().UTC()}},
		RuleSets: []observability.RuleSetMetadata{{Name: "geo-cn", Version: "test", SHA256: strings.Repeat("a", 64), Source: "https://example.test/rules?credential=SENTINEL_CREDENTIAL", RuleCount: 1, Size: 64, LoadResult: "loaded"}},
	}
}

func inspect(path string, expected []string) (report, error) {
	archive, err := zip.OpenReader(path)
	if err != nil {
		return report{}, err
	}
	defer archive.Close()
	result := report{Bundle: filepath.Clean(path), Passed: true}
	var combined strings.Builder
	for _, file := range archive.File {
		result.Files = append(result.Files, file.Name)
		reader, openErr := file.Open()
		if openErr != nil {
			return report{}, openErr
		}
		data, readErr := io.ReadAll(reader)
		reader.Close()
		if readErr != nil {
			return report{}, readErr
		}
		combined.Write(data)
		var value any
		valid := json.Unmarshal(data, &value) == nil
		result.add("valid-json:"+file.Name, valid, "")
	}
	sort.Strings(result.Files)
	want := append([]string(nil), expected...)
	sort.Strings(want)
	result.add("fixed-file-list", strings.Join(result.Files, "\n") == strings.Join(want, "\n"), strings.Join(result.Files, ", "))
	text := combined.String()
	for _, sentinel := range []string{"SENTINEL_PASSWORD", "SENTINEL_TOKEN", "SENTINEL_API_KEY", "SENTINEL_BEARER", "SENTINEL_INLINE", "SENTINEL_SUBSCRIPTION", "SENTINEL_CREDENTIAL", "550e8400-e29b-41d4-a716-446655440000"} {
		result.add("redacted:"+sentinel, !strings.Contains(text, sentinel), "")
	}
	result.add("redaction-marker-present", strings.Contains(text, "[REDACTED]") || strings.Contains(text, "%5BREDACTED%5D"), "")
	for _, name := range result.Files {
		lower := strings.ToLower(name)
		result.add("no-rule-set-binary:"+name, !strings.Contains(lower, "rule-set") || strings.HasSuffix(lower, ".json"), "")
	}
	return result, nil
}

func (r *report) add(name string, passed bool, detail string) {
	r.Checks = append(r.Checks, check{Name: name, Passed: passed, Detail: detail})
	if !passed {
		r.Passed = false
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

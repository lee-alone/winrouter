//go:build windows

package config

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"winrouter/internal/core"
)

func TestLockedCoreAcceptsLocalBinaryRuleSets(t *testing.T) {
	corePath := filepath.Join("..", "..", "resources", "core", "sing-box.exe")
	directory := t.TempDir()
	sourcePath := filepath.Join(directory, "domain.json")
	binaryPath := filepath.Join(directory, "domain.srs")
	if err := os.WriteFile(sourcePath, []byte(`{"version":3,"rules":[{"domain_suffix":["example.cn"]}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if output, err := exec.Command(corePath, "rule-set", "compile", "-o", binaryPath, sourcePath).CombinedOutput(); err != nil {
		t.Fatalf("compile SRS: %v: %s", err, output)
	}
	input := fixtureInput(t)
	input.RuleSets = []MVPRuleSet{{Tag: "geosite-cn", Kind: "domain", Action: "a", Path: binaryPath}}
	generated, report, err := ValidateMVPWithCore(context.Background(), input, corePath)
	if err != nil {
		t.Fatal(err)
	}
	lastDNSRule := generated.Model.DNS.Rules[len(generated.Model.DNS.Rules)-1]
	if report.Core.SHA256 == "" || len(generated.Model.Route.RuleSets) != 1 || len(lastDNSRule.RuleSet) != 1 {
		t.Fatalf("SRS validation = %#v, route = %#v, DNS = %#v", report, generated.Model.Route, generated.Model.DNS.Rules)
	}
}

func TestGeneratedMVPPassesLockedSingBoxCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping locked core integration check")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	generated, report, err := ValidateMVPWithCore(ctx, fixtureInput(t), filepath.Join("..", "..", "resources", "core", "sing-box.exe"))
	if err != nil {
		t.Fatalf("ValidateMVPWithCore() error: %v", err)
	}
	if len(generated.JSON) == 0 || !report.ModelValid || !report.SchemaValid || !report.SemanticValid || report.Core.SHA256 == "" {
		t.Fatalf("validation report = %#v", report)
	}
}

func TestGeneratedConnectionObservationPassesLockedSingBoxCheck(t *testing.T) {
	input := fixtureInput(t)
	input.ConnectionObservation = true
	input.ConnectionAPISecret = "integration-test-secret"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, _, err := ValidateMVPWithCore(ctx, input, filepath.Join("..", "..", "resources", "core", "sing-box.exe")); err != nil {
		t.Fatalf("locked core rejected connection observation: %v", err)
	}
}

func TestLockedCoreAcceptsWindowsProcessMatchers(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping locked core process matcher check")
	}
	generated, err := GenerateMVP(fixtureInput(t))
	if err != nil {
		t.Fatal(err)
	}
	processRule := RouteRule{ProcessName: []string{"winrouter-probe.exe"}, ProcessPath: []string{`C:\Program Files\WinRouter\winrouter-probe.exe`}, Action: "route", Outbound: "domestic-direct"}
	generated.Model.Route.Rules = append([]RouteRule{processRule}, generated.Model.Route.Rules...)
	data, err := json.Marshal(generated.Model)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result, err := core.Validate(ctx, filepath.Join("..", "..", "resources", "core", "sing-box.exe"), core.LockedVersion, core.LockedSHA256, data)
	if err != nil {
		t.Fatalf("locked core rejected Windows process matchers: %v", err)
	}
	if result.SHA256 == "" {
		t.Fatal("locked core validation did not report its hash")
	}
}

func TestGeneratedProxySplitPassesLockedSingBoxCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping locked core integration check")
	}
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "fixtures", "config", "proxy-split-http.json"))
	if err != nil {
		t.Fatal(err)
	}
	input, err := DecodeMVPConfig(data)
	if err != nil {
		t.Fatal(err)
	}
	input.Proxy.Username = "alice"
	input.Proxy.Password = "secret"
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	generated, report, err := ValidateMVPWithCore(ctx, input, filepath.Join("..", "..", "resources", "core", "sing-box.exe"))
	if err != nil {
		t.Fatalf("ValidateMVPWithCore() error: %v", err)
	}
	if generated.Model.Route.Final != "proxy" || !report.ModelValid || !report.SchemaValid || !report.SemanticValid || report.Core.SHA256 == "" {
		t.Fatalf("validation report = %#v", report)
	}
}

func TestGeneratedShadowsocksProxyPassesLockedSingBoxCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping locked core integration check")
	}
	input := fixtureInput(t)
	input.Mode = ModeProxySplit
	input.Proxy = &MVPProxy{Type: "shadowsocks", Server: "37.19.198.244", Port: 443, Method: "aes-128-gcm", Password: "shadowsocks"}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	generated, report, err := ValidateMVPWithCore(ctx, input, filepath.Join("..", "..", "resources", "core", "sing-box.exe"))
	if err != nil {
		t.Fatalf("ValidateMVPWithCore() error: %v", err)
	}
	if generated.Model.Route.Final != "proxy" || !report.ModelValid || !report.SchemaValid || !report.SemanticValid || report.Core.SHA256 == "" {
		t.Fatalf("validation report = %#v", report)
	}
}

func TestGeneratedVMessProxyPassesLockedSingBoxCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping locked core integration check")
	}
	input := fixtureInput(t)
	input.Mode = ModeProxySplit
	input.Proxy = &MVPProxy{
		Type:      "vmess",
		Server:    "203.0.113.88",
		Port:      443,
		UUID:      "a8e678c0-8903-4402-8e99-20aadf1a7cd1",
		TLS:       &MVPProxyTLS{Enabled: true, ServerName: "proxy.example.com"},
		Transport: &MVPProxyTransport{Type: "ws", Path: "/chat", Host: "proxy.example.com"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	generated, report, err := ValidateMVPWithCore(ctx, input, filepath.Join("..", "..", "resources", "core", "sing-box.exe"))
	if err != nil {
		t.Fatalf("ValidateMVPWithCore() error: %v", err)
	}
	if generated.Model.Route.Final != "proxy" || !report.ModelValid || !report.SchemaValid || !report.SemanticValid || report.Core.SHA256 == "" {
		t.Fatalf("validation report = %#v", report)
	}
}

func TestGeneratedVLESSProxyPassesLockedSingBoxCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping locked core integration check")
	}
	input := fixtureInput(t)
	input.Mode = ModeProxySplit
	input.Proxy = &MVPProxy{
		Type:      "vless",
		Server:    "203.0.113.89",
		Port:      443,
		UUID:      "a8e678c0-8903-4402-8e99-20aadf1a7cd1",
		Flow:      "xtls-rprx-vision",
		TLS:       &MVPProxyTLS{Enabled: true, ServerName: "vless.example.com"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	generated, report, err := ValidateMVPWithCore(ctx, input, filepath.Join("..", "..", "resources", "core", "sing-box.exe"))
	if err != nil {
		t.Fatalf("ValidateMVPWithCore() error: %v", err)
	}
	if generated.Model.Route.Final != "proxy" || !report.ModelValid || !report.SchemaValid || !report.SemanticValid || report.Core.SHA256 == "" {
		t.Fatalf("validation report = %#v", report)
	}
}

func TestGeneratedTrojanProxyPassesLockedSingBoxCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping locked core integration check")
	}
	input := fixtureInput(t)
	input.Mode = ModeProxySplit
	input.Proxy = &MVPProxy{
		Type:     "trojan",
		Server:   "203.0.113.90",
		Port:     443,
		Password: "trojanpassword",
		TLS:      &MVPProxyTLS{Enabled: true, ServerName: "trojan.example.com"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	generated, report, err := ValidateMVPWithCore(ctx, input, filepath.Join("..", "..", "resources", "core", "sing-box.exe"))
	if err != nil {
		t.Fatalf("ValidateMVPWithCore() error: %v", err)
	}
	if generated.Model.Route.Final != "proxy" || !report.ModelValid || !report.SchemaValid || !report.SemanticValid || report.Core.SHA256 == "" {
		t.Fatalf("validation report = %#v", report)
	}
}

func TestGeneratedProxyEgressAPassesLockedSingBoxCheck(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping locked core integration check")
	}
	input := fixtureInput(t)
	input.Mode = ModeProxySplit
	input.Proxy = &MVPProxy{
		Type:     "trojan",
		Server:   "203.0.113.91",
		Port:     443,
		Egress:   "a",
		Password: "trojanpassword",
		TLS:      &MVPProxyTLS{Enabled: true, ServerName: "trojan.example.com"},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	generated, report, err := ValidateMVPWithCore(ctx, input, filepath.Join("..", "..", "resources", "core", "sing-box.exe"))
	if err != nil {
		t.Fatalf("ValidateMVPWithCore() error: %v", err)
	}
	if generated.Model.Route.Final != "proxy" || !report.ModelValid || !report.SchemaValid || !report.SemanticValid || report.Core.SHA256 == "" {
		t.Fatalf("validation report = %#v", report)
	}
	var proxyOutbound *Outbound
	for i := range generated.Model.Outbounds {
		if generated.Model.Outbounds[i].Tag == "proxy" {
			proxyOutbound = &generated.Model.Outbounds[i]
		}
	}
	if proxyOutbound == nil || proxyOutbound.BindInterface != input.InterfaceA.BindInterface {
		t.Fatalf("proxy outbound bind interface = %#v, expected %s", proxyOutbound, input.InterfaceA.BindInterface)
	}
}

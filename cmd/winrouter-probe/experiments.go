package main

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"winrouter/internal/config"
	"winrouter/internal/core"
	"winrouter/internal/interfaces"
)

func activeCandidates(adapters []interfaces.Adapter) []interfaces.Adapter {
	result := make([]interfaces.Adapter, 0, 2)
	for _, adapter := range adapters {
		if adapter.Candidate && adapter.Status == "up" && len(adapter.Addresses) > 0 {
			result = append(result, adapter)
		}
	}
	return result
}

func processExperimentInput(prefix, stack string, candidates []interfaces.Adapter, rule config.MVPCustomRule) config.MVPConfig {
	input := config.MVPConfig{
		SchemaVersion: config.SchemaVersion1,
		TUN:           config.MVPTUN{Prefix: prefix, Stack: stack},
		InterfaceA:    config.MVPInterface{GUID: candidates[0].GUID, BindInterface: candidates[0].FriendlyName},
		InterfaceB:    config.MVPInterface{GUID: candidates[1].GUID, BindInterface: candidates[1].FriendlyName},
		CustomRules:   []config.MVPCustomRule{rule},
		Domestic:      config.MVPDomestic{CIDRs: []string{"223.5.5.5/32"}, DomainSuffixes: []string{"baidu.com"}},
		DNS: config.MVPDNS{
			Domestic: config.MVPDNSServer{Type: "udp", Server: "223.5.5.5", Port: 53},
			Global:   config.MVPDNSServer{Type: "udp", Server: "8.8.8.8", Port: 53},
		},
		IPv6: config.IPv6Block,
	}
	for _, direct := range interfaces.BuildTopology(candidates).Prefixes {
		parsed, err := netip.ParsePrefix(direct.Prefix)
		if err == nil && direct.Action == interfaces.PrefixBindInterface && parsed.Addr().Is4() {
			input.DirectPrefixes = append(input.DirectPrefixes, config.MVPDirectPrefix{Prefix: direct.Prefix, BindInterface: direct.AdapterName})
		}
	}
	return input
}

func runProcessExperiment(corePath, tunPrefix, stack string, adapters []interfaces.Adapter, processRule string) (any, error) {
	candidates := activeCandidates(adapters)
	if len(candidates) < 2 {
		return nil, errors.New("process validation requires two active candidate interfaces")
	}
	executable, pathErr := os.Executable()
	if pathErr != nil {
		return nil, fmt.Errorf("resolve probe executable: %w", pathErr)
	}
	executable, pathErr = filepath.Abs(executable)
	if pathErr != nil {
		return nil, fmt.Errorf("resolve absolute probe executable: %w", pathErr)
	}
	ruleType, ruleValue := "process-name", strings.ToLower(filepath.Base(executable))
	if processRule == "path" {
		ruleType, ruleValue = "process-path", executable
	} else if processRule != "name" {
		return nil, errors.New("process-rule must be name or path")
	}
	input := processExperimentInput(tunPrefix, stack, candidates[:2], config.MVPCustomRule{Name: "probe process to A", Type: ruleType, Value: ruleValue, Action: "a"})
	mvp, generateErr := config.GenerateMVP(input)
	if generateErr != nil {
		return nil, fmt.Errorf("generate process experiment config: %w", generateErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	experiment, runErr := core.RunTUNExperiment(ctx, corePath, mvp.JSON, "WinRouter-TUN", tunPrefix, time.Second, []string{"1.1.1.1:443"}, nil, nil, nil, nil, nil)
	if runErr != nil {
		return nil, fmt.Errorf("run process routing experiment: %w", runErr)
	}
	const expectedLog = "outbound/direct[domestic-direct]: outbound connection to 1.1.1.1:443"
	matched := strings.Contains(experiment.CoreOutput, expectedLog)
	if !matched {
		return nil, errors.New("process rule did not route the probe target through domestic-direct")
	}
	return struct {
		RuleType         string               `json:"rule_type"`
		RuleValue        string               `json:"rule_value"`
		ExpectedOutbound string               `json:"expected_outbound"`
		Matched          bool                 `json:"matched"`
		Interfaces       []interfaces.Adapter `json:"interfaces"`
		Experiment       core.TUNExperiment   `json:"experiment"`
	}{RuleType: ruleType, RuleValue: ruleValue, ExpectedOutbound: "domestic-direct", Matched: matched, Interfaces: candidates[:2], Experiment: experiment}, nil
}

func runMVPSemanticExperiment(corePath, tunPrefix, stack string, adapters []interfaces.Adapter) (any, error) {
	candidates := activeCandidates(adapters)
	if len(candidates) < 2 {
		return nil, errors.New("final MVP validation requires two active candidate interfaces")
	}
	input := config.MVPConfig{
		SchemaVersion: config.SchemaVersion1,
		TUN:           config.MVPTUN{Prefix: tunPrefix, Stack: stack},
		InterfaceA:    config.MVPInterface{GUID: candidates[0].GUID, BindInterface: candidates[0].FriendlyName},
		InterfaceB:    config.MVPInterface{GUID: candidates[1].GUID, BindInterface: candidates[1].FriendlyName},
		Domestic:      config.MVPDomestic{CIDRs: []string{"223.5.5.5/32"}, DomainSuffixes: []string{"baidu.com"}},
		DNS: config.MVPDNS{
			Domestic: config.MVPDNSServer{Type: "udp", Server: "223.5.5.5", Port: 53},
			Global:   config.MVPDNSServer{Type: "udp", Server: "8.8.8.8", Port: 53},
		},
		IPv6: config.IPv6Block,
	}
	for _, prefix := range interfaces.BuildTopology(candidates[:2]).Prefixes {
		parsed, parseErr := netip.ParsePrefix(prefix.Prefix)
		if parseErr == nil && prefix.Action == interfaces.PrefixBindInterface && parsed.Addr().Is4() {
			input.DirectPrefixes = append(input.DirectPrefixes, config.MVPDirectPrefix{Prefix: prefix.Prefix, BindInterface: prefix.AdapterName})
		}
	}
	mvp, generateErr := config.GenerateMVP(input)
	if generateErr != nil {
		return nil, fmt.Errorf("generate final MVP config: %w", generateErr)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	experiment, runErr := core.RunTUNExperiment(ctx, corePath, mvp.JSON, "WinRouter-TUN", tunPrefix, 3*time.Second, []string{"223.5.5.5:443", "1.1.1.1:443"}, nil, nil, []string{"www.baidu.com", "example.com"}, nil, nil)
	if runErr != nil {
		return nil, fmt.Errorf("run final MVP semantic experiment: %w", runErr)
	}
	return struct {
		Input      config.MVPConfig     `json:"mvp_input"`
		Interfaces []interfaces.Adapter `json:"interfaces"`
		Rules      []string             `json:"rule_categories"`
		Experiment core.TUNExperiment   `json:"experiment"`
	}{Input: input, Interfaces: candidates[:2], Rules: mvp.RuleCategories, Experiment: experiment}, nil
}

func runSleepResumeExperiment(corePath, tunPrefix, stack string) (any, error) {
	generated, err := config.GenerateDualDirectTUN(tunPrefix, stack, "WLAN", "以太网", "1.1.1.1/32", "8.8.8.8/32", "162.159.200.1/32", "162.159.200.123/32")
	if err != nil {
		return nil, fmt.Errorf("generate sleep/resume config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	experiment, err := core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 9*time.Minute, nil, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("run sleep/resume experiment: %w", err)
	}
	if !experiment.SleepResumeDetected {
		return nil, errors.New("sleep/resume experiment timed out without a suspend gap")
	}
	return experiment, nil
}

func runNetworkChangeExperiment(corePath, tunPrefix, stack string) (any, error) {
	generated, err := config.GenerateDualDirectTUN(tunPrefix, stack, "WLAN", "以太网", "1.1.1.1/32", "8.8.8.8/32", "162.159.200.1/32", "162.159.200.123/32")
	if err != nil {
		return nil, fmt.Errorf("generate network-change config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	experiment, err := core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 60*time.Second, nil, nil, nil, nil, nil, []string{"WLAN", "以太网"})
	if err != nil {
		return nil, fmt.Errorf("run network-change experiment: %w", err)
	}
	if !experiment.NetworkChangeDetected {
		return nil, errors.New("network-change experiment timed out without an interface change")
	}
	return experiment, nil
}

func runLifecycleExperiment(corePath, tunPrefix string, generated []byte, cycles int) (any, error) {
	if cycles < 1 || cycles > 100 {
		return nil, errors.New("cycles must be between 1 and 100")
	}
	type cycleResult struct {
		Cycle            int    `json:"cycle"`
		StopMethod       string `json:"stop_method"`
		GracefulStop     bool   `json:"graceful_stop"`
		InterfaceCleaned bool   `json:"interface_cleaned"`
		RoutesCleaned    bool   `json:"routes_cleaned"`
		ElapsedMillis    int64  `json:"elapsed_millis"`
	}
	report := struct {
		Requested int           `json:"requested"`
		Completed int           `json:"completed"`
		Cycles    []cycleResult `json:"cycles"`
	}{Requested: cycles}
	for cycle := 1; cycle <= cycles; cycle++ {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		experiment, runErr := core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 0, nil, nil, nil, nil, nil, nil)
		cancel()
		if runErr != nil {
			return nil, fmt.Errorf("lifecycle cycle %d: %w", cycle, runErr)
		}
		report.Completed = cycle
		report.Cycles = append(report.Cycles, cycleResult{
			Cycle: cycle, StopMethod: experiment.StopMethod, GracefulStop: experiment.GracefulStop,
			InterfaceCleaned: experiment.InterfaceCleaned, RoutesCleaned: experiment.RoutesCleaned,
			ElapsedMillis: experiment.ElapsedMillis,
		})
	}
	return report, nil
}

func runDualExperiment(corePath, tunPrefix, stack string) (any, error) {
	generated, err := config.GenerateDualDirectTUN(tunPrefix, stack, "WLAN", "以太网", "1.1.1.1/32", "8.8.8.8/32", "162.159.200.1/32", "162.159.200.123/32")
	if err != nil {
		return nil, fmt.Errorf("generate dual direct TUN config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 3*time.Second, []string{"1.1.1.1:443", "8.8.8.8:443"}, []string{"162.159.200.1:123", "162.159.200.123:123"}, nil, nil, nil, nil)
}

func runLANExperiment(corePath, tunPrefix, stack string) (any, error) {
	generated, err := config.GenerateDualLANDirectTUN(tunPrefix, stack, "WLAN", "以太网", "10.12.85.0/24", "192.168.1.0/24")
	if err != nil {
		return nil, fmt.Errorf("generate dual LAN TUN config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 3*time.Second, nil, nil, []string{"10.12.85.232", "192.168.1.1"}, nil, nil, nil)
}

func runDNSExperiment(corePath, tunPrefix, stack string) (any, error) {
	generated, err := config.GenerateDualDNSTUN(tunPrefix, stack, "WLAN", "以太网")
	if err != nil {
		return nil, fmt.Errorf("generate dual DNS TUN config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 3*time.Second, nil, nil, nil, []string{"one.one.one.one", "dns.google"}, nil, nil)
}

func runProxyExperiment(corePath, tunPrefix, stack string) (any, error) {
	const proxyAddress = "192.168.1.166:18080"
	proxy, err := core.StartMockHTTPProxy(proxyAddress)
	if err != nil {
		return nil, fmt.Errorf("start controlled proxy: %w", err)
	}
	defer proxy.Close()
	generated, err := config.GenerateDomainProxyLoopTUN(tunPrefix, stack, "WLAN", "以太网", "proxy.winrouter.test", "192.168.1.166", 18080, "1.1.1.1/32")
	if err != nil {
		return nil, fmt.Errorf("generate proxy-loop config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	experiment, err := core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 3*time.Second, []string{"1.1.1.1:443"}, nil, nil, nil, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("run proxy-loop experiment: %w", err)
	}
	return struct {
		Experiment       core.TUNExperiment `json:"experiment"`
		ProxyConnections int64              `json:"proxy_connections"`
	}{Experiment: experiment, ProxyConnections: proxy.Connections()}, nil
}

func runDNSAddressModelExperiment(corePath, tunPrefix, stack string, fakeIP bool) (any, error) {
	var generated []byte
	var err error
	if fakeIP {
		generated, err = config.GenerateFakeIPTUN(tunPrefix, stack, "WLAN", "以太网")
	} else {
		generated, err = config.GenerateDualDNSTUN(tunPrefix, stack, "WLAN", "以太网")
	}
	if err != nil {
		return nil, fmt.Errorf("generate DNS address-model config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 3*time.Second, []string{"example.com:80"}, nil, nil, []string{"example.com"}, nil, nil)
}

func runEncryptedDNSExperiment(corePath, tunPrefix, stack string) (any, error) {
	generated, err := config.GenerateEncryptedDNSTUN(tunPrefix, stack, "WLAN", "以太网")
	if err != nil {
		return nil, fmt.Errorf("generate encrypted DNS TUN config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 3*time.Second, nil, nil, nil, []string{"one.one.one.one", "dns.google"}, nil, nil)
}

func runIPv6Experiment(corePath, tunPrefix, stack string) (any, error) {
	generated, err := config.GenerateIPv6BlockedTUN(tunPrefix, stack, "WLAN", "以太网")
	if err != nil {
		return nil, fmt.Errorf("generate IPv6-blocked TUN config: %w", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	return core.RunTUNExperiment(ctx, corePath, generated, "WinRouter-TUN", tunPrefix, 3*time.Second, nil, nil, nil, nil, &core.IPv6Probe{DNSName: "one.one.one.one", ConnectionTarget: "[2606:4700:4700::1111]:443"}, nil)
}

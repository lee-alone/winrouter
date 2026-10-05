package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"time"

	"winrouter/internal/config"
	"winrouter/internal/core"
	"winrouter/internal/interfaces"
	"winrouter/internal/processrules"
	"winrouter/internal/routes"
	"winrouter/internal/tunprefix"
)

func main() {
	topology := flag.Bool("topology", false, "emit direct prefixes and overlap diagnostics")
	tunPrefix := flag.Bool("tun-prefix", false, "select a conflict-free TUN prefix")
	tunConfig := flag.Bool("tun-config", false, "emit a minimal sing-box TUN config")
	checkTUN := flag.Bool("check-tun", false, "validate the minimal config with the locked sing-box core")
	runTUN := flag.Bool("run-tun", false, "run a bounded elevated TUN lifecycle experiment")
	runDual := flag.Bool("run-dual", false, "run a bounded dual direct-outbound experiment")
	runLAN := flag.Bool("run-lan", false, "run a bounded dual LAN direct-outbound experiment")
	runDNS := flag.Bool("run-dns", false, "run a bounded dual DNS experiment")
	runMVPSemantic := flag.Bool("run-mvp-semantic", false, "validate final MVP routing and DNS semantics")
	runIPv6 := flag.Bool("run-ipv6", false, "run a bounded IPv6 blocking experiment")
	runEncryptedDNS := flag.Bool("run-encrypted-dns", false, "run a bounded DoH/DoT experiment")
	runRealIP := flag.Bool("run-realip", false, "run a real-IP DNS compatibility experiment")
	runFakeIP := flag.Bool("run-fakeip", false, "run a fake-IP DNS compatibility experiment")
	runProxy := flag.Bool("run-proxy", false, "run a bounded proxy loop-prevention experiment")
	runLifecycle := flag.Bool("run-lifecycle", false, "run repeated graceful TUN lifecycle experiments")
	runNetworkChange := flag.Bool("run-network-change", false, "stop safely when a selected interface changes")
	runSleepResume := flag.Bool("run-sleep-resume", false, "stop safely after detecting a sleep/resume gap")
	runProcess := flag.Bool("run-process", false, "verify a process rule with a real TUN connection")
	processRule := flag.String("process-rule", "name", "process identity for --run-process: name or path")
	inspectProcessName := flag.String("inspect-process-name", "", "inspect a running process by executable name")
	inspectProcessPath := flag.String("inspect-process-path", "", "inspect a running process by absolute executable path")
	cycles := flag.Int("cycles", 50, "number of lifecycle experiment cycles")
	holdSeconds := flag.Int("hold-seconds", 3, "seconds to hold a TUN experiment before stopping")
	preferred := flag.String("preferred", "", "previously selected TUN prefix")
	prefixState := flag.String("prefix-state", "", "optional path for persisted TUN prefix state")
	stack := flag.String("stack", config.TUNStackSystem, "TUN stack: system, gvisor, or mixed")
	corePath := flag.String("core", "resources/core/sing-box.exe", "path to sing-box executable")
	flag.Parse()

	adapters, err := interfaces.Enumerate()
	if err != nil {
		fmt.Fprintf(os.Stderr, "enumerate interfaces: %v\n", err)
		os.Exit(1)
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	output := any(adapters)
	if *tunPrefix || *tunConfig || *checkTUN || *runTUN || *runDual || *runLAN || *runDNS || *runMVPSemantic || *runIPv6 || *runEncryptedDNS || *runRealIP || *runFakeIP || *runProxy || *runLifecycle || *runNetworkChange || *runSleepResume || *runProcess {
		routeTable, err := routes.Enumerate()
		if err != nil {
			fmt.Fprintf(os.Stderr, "enumerate routes: %v\n", err)
			os.Exit(1)
		}
		preferredPrefix := *preferred
		if preferredPrefix == "" && *prefixState != "" {
			state, stateErr := tunprefix.LoadState(*prefixState)
			if stateErr != nil {
				fmt.Fprintf(os.Stderr, "load TUN prefix state: %v\n", stateErr)
				os.Exit(1)
			}
			preferredPrefix = state.Prefix
		}
		allocation, err := tunprefix.Allocate(tunprefix.DefaultPool, preferredPrefix, adapters, routeTable)
		if err != nil {
			fmt.Fprintf(os.Stderr, "allocate TUN prefix: %v\n", err)
			os.Exit(1)
		}
		if *prefixState != "" {
			if err := tunprefix.SaveState(*prefixState, allocation.Prefix); err != nil {
				fmt.Fprintf(os.Stderr, "save TUN prefix state: %v\n", err)
				os.Exit(1)
			}
		}
		output = allocation
		if *tunConfig || *checkTUN || *runTUN || *runDual || *runLAN || *runDNS || *runMVPSemantic || *runIPv6 || *runEncryptedDNS || *runRealIP || *runFakeIP || *runProxy || *runLifecycle || *runNetworkChange || *runSleepResume || *runProcess {
			generated, err := config.GenerateMinimalTUN(allocation.Prefix, *stack)
			if err != nil {
				fmt.Fprintf(os.Stderr, "generate minimal TUN config: %v\n", err)
				os.Exit(1)
			}
			if *runProcess {
				candidates := activeCandidates(adapters)
				if len(candidates) < 2 {
					fmt.Fprintln(os.Stderr, "process validation requires two active candidate interfaces")
					os.Exit(1)
				}
				executable, pathErr := os.Executable()
				if pathErr != nil {
					fmt.Fprintf(os.Stderr, "resolve probe executable: %v\n", pathErr)
					os.Exit(1)
				}
				executable, pathErr = filepath.Abs(executable)
				if pathErr != nil {
					fmt.Fprintf(os.Stderr, "resolve absolute probe executable: %v\n", pathErr)
					os.Exit(1)
				}
				ruleType, ruleValue := "process-name", strings.ToLower(filepath.Base(executable))
				if *processRule == "path" {
					ruleType, ruleValue = "process-path", executable
				} else if *processRule != "name" {
					fmt.Fprintln(os.Stderr, "process-rule must be name or path")
					os.Exit(1)
				}
				input := processExperimentInput(allocation.Prefix, *stack, candidates[:2], config.MVPCustomRule{Name: "probe process to A", Type: ruleType, Value: ruleValue, Action: "a"})
				mvp, generateErr := config.GenerateMVP(input)
				if generateErr != nil {
					fmt.Fprintf(os.Stderr, "generate process experiment config: %v\n", generateErr)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				experiment, runErr := core.RunTUNExperiment(ctx, *corePath, mvp.JSON, "WinRouter-TUN", allocation.Prefix, time.Second, []string{"1.1.1.1:443"}, nil, nil, nil, nil, nil)
				if runErr != nil {
					fmt.Fprintf(os.Stderr, "run process routing experiment: %v\n", runErr)
					os.Exit(1)
				}
				const expectedLog = "outbound/direct[domestic-direct]: outbound connection to 1.1.1.1:443"
				matched := strings.Contains(experiment.CoreOutput, expectedLog)
				if !matched {
					fmt.Fprintln(os.Stderr, "process rule did not route the probe target through domestic-direct")
					os.Exit(1)
				}
				output = struct {
					RuleType         string               `json:"rule_type"`
					RuleValue        string               `json:"rule_value"`
					ExpectedOutbound string               `json:"expected_outbound"`
					Matched          bool                 `json:"matched"`
					Interfaces       []interfaces.Adapter `json:"interfaces"`
					Experiment       core.TUNExperiment   `json:"experiment"`
				}{RuleType: ruleType, RuleValue: ruleValue, ExpectedOutbound: "domestic-direct", Matched: matched, Interfaces: candidates[:2], Experiment: experiment}
			} else if *runMVPSemantic {
				candidates := make([]interfaces.Adapter, 0, 2)
				for _, adapter := range adapters {
					if adapter.Candidate && adapter.Status == "up" && len(adapter.Addresses) > 0 {
						candidates = append(candidates, adapter)
					}
				}
				if len(candidates) < 2 {
					fmt.Fprintln(os.Stderr, "final MVP validation requires two active candidate interfaces")
					os.Exit(1)
				}
				input := config.MVPConfig{
					SchemaVersion: config.SchemaVersion1,
					TUN:           config.MVPTUN{Prefix: allocation.Prefix, Stack: *stack},
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
					fmt.Fprintf(os.Stderr, "generate final MVP config: %v\n", generateErr)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				experiment, runErr := core.RunTUNExperiment(ctx, *corePath, mvp.JSON, "WinRouter-TUN", allocation.Prefix, 3*time.Second, []string{"223.5.5.5:443", "1.1.1.1:443"}, nil, nil, []string{"www.baidu.com", "example.com"}, nil, nil)
				if runErr != nil {
					fmt.Fprintf(os.Stderr, "run final MVP semantic experiment: %v\n", runErr)
					os.Exit(1)
				}
				output = struct {
					Input      config.MVPConfig     `json:"mvp_input"`
					Interfaces []interfaces.Adapter `json:"interfaces"`
					Rules      []string             `json:"rule_categories"`
					Experiment core.TUNExperiment   `json:"experiment"`
				}{Input: input, Interfaces: candidates[:2], Rules: mvp.RuleCategories, Experiment: experiment}
			} else if *runSleepResume {
				generated, err = config.GenerateDualDirectTUN(allocation.Prefix, *stack, "WLAN", "以太网", "1.1.1.1/32", "8.8.8.8/32", "162.159.200.1/32", "162.159.200.123/32")
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate sleep/resume config: %v\n", err)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 9*time.Minute, nil, nil, nil, nil, nil, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "run sleep/resume experiment: %v\n", err)
					os.Exit(1)
				}
				if !experiment.SleepResumeDetected {
					fmt.Fprintln(os.Stderr, "sleep/resume experiment timed out without a suspend gap")
					os.Exit(1)
				}
				output = experiment
			} else if *runNetworkChange {
				generated, err = config.GenerateDualDirectTUN(allocation.Prefix, *stack, "WLAN", "以太网", "1.1.1.1/32", "8.8.8.8/32", "162.159.200.1/32", "162.159.200.123/32")
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate network-change config: %v\n", err)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 60*time.Second, nil, nil, nil, nil, nil, []string{"WLAN", "以太网"})
				if err != nil {
					fmt.Fprintf(os.Stderr, "run network-change experiment: %v\n", err)
					os.Exit(1)
				}
				if !experiment.NetworkChangeDetected {
					fmt.Fprintln(os.Stderr, "network-change experiment timed out without an interface change")
					os.Exit(1)
				}
				output = experiment
			} else if *runLifecycle {
				if *cycles < 1 || *cycles > 100 {
					fmt.Fprintln(os.Stderr, "cycles must be between 1 and 100")
					os.Exit(1)
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
				}{Requested: *cycles}
				for cycle := 1; cycle <= *cycles; cycle++ {
					ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
					experiment, runErr := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 0, nil, nil, nil, nil, nil, nil)
					cancel()
					if runErr != nil {
						fmt.Fprintf(os.Stderr, "lifecycle cycle %d: %v\n", cycle, runErr)
						os.Exit(1)
					}
					report.Completed = cycle
					report.Cycles = append(report.Cycles, cycleResult{
						Cycle: cycle, StopMethod: experiment.StopMethod, GracefulStop: experiment.GracefulStop,
						InterfaceCleaned: experiment.InterfaceCleaned, RoutesCleaned: experiment.RoutesCleaned,
						ElapsedMillis: experiment.ElapsedMillis,
					})
				}
				output = report
			} else if *runDual {
				generated, err = config.GenerateDualDirectTUN(allocation.Prefix, *stack, "WLAN", "以太网", "1.1.1.1/32", "8.8.8.8/32", "162.159.200.1/32", "162.159.200.123/32")
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate dual direct TUN config: %v\n", err)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 3*time.Second, []string{"1.1.1.1:443", "8.8.8.8:443"}, []string{"162.159.200.1:123", "162.159.200.123:123"}, nil, nil, nil, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "run dual TUN experiment: %v\n", err)
					os.Exit(1)
				}
				output = experiment
			} else if *runLAN {
				generated, err = config.GenerateDualLANDirectTUN(allocation.Prefix, *stack, "WLAN", "以太网", "10.12.85.0/24", "192.168.1.0/24")
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate dual LAN TUN config: %v\n", err)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 3*time.Second, nil, nil, []string{"10.12.85.232", "192.168.1.1"}, nil, nil, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "run dual LAN TUN experiment: %v\n", err)
					os.Exit(1)
				}
				output = experiment
			} else if *runDNS {
				generated, err = config.GenerateDualDNSTUN(allocation.Prefix, *stack, "WLAN", "以太网")
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate dual DNS TUN config: %v\n", err)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 3*time.Second, nil, nil, nil, []string{"one.one.one.one", "dns.google"}, nil, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "run dual DNS TUN experiment: %v\n", err)
					os.Exit(1)
				}
				output = experiment
			} else if *runProxy {
				const proxyAddress = "192.168.1.166:18080"
				proxy, err := core.StartMockHTTPProxy(proxyAddress)
				if err != nil {
					fmt.Fprintf(os.Stderr, "start controlled proxy: %v\n", err)
					os.Exit(1)
				}
				defer proxy.Close()
				generated, err = config.GenerateDomainProxyLoopTUN(allocation.Prefix, *stack, "WLAN", "以太网", "proxy.winrouter.test", "192.168.1.166", 18080, "1.1.1.1/32")
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate proxy-loop config: %v\n", err)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 3*time.Second, []string{"1.1.1.1:443"}, nil, nil, nil, nil, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "run proxy-loop experiment: %v\n", err)
					os.Exit(1)
				}
				output = struct {
					Experiment       core.TUNExperiment `json:"experiment"`
					ProxyConnections int64              `json:"proxy_connections"`
				}{Experiment: experiment, ProxyConnections: proxy.Connections()}
			} else if *runRealIP || *runFakeIP {
				if *runFakeIP {
					generated, err = config.GenerateFakeIPTUN(allocation.Prefix, *stack, "WLAN", "以太网")
				} else {
					generated, err = config.GenerateDualDNSTUN(allocation.Prefix, *stack, "WLAN", "以太网")
				}
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate DNS address-model config: %v\n", err)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 3*time.Second, []string{"example.com:80"}, nil, nil, []string{"example.com"}, nil, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "run DNS address-model experiment: %v\n", err)
					os.Exit(1)
				}
				output = experiment
			} else if *runEncryptedDNS {
				generated, err = config.GenerateEncryptedDNSTUN(allocation.Prefix, *stack, "WLAN", "以太网")
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate encrypted DNS TUN config: %v\n", err)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 3*time.Second, nil, nil, nil, []string{"one.one.one.one", "dns.google"}, nil, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "run encrypted DNS TUN experiment: %v\n", err)
					os.Exit(1)
				}
				output = experiment
			} else if *runIPv6 {
				generated, err = config.GenerateIPv6BlockedTUN(allocation.Prefix, *stack, "WLAN", "以太网")
				if err != nil {
					fmt.Fprintf(os.Stderr, "generate IPv6-blocked TUN config: %v\n", err)
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, 3*time.Second, nil, nil, nil, nil, &core.IPv6Probe{DNSName: "one.one.one.one", ConnectionTarget: "[2606:4700:4700::1111]:443"}, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "run IPv6 TUN experiment: %v\n", err)
					os.Exit(1)
				}
				output = experiment
			} else if *runTUN {
				if *holdSeconds < 0 || *holdSeconds > 300 {
					fmt.Fprintln(os.Stderr, "hold-seconds must be between 0 and 300")
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*holdSeconds)*time.Second+30*time.Second)
				defer cancel()
				experiment, err := core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, time.Duration(*holdSeconds)*time.Second, nil, nil, nil, nil, nil, nil)
				if err != nil {
					fmt.Fprintf(os.Stderr, "run TUN experiment: %v\n", err)
					os.Exit(1)
				}
				output = experiment
			} else if *checkTUN {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				validation, err := core.ValidateCore(ctx, *corePath, generated)
				if err != nil {
					fmt.Fprintf(os.Stderr, "validate minimal TUN config: %v\n", err)
					os.Exit(1)
				}
				output = validation
			} else {
				var model config.MinimalTUN
				if err := json.Unmarshal(generated, &model); err != nil {
					fmt.Fprintf(os.Stderr, "decode generated TUN config: %v\n", err)
					os.Exit(1)
				}
				output = model
			}
		}
	} else if *topology {
		output = interfaces.BuildTopology(adapters)
	} else if *inspectProcessName != "" || *inspectProcessPath != "" {
		identities := []processrules.Identity{}
		if *inspectProcessName != "" {
			identities = append(identities, processrules.Identity{Type: "process-name", Value: *inspectProcessName})
		}
		if *inspectProcessPath != "" {
			identities = append(identities, processrules.Identity{Type: "process-path", Value: *inspectProcessPath})
		}
		statuses, inspectErr := processrules.Inspect(identities)
		if inspectErr != nil {
			fmt.Fprintf(os.Stderr, "inspect processes: %v\n", inspectErr)
			os.Exit(1)
		}
		output = statuses
	}
	if err := encoder.Encode(output); err != nil {
		fmt.Fprintf(os.Stderr, "encode result: %v\n", err)
		os.Exit(1)
	}
}

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

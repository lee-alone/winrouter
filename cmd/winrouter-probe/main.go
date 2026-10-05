package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
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

			var expErr error
			switch {
			case *runProcess:
				output, expErr = runProcessExperiment(*corePath, allocation.Prefix, *stack, adapters, *processRule)
			case *runMVPSemantic:
				output, expErr = runMVPSemanticExperiment(*corePath, allocation.Prefix, *stack, adapters)
			case *runSleepResume:
				output, expErr = runSleepResumeExperiment(*corePath, allocation.Prefix, *stack)
			case *runNetworkChange:
				output, expErr = runNetworkChangeExperiment(*corePath, allocation.Prefix, *stack)
			case *runLifecycle:
				output, expErr = runLifecycleExperiment(*corePath, allocation.Prefix, generated, *cycles)
			case *runDual:
				output, expErr = runDualExperiment(*corePath, allocation.Prefix, *stack)
			case *runLAN:
				output, expErr = runLANExperiment(*corePath, allocation.Prefix, *stack)
			case *runDNS:
				output, expErr = runDNSExperiment(*corePath, allocation.Prefix, *stack)
			case *runProxy:
				output, expErr = runProxyExperiment(*corePath, allocation.Prefix, *stack)
			case *runRealIP || *runFakeIP:
				output, expErr = runDNSAddressModelExperiment(*corePath, allocation.Prefix, *stack, *runFakeIP)
			case *runEncryptedDNS:
				output, expErr = runEncryptedDNSExperiment(*corePath, allocation.Prefix, *stack)
			case *runIPv6:
				output, expErr = runIPv6Experiment(*corePath, allocation.Prefix, *stack)
			case *runTUN:
				if *holdSeconds < 0 || *holdSeconds > 300 {
					fmt.Fprintln(os.Stderr, "hold-seconds must be between 0 and 300")
					os.Exit(1)
				}
				ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*holdSeconds)*time.Second+30*time.Second)
				defer cancel()
				output, expErr = core.RunTUNExperiment(ctx, *corePath, generated, "WinRouter-TUN", allocation.Prefix, time.Duration(*holdSeconds)*time.Second, nil, nil, nil, nil, nil, nil)
			case *checkTUN:
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				output, expErr = core.ValidateCore(ctx, *corePath, generated)
			default:
				var model config.MinimalTUN
				if err := json.Unmarshal(generated, &model); err != nil {
					fmt.Fprintf(os.Stderr, "decode generated TUN config: %v\n", err)
					os.Exit(1)
				}
				output = model
			}

			if expErr != nil {
				fmt.Fprintf(os.Stderr, "experiment error: %v\n", expErr)
				os.Exit(1)
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

package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/netip"
	"os"
	"time"

	"winrouter/internal/config"
	"winrouter/internal/core"
	"winrouter/internal/helperclient"
	"winrouter/internal/helperipc"
	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
	"winrouter/internal/tunprefix"
)

type report struct {
	InterfaceA string       `json:"interface_a"`
	InterfaceB string       `json:"interface_b"`
	Applied    core.Status  `json:"applied"`
	Stopped    core.Status  `json:"stopped"`
	TUNCleaned bool         `json:"tun_cleaned"`
	Requested  int          `json:"requested"`
	Completed  int          `json:"completed"`
	Cycles     []cycle      `json:"cycles,omitempty"`
	Crash      *crashResult `json:"crash,omitempty"`
}

type crashResult struct {
	OriginalPID      int         `json:"original_pid"`
	Restarted        core.Status `json:"restarted"`
	OldProcessExited bool        `json:"old_process_exited"`
}

type cycle struct {
	Number        int    `json:"number"`
	Generation    uint64 `json:"generation"`
	PID           int    `json:"pid"`
	Cleaned       bool   `json:"cleaned"`
	CleanupWaitMS int64  `json:"cleanup_wait_ms"`
	ElapsedMS     int64  `json:"elapsed_ms"`
}

func main() {
	cycles := flag.Int("cycles", 1, "number of MVP apply/stop cleanup cycles (1-100)")
	crashTest := flag.Bool("crash-test", false, "terminate the production core and verify bounded restart")
	flag.Parse()
	if *cycles < 1 || *cycles > 100 {
		fail(fmt.Errorf("cycles must be between 1 and 100"))
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(*cycles)*20*time.Second+time.Minute)
	defer cancel()
	adapters, err := interfaces.Enumerate()
	if err != nil {
		fail(err)
	}
	candidates := make([]interfaces.Adapter, 0, 2)
	for _, adapter := range adapters {
		if adapter.Candidate && len(adapter.Addresses) > 0 {
			candidates = append(candidates, adapter)
		}
	}
	if len(candidates) < 2 {
		fail(fmt.Errorf("two identified physical interfaces are required"))
	}
	if candidates[0].Status != "up" && candidates[1].Status != "up" {
		fail(fmt.Errorf("at least one physical interface must be active"))
	}
	routeTable, err := routes.Enumerate()
	if err != nil {
		fail(err)
	}
	allocation, err := tunprefix.Allocate(tunprefix.DefaultPool, "", adapters, routeTable)
	if err != nil {
		fail(err)
	}
	input := config.MVPConfig{
		SchemaVersion: config.SchemaVersion1, Mode: config.ModeDirectSplit,
		TUN:        config.MVPTUN{Prefix: allocation.Prefix, Stack: config.TUNStackSystem},
		InterfaceA: config.MVPInterface{GUID: candidates[0].GUID, BindInterface: candidates[0].FriendlyName},
		InterfaceB: config.MVPInterface{GUID: candidates[1].GUID, BindInterface: candidates[1].FriendlyName},
		Domestic:   config.MVPDomestic{CIDRs: []string{"1.0.1.0/24"}, DomainSuffixes: []string{"example.cn"}},
		DNS:        config.MVPDNS{Domestic: config.MVPDNSServer{Type: "udp", Server: "223.5.5.5", Port: 53}, Global: config.MVPDNSServer{Type: "tls", Server: "1.1.1.1", Port: 853, ServerName: "cloudflare-dns.com"}},
		IPv6:       config.IPv6Block,
	}
	for _, prefix := range interfaces.BuildTopology(candidates[:2]).Prefixes {
		parsed, _ := netip.ParsePrefix(prefix.Prefix)
		if prefix.Action == interfaces.PrefixBindInterface && parsed.Addr().Is4() {
			input.DirectPrefixes = append(input.DirectPrefixes, config.MVPDirectPrefix{Prefix: prefix.Prefix, BindInterface: prefix.AdapterName})
		}
	}
	if _, err := config.GenerateMVP(input); err != nil {
		fail(err)
	}
	var session *helperclient.Session
	if *crashTest {
		session, err = helperclient.LaunchFaultTest(ctx)
	} else {
		session, err = helperclient.Launch(ctx)
	}
	if err != nil {
		fail(err)
	}
	defer session.Close()
	data, err := json.Marshal(input)
	if err != nil {
		fail(err)
	}
	result := report{InterfaceA: candidates[0].FriendlyName, InterfaceB: candidates[1].FriendlyName, Requested: *cycles, TUNCleaned: true, Cycles: make([]cycle, 0, *cycles)}
	for number := 1; number <= *cycles; number++ {
		started := time.Now()
		var applied core.Status
		if err := session.Client.Call(helperipc.MethodApply, helperipc.ConfigParams{Config: data}, &applied); err != nil {
			fail(fmt.Errorf("cycle %d apply: %w", number, err))
		}
		if *crashTest {
			var terminated core.Status
			if err := session.Client.Call(helperipc.MethodFaultTerminateCore, nil, &terminated); err != nil {
				fail(fmt.Errorf("terminate core: %w", err))
			}
			restarted, err := waitForRestart(session, applied.PID, 15*time.Second)
			if err != nil {
				fail(err)
			}
			result.Crash = &crashResult{OriginalPID: applied.PID, Restarted: restarted, OldProcessExited: true}
		}
		var stopped core.Status
		if err := session.Client.Call(helperipc.MethodStop, nil, &stopped); err != nil {
			fail(fmt.Errorf("cycle %d stop: %w", number, err))
		}
		cleanupStarted := time.Now()
		cleaned, err := waitForTUNCleanup(5 * time.Second)
		if err != nil {
			fail(err)
		}
		result.Applied, result.Stopped = applied, stopped
		result.Cycles = append(result.Cycles, cycle{Number: number, Generation: applied.Generation, PID: applied.PID, Cleaned: cleaned, CleanupWaitMS: time.Since(cleanupStarted).Milliseconds(), ElapsedMS: time.Since(started).Milliseconds()})
		if applied.State != core.StateRunning || stopped.State != core.StateStopped || !cleaned {
			result.TUNCleaned = false
			break
		}
		result.Completed = number
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(result)
	if result.Completed != result.Requested || !result.TUNCleaned {
		os.Exit(2)
	}
}

func waitForRestart(session *helperclient.Session, originalPID int, timeout time.Duration) (core.Status, error) {
	deadline := time.Now().Add(timeout)
	for {
		var status core.Status
		if err := session.Client.Call(helperipc.MethodStatus, nil, &status); err != nil {
			return core.Status{}, err
		}
		if status.State == core.StateRunning && status.PID != 0 && status.PID != originalPID && status.RestartAttempts == 1 && status.Abnormal {
			return status, nil
		}
		if time.Now().After(deadline) {
			return status, fmt.Errorf("core did not restart after crash: %#v", status)
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func waitForTUNCleanup(timeout time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for {
		remaining, err := interfaces.Enumerate()
		if err != nil {
			return false, err
		}
		found := false
		for _, adapter := range remaining {
			if adapter.FriendlyName == "WinRouter-TUN" {
				found = true
				break
			}
		}
		if !found {
			return true, nil
		}
		if time.Now().After(deadline) {
			return false, nil
		}
		time.Sleep(100 * time.Millisecond)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

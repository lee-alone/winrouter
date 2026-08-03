package interfacemanager

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
	"winrouter/internal/tunprefix"
)

func TestManagerSelectPersistsAndResolvesByGUID(t *testing.T) {
	first := adapter("{A}", "00:00:00:00:00:01", "Ethernet A", "192.168.10.2", 24)
	second := adapter("{B}", "00:00:00:00:00:02", "Ethernet B", "192.168.20.2", 24)
	path := filepath.Join(t.TempDir(), "interfaces.json")
	manager := newTestManager(t, path, func() []interfaces.Adapter { return []interfaces.Adapter{first, second} }, nil)

	snapshot, err := manager.Select(first, second)
	if err != nil {
		t.Fatalf("Select() error: %v", err)
	}
	if snapshot.InterfaceA.Status != "resolved" || snapshot.InterfaceB.Status != "resolved" {
		t.Fatalf("selection = %#v / %#v", snapshot.InterfaceA, snapshot.InterfaceB)
	}
	state, err := LoadState(path)
	if err != nil {
		t.Fatalf("LoadState() error: %v", err)
	}
	if state.InterfaceA.GUID != first.GUID || state.InterfaceB.GUID != second.GUID || state.TUNPrefix == "" {
		t.Fatalf("state = %#v", state)
	}
}

func TestManagerRequiresSelectionWhenInterfaceDisappears(t *testing.T) {
	first := adapter("{A}", "00:00:00:00:00:01", "Ethernet A", "192.168.10.2", 24)
	second := adapter("{B}", "00:00:00:00:00:02", "Ethernet B", "192.168.20.2", 24)
	current := []interfaces.Adapter{first, second}
	manager := newTestManager(t, "", func() []interfaces.Adapter { return current }, nil)
	if _, err := manager.Select(first, second); err != nil {
		t.Fatal(err)
	}
	current = []interfaces.Adapter{second}
	snapshot, err := manager.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.InterfaceA.Status != "selection-required" || snapshot.InterfaceA.Error == "" {
		t.Fatalf("interface A = %#v", snapshot.InterfaceA)
	}
}

func TestManagerUsesAuxiliaryEvidenceAfterGUIDChange(t *testing.T) {
	old := adapter("{OLD}", "00:00:00:00:00:01", "Ethernet A", "192.168.10.2", 24)
	second := adapter("{B}", "00:00:00:00:00:02", "Ethernet B", "192.168.20.2", 24)
	current := []interfaces.Adapter{old, second}
	manager := newTestManager(t, "", func() []interfaces.Adapter { return current }, nil)
	if _, err := manager.Select(old, second); err != nil {
		t.Fatal(err)
	}
	replacement := old
	replacement.GUID = "{NEW}"
	current = []interfaces.Adapter{replacement, second}
	snapshot, err := manager.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.InterfaceA.Match == nil || snapshot.InterfaceA.Match.Method != interfaces.MatchByAuxiliary || snapshot.InterfaceA.Match.Adapter.GUID != "{NEW}" {
		t.Fatalf("interface A = %#v", snapshot.InterfaceA)
	}
}

func TestManagerRejectsSameAndVirtualInterfaces(t *testing.T) {
	physical := adapter("{A}", "00:00:00:00:00:01", "Ethernet A", "192.168.10.2", 24)
	virtual := adapter("{V}", "", "vEthernet", "172.20.0.1", 20)
	virtual.Kind = interfaces.KindHyperV
	virtual.Candidate = false
	manager := newTestManager(t, "", func() []interfaces.Adapter { return []interfaces.Adapter{physical, virtual} }, nil)
	if _, err := manager.Select(physical, physical); !errors.Is(err, ErrSameAdapter) {
		t.Fatalf("same interface error = %v", err)
	}
	if _, err := manager.Select(physical, virtual); !errors.Is(err, ErrSelectionInvalid) {
		t.Fatalf("virtual interface error = %v", err)
	}
}

func TestBuildCandidatesFiltersAndRanks(t *testing.T) {
	good := adapter("{A}", "", "Ethernet A", "192.168.10.2", 24)
	good.IPv4Metric = 10
	down := adapter("{B}", "", "Ethernet B", "192.168.20.2", 24)
	down.Status = "down"
	virtual := adapter("{V}", "", "VPN", "10.0.0.2", 24)
	virtual.Kind = interfaces.KindVPN
	virtual.Candidate = false
	candidates := BuildCandidates([]interfaces.Adapter{down, virtual, good})
	if len(candidates) != 2 || candidates[0].Adapter.GUID != good.GUID || !candidates[0].Eligible || candidates[1].Eligible {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestBuildCandidatesRequiresIPv4GatewayButAcceptsDualStackGateway(t *testing.T) {
	ipv6Only := adapter("{V6}", "", "IPv6 gateway only", "192.168.30.2", 24)
	ipv6Only.Gateways = []string{"fe80::1"}
	dualStack := adapter("{DUAL}", "", "Dual stack", "192.168.40.2", 24)
	dualStack.Gateways = []string{"fe80::2", "192.168.40.1"}
	candidates := BuildCandidates([]interfaces.Adapter{ipv6Only, dualStack})
	if len(candidates) != 2 || !candidates[0].Eligible || candidates[0].Adapter.GUID != dualStack.GUID || candidates[1].Eligible {
		t.Fatalf("candidates = %#v", candidates)
	}
}

func TestSelectedInterfacesRequireIndependentIPv4DefaultRoutes(t *testing.T) {
	first := adapter("{A}", "", "Ethernet A", "192.168.10.2", 24)
	first.Index = 10
	second := adapter("{B}", "", "Ethernet B", "192.168.20.2", 24)
	second.Index = 20
	routeTable := []routes.Route{{Prefix: "0.0.0.0/0", InterfaceIndex: first.Index}}
	manager := newTestManager(t, "", func() []interfaces.Adapter { return []interfaces.Adapter{first, second} }, routeTable)
	snapshot, err := manager.Select(first, second)
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnostic(t, snapshot.Diagnostics, "missing-default-route-b")
	routeTable = append(routeTable, routes.Route{Prefix: "0.0.0.0/0", InterfaceIndex: second.Index})
	manager.options.EnumerateRoutes = func() ([]routes.Route, error) { return routeTable, nil }
	snapshot, err = manager.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	assertNoDiagnostic(t, snapshot.Diagnostics, "missing-default-route-a")
	assertNoDiagnostic(t, snapshot.Diagnostics, "missing-default-route-b")
}

func TestManagerReportsOverlapAndPoolExhaustion(t *testing.T) {
	first := adapter("{A}", "", "Ethernet A", "192.168.10.2", 24)
	second := adapter("{B}", "", "Ethernet B", "192.168.10.3", 24)
	pool := []string{"172.19.0.0/30"}
	routeTable := []routes.Route{{Prefix: "172.19.0.0/24", InterfaceIndex: 9}}
	manager := newTestManager(t, "", func() []interfaces.Adapter { return []interfaces.Adapter{first, second} }, routeTable)
	manager.options.TUNPool = pool
	snapshot, err := manager.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.TUN != nil {
		t.Fatalf("TUN = %#v, want nil", snapshot.TUN)
	}
	assertDiagnostic(t, snapshot.Diagnostics, "prefix-overlap")
	assertDiagnostic(t, snapshot.Diagnostics, "tun-pool-exhausted")
}

func TestManagerRunPublishesNetworkChange(t *testing.T) {
	first := adapter("{A}", "", "Ethernet A", "192.168.10.2", 24)
	var mu sync.Mutex
	current := []interfaces.Adapter{first}
	manager := newTestManager(t, "", func() []interfaces.Adapter {
		mu.Lock()
		defer mu.Unlock()
		return append([]interfaces.Adapter(nil), current...)
	}, nil)
	manager.options.PollInterval = 5 * time.Millisecond
	events, unsubscribe := manager.Subscribe(1)
	defer unsubscribe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()
	waitForSequence(t, manager)
	mu.Lock()
	current[0].Status = "down"
	mu.Unlock()
	select {
	case event := <-events:
		if event.Reason != "network-change" {
			t.Fatalf("reason = %q", event.Reason)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for network change")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestManagerRunPublishesDefaultRouteChange(t *testing.T) {
	first := adapter("{A}", "", "Ethernet A", "192.168.10.2", 24)
	second := adapter("{B}", "", "Ethernet B", "192.168.20.2", 24)
	first.Index = 1
	second.Index = 2
	var mu sync.Mutex
	routeTable := []routes.Route{
		{Prefix: "0.0.0.0/0", InterfaceIndex: first.Index, Metric: 25},
		{Prefix: "0.0.0.0/0", InterfaceIndex: second.Index, Metric: 25},
	}
	manager, err := New(Options{
		EnumerateInterfaces: func() ([]interfaces.Adapter, error) { return []interfaces.Adapter{first, second}, nil },
		EnumerateRoutes: func() ([]routes.Route, error) {
			mu.Lock()
			defer mu.Unlock()
			return append([]routes.Route(nil), routeTable...), nil
		},
		TUNPool: []string{"172.19.0.0/30"}, PollInterval: 5 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := manager.Select(first, second); err != nil {
		t.Fatal(err)
	}
	events, unsubscribe := manager.Subscribe(1)
	defer unsubscribe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()
	waitForSequence(t, manager)
	mu.Lock()
	routeTable = routeTable[1:]
	mu.Unlock()
	select {
	case event := <-events:
		if len(event.Snapshot.Routes) != 1 || event.Snapshot.Routes[0].InterfaceIndex != second.Index {
			t.Fatalf("routes = %#v", event.Snapshot.Routes)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for default route change")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestManagerRunIgnoresUnselectedVirtualInterfaceChanges(t *testing.T) {
	physical := adapter("{A}", "00:00:00:00:00:01", "Ethernet A", "192.168.10.2", 24)
	virtual := adapter("{V}", "", "vEthernet (WSL)", "172.20.0.1", 20)
	virtual.Kind = interfaces.KindWSL
	virtual.Candidate = false
	var mu sync.Mutex
	current := []interfaces.Adapter{physical, virtual}
	manager := newTestManager(t, "", func() []interfaces.Adapter {
		mu.Lock()
		defer mu.Unlock()
		return append([]interfaces.Adapter(nil), current...)
	}, []routes.Route{{Prefix: "0.0.0.0/0", InterfaceIndex: physical.Index}})
	manager.options.PollInterval = 5 * time.Millisecond
	events, unsubscribe := manager.Subscribe(1)
	defer unsubscribe()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- manager.Run(ctx) }()
	waitForSequence(t, manager)

	mu.Lock()
	current[1].Addresses = []interfaces.Address{{IP: "172.20.0.2", PrefixLength: 20}}
	mu.Unlock()
	select {
	case event := <-events:
		t.Fatalf("unexpected event for unselected virtual interface: %#v", event)
	case <-time.After(30 * time.Millisecond):
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatalf("Run() error: %v", err)
	}
}

func TestFingerprintNormalizesOrderingAndTracksRelevantChanges(t *testing.T) {
	first := adapter("{A}", "00:00:00:00:00:01", "Ethernet A", "192.168.10.2", 24)
	first.Addresses = append(first.Addresses, interfaces.Address{IP: "2001:db8::2", PrefixLength: 64})
	first.Gateways = []string{"192.168.10.1", "fe80::1"}
	first.DNSServers = []string{"1.1.1.1", "2606:4700:4700::1111"}
	second := adapter("{B}", "00:00:00:00:00:02", "Ethernet B", "192.168.20.2", 24)
	virtual := adapter("{V}", "", "vEthernet (WSL)", "172.20.0.1", 20)
	virtual.Kind = interfaces.KindWSL
	virtual.Candidate = false
	base := Snapshot{
		Adapters: []interfaces.Adapter{first, second, virtual},
		Routes: []routes.Route{
			{Prefix: "0.0.0.0/0", InterfaceIndex: first.Index, Metric: 10},
			{Prefix: "0.0.0.0/0", InterfaceIndex: second.Index, Metric: 20},
			{Prefix: "172.20.0.0/20", InterfaceIndex: virtual.Index},
		},
		InterfaceA: ResolvedSelection{Role: "A", Saved: interfaces.Identity{GUID: first.GUID}, Match: &interfaces.Match{Adapter: first}, Status: "resolved"},
		InterfaceB: ResolvedSelection{Role: "B", Saved: interfaces.Identity{GUID: second.GUID}, Match: &interfaces.Match{Adapter: second}, Status: "resolved"},
		TUN:        &tunprefix.Allocation{Prefix: "172.19.0.0/30"},
	}
	reordered := base
	reordered.Adapters = append([]interfaces.Adapter(nil), base.Adapters...)
	reordered.Adapters[0].Addresses = []interfaces.Address{first.Addresses[1], first.Addresses[0]}
	reordered.Adapters[0].Gateways = []string{first.Gateways[1], first.Gateways[0]}
	reordered.Adapters[0].DNSServers = []string{first.DNSServers[1], first.DNSServers[0]}
	reordered.Routes = []routes.Route{base.Routes[2], base.Routes[1], base.Routes[0]}
	reordered.Adapters[2].Addresses = []interfaces.Address{{IP: "172.20.0.2", PrefixLength: 20}}
	if fingerprint(base) != fingerprint(reordered) {
		t.Fatal("ordering or unselected virtual interface change altered fingerprint")
	}

	changed := base
	changed.Adapters = append([]interfaces.Adapter(nil), base.Adapters...)
	changed.Adapters[0].Gateways = []string{"192.168.10.254", "fe80::1"}
	if fingerprint(base) == fingerprint(changed) {
		t.Fatal("selected physical gateway change did not alter fingerprint")
	}
}

func TestPoolExhaustionErrorRemainsInspectable(t *testing.T) {
	manager := newTestManager(t, "", func() []interfaces.Adapter { return nil }, []routes.Route{{Prefix: "172.19.0.0/24"}})
	manager.options.TUNPool = []string{"172.19.0.0/30"}
	snapshot, err := manager.Refresh()
	if err != nil {
		t.Fatal(err)
	}
	assertDiagnostic(t, snapshot.Diagnostics, "tun-pool-exhausted")
	_, allocationErr := tunprefix.Allocate(manager.options.TUNPool, "", nil, []routes.Route{{Prefix: "172.19.0.0/24"}})
	if !errors.Is(allocationErr, tunprefix.ErrPoolExhausted) {
		t.Fatalf("allocation error = %v", allocationErr)
	}
}

func newTestManager(t *testing.T, path string, enumerate func() []interfaces.Adapter, routeTable []routes.Route) *Manager {
	t.Helper()
	manager, err := New(Options{StatePath: path, EnumerateInterfaces: func() ([]interfaces.Adapter, error) { return enumerate(), nil }, EnumerateRoutes: func() ([]routes.Route, error) { return routeTable, nil }, TUNPool: []string{"172.19.0.0/30"}})
	if err != nil {
		t.Fatalf("New() error: %v", err)
	}
	return manager
}

func adapter(guid, mac, name, ip string, prefix uint8) interfaces.Adapter {
	return interfaces.Adapter{GUID: guid, LUID: 1, Index: 1, MAC: mac, FriendlyName: name, Status: "up", Kind: interfaces.KindEthernet, Candidate: true, IPv4Metric: 25, Addresses: []interfaces.Address{{IP: ip, PrefixLength: prefix}}, Gateways: []string{"192.0.2.1"}, DNSServers: []string{"1.1.1.1"}}
}

func assertDiagnostic(t *testing.T, diagnostics []Diagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			return
		}
	}
	t.Fatalf("diagnostic %q not found in %#v", code, diagnostics)
}

func assertNoDiagnostic(t *testing.T, diagnostics []Diagnostic, code string) {
	t.Helper()
	for _, diagnostic := range diagnostics {
		if diagnostic.Code == code {
			t.Fatalf("unexpected diagnostic %q in %#v", code, diagnostics)
		}
	}
}

func waitForSequence(t *testing.T, manager *Manager) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for manager.Snapshot().Sequence == 0 {
		if time.Now().After(deadline) {
			t.Fatal("manager did not produce its initial snapshot")
		}
		time.Sleep(time.Millisecond)
	}
}

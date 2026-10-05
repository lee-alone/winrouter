package tunprefix

import (
	"errors"
	"net/netip"
	"testing"

	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
)

func TestAllocateReusesAvailablePreferredPrefix(t *testing.T) {
	allocation, err := Allocate([]string{"172.19.0.0/30", "172.19.0.4/30"}, "172.19.0.4/30", nil, []routes.Route{{Prefix: "0.0.0.0/0"}})
	if err != nil || allocation.Prefix != "172.19.0.4/30" || !allocation.Reused {
		t.Fatalf("Allocate() = %#v, %v", allocation, err)
	}
}

func TestAllocateFallsBackDeterministicallyAndReportsConflict(t *testing.T) {
	adapters := []interfaces.Adapter{{Index: 7, FriendlyName: "Ethernet", Addresses: []interfaces.Address{{IP: "172.19.0.2", PrefixLength: 30}}}}
	allocation, err := Allocate([]string{"172.19.0.0/30", "172.19.0.4/30"}, "172.19.0.0/30", adapters, nil)
	if err != nil {
		t.Fatalf("Allocate() error: %v", err)
	}
	if allocation.Prefix != "172.19.0.4/30" || allocation.Reused || len(allocation.Conflicts) != 1 || allocation.Conflicts[0].Source != ConflictInterface {
		t.Fatalf("Allocate() = %#v", allocation)
	}
}

func TestAllocateChecksSpecificRoutesAndExhaustion(t *testing.T) {
	routeTable := []routes.Route{{Prefix: "172.19.0.0/29", InterfaceIndex: 9}}
	_, err := Allocate([]string{"172.19.0.0/30", "172.19.0.4/30"}, "", nil, routeTable)
	if !errors.Is(err, ErrPoolExhausted) {
		t.Fatalf("Allocate() error = %v, want ErrPoolExhausted", err)
	}
	var exhausted *PoolExhaustedError
	if !errors.As(err, &exhausted) || len(exhausted.Conflicts) != 2 {
		t.Fatalf("Allocate() error = %#v", err)
	}
}

func TestAllocateRejectsInvalidPool(t *testing.T) {
	for _, pool := range [][]string{{}, {"bad"}, {"8.8.8.0/24"}, {"10.0.0.0/7"}, {"10.0.0.0/30", "10.0.0.0/30"}} {
		if _, err := Allocate(pool, "", nil, nil); err == nil {
			t.Fatalf("Allocate(%v) succeeded, want error", pool)
		}
	}
}

func TestAllocateIPv6PrefixWithConflict(t *testing.T) {
	adapters := []interfaces.Adapter{
		{
			Index: 5, IPv6Index: 15, FriendlyName: "Ethernet 2",
			Addresses: []interfaces.Address{{IP: "fdfe:dcba:9876::1", PrefixLength: 126}},
		},
	}
	pool := []string{"fdfe:dcba:9876::/126", "fdfe:dcba:9876::4/126"}
	allocation, err := Allocate(pool, "fdfe:dcba:9876::/126", adapters, nil)
	if err != nil {
		t.Fatalf("Allocate() error: %v", err)
	}
	if allocation.Prefix != "fdfe:dcba:9876::4/126" || allocation.Reused {
		t.Fatalf("allocation = %#v", allocation)
	}
	if len(allocation.Conflicts) != 1 || allocation.Conflicts[0].InterfaceIndex != 15 || allocation.Conflicts[0].InterfaceName != "Ethernet 2" {
		t.Fatalf("conflicts = %#v", allocation.Conflicts)
	}
}

func TestConflictsForPrefixIgnoresSummaryRoutes(t *testing.T) {
	routeTable := []routes.Route{
		{Prefix: "0.0.0.0/0", InterfaceIndex: 1},
		{Prefix: "::/0", InterfaceIndex: 1},
		{Prefix: "fc00::/7", InterfaceIndex: 1},
		{Prefix: "10.0.0.0/8", InterfaceIndex: 1},
		{Prefix: "172.16.0.0/12", InterfaceIndex: 1},
		{Prefix: "192.168.0.0/16", InterfaceIndex: 1},
	}
	v6Candidate, err := netip.ParsePrefix("fdfe:dcba:9876::/126")
	if err != nil {
		t.Fatal(err)
	}
	conflicts := ConflictsForPrefix(v6Candidate, nil, routeTable)
	if len(conflicts) != 0 {
		t.Fatalf("ConflictsForPrefix(v6) = %#v, want 0 conflicts", conflicts)
	}

	v4Candidate, err := netip.ParsePrefix("172.19.0.0/30")
	if err != nil {
		t.Fatal(err)
	}
	conflicts = ConflictsForPrefix(v4Candidate, nil, routeTable)
	if len(conflicts) != 0 {
		t.Fatalf("ConflictsForPrefix(v4) = %#v, want 0 conflicts", conflicts)
	}
}

func TestConflictsForPrefixIgnoresWinRouterTUN(t *testing.T) {
	tunAdapter := interfaces.Adapter{
		Index: 12, IPv6Index: 13, LUID: 9999, FriendlyName: "WinRouter-TUN", Kind: interfaces.KindTunnel,
		Addresses: []interfaces.Address{
			{IP: "172.19.0.1", PrefixLength: 30},
			{IP: "fdfe:dcba:9876::1", PrefixLength: 126},
		},
	}
	routeTable := []routes.Route{
		{Prefix: "172.19.0.0/30", InterfaceIndex: 12, InterfaceLUID: 9999},
		{Prefix: "fdfe:dcba:9876::/126", InterfaceIndex: 13, InterfaceLUID: 9999},
	}
	v4Candidate := netip.MustParsePrefix("172.19.0.0/30")
	v6Candidate := netip.MustParsePrefix("fdfe:dcba:9876::/126")

	if conflicts := ConflictsForPrefix(v4Candidate, []interfaces.Adapter{tunAdapter}, routeTable); len(conflicts) != 0 {
		t.Fatalf("ConflictsForPrefix(v4) on active WinRouter-TUN = %#v, want 0 conflicts", conflicts)
	}
	if conflicts := ConflictsForPrefix(v6Candidate, []interfaces.Adapter{tunAdapter}, routeTable); len(conflicts) != 0 {
		t.Fatalf("ConflictsForPrefix(v6) on active WinRouter-TUN = %#v, want 0 conflicts", conflicts)
	}

	allocation, err := Allocate([]string{"172.19.0.0/30", "172.19.0.4/30"}, "172.19.0.0/30", []interfaces.Adapter{tunAdapter}, routeTable)
	if err != nil {
		t.Fatalf("Allocate() error: %v", err)
	}
	if allocation.Prefix != "172.19.0.0/30" || !allocation.Reused {
		t.Fatalf("Allocate() = %#v, want 172.19.0.0/30 reused without jumping prefix", allocation)
	}
}

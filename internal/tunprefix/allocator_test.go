package tunprefix

import (
	"errors"
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

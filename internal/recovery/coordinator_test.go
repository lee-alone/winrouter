package recovery

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	"winrouter/internal/interfacemanager"
	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
	"winrouter/internal/tunprefix"
)

func TestCoordinatorStopsAndRestoresAfterStableNetwork(t *testing.T) {
	var stops, applies atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error { stops.Add(1); return nil }, func(context.Context, interfacemanager.Snapshot) error { applies.Add(1); return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	coordinator.SetDesired(true)
	coordinator.NetworkChanged(snapshot(false, 1))
	waitStatus(t, coordinator, StateWaiting)
	coordinator.NetworkChanged(snapshot(true, 2))
	waitStatus(t, coordinator, StateMonitoring)
	if stops.Load() != 1 || applies.Load() != 1 {
		t.Fatalf("operations stop=%d apply=%d", stops.Load(), applies.Load())
	}
}

func TestCoordinatorBoundsRecoveryRetries(t *testing.T) {
	var applies atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error { return nil }, func(context.Context, interfacemanager.Snapshot) error {
		applies.Add(1)
		return errors.New("injected apply failure")
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	coordinator.SetDesired(true)
	coordinator.NetworkChanged(snapshot(false, 1))
	waitStatus(t, coordinator, StateWaiting)
	coordinator.NetworkChanged(snapshot(true, 2))
	waitFor(t, func() bool { return coordinator.Status().State == StateFailed && coordinator.Status().Attempts == 3 })
	if applies.Load() != 3 {
		t.Fatalf("apply attempts = %d", applies.Load())
	}
}

func TestCoordinatorDoesNotRestoreAfterUserStop(t *testing.T) {
	var applies atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error { return nil }, func(context.Context, interfacemanager.Snapshot) error { applies.Add(1); return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	coordinator.SetDesired(true)
	coordinator.NetworkChanged(snapshot(false, 1))
	waitStatus(t, coordinator, StateWaiting)
	coordinator.SetDesired(false)
	coordinator.NetworkChanged(snapshot(true, 2))
	waitStatus(t, coordinator, StateIdle)
	time.Sleep(30 * time.Millisecond)
	if applies.Load() != 0 {
		t.Fatalf("apply attempts = %d", applies.Load())
	}
}

func TestCoordinatorRebuildsAfterSelectedAddressChanges(t *testing.T) {
	var stops, applies atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error { stops.Add(1); return nil }, func(context.Context, interfacemanager.Snapshot) error { applies.Add(1); return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	initial := snapshot(true, 1)
	coordinator.NetworkChanged(initial)
	coordinator.SetDesired(true)
	changed := snapshot(true, 2)
	changed.InterfaceA.Match.Adapter.Addresses[0].IP = "192.0.2.20"
	coordinator.NetworkChanged(changed)
	waitFor(t, func() bool { return coordinator.Status().State == StateMonitoring && applies.Load() == 1 })
	if stops.Load() != 1 {
		t.Fatalf("stop operations = %d", stops.Load())
	}
}

func TestCoordinatorRebuildsAfterSelectedGatewayChanges(t *testing.T) {
	var stops, applies atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error { stops.Add(1); return nil }, func(context.Context, interfacemanager.Snapshot) error { applies.Add(1); return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	initial := snapshot(true, 1)
	coordinator.NetworkChanged(initial)
	coordinator.SetDesired(true)
	changed := snapshot(true, 2)
	changed.InterfaceB.Match.Adapter.Gateways[0] = "198.51.100.254"
	coordinator.NetworkChanged(changed)
	waitFor(t, func() bool { return coordinator.Status().State == StateMonitoring && applies.Load() == 1 })
	if stops.Load() != 1 {
		t.Fatalf("stop operations = %d", stops.Load())
	}
}

func TestCoordinatorStopsWhenSelectedDefaultRouteDisappearsAndRestoresWhenItReturns(t *testing.T) {
	var stops, applies atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error { stops.Add(1); return nil }, func(context.Context, interfacemanager.Snapshot) error { applies.Add(1); return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	initial := snapshot(true, 1)
	coordinator.NetworkChanged(initial)
	coordinator.SetDesired(true)

	missing := snapshot(true, 2)
	missing.Routes = missing.Routes[1:]
	missing.Diagnostics = append(missing.Diagnostics, interfacemanager.Diagnostic{Code: "missing-default-route-a", Severity: "error"})
	coordinator.NetworkChanged(missing)
	waitStatus(t, coordinator, StateWaiting)
	if stops.Load() != 1 || applies.Load() != 0 {
		t.Fatalf("operations while route is missing: stop=%d apply=%d", stops.Load(), applies.Load())
	}

	restored := snapshot(true, 3)
	coordinator.NetworkChanged(restored)
	waitFor(t, func() bool { return coordinator.Status().State == StateMonitoring && applies.Load() == 1 })
}

func TestCoordinatorDoesNotApplyUntilSafeStopSucceeds(t *testing.T) {
	var stops, applies atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error {
		if stops.Add(1) == 1 {
			return errors.New("injected stop failure")
		}
		return nil
	}, func(context.Context, interfacemanager.Snapshot) error {
		applies.Add(1)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	initial := snapshot(true, 1)
	coordinator.NetworkChanged(initial)
	coordinator.SetDesired(true)
	changed := snapshot(true, 2)
	changed.InterfaceA.Match.Adapter.Addresses[0].IP = "192.0.2.20"
	coordinator.NetworkChanged(changed)
	waitStatus(t, coordinator, StateFailed)
	if applies.Load() != 0 {
		t.Fatalf("apply ran after failed stop: %d", applies.Load())
	}
	coordinator.NetworkChanged(changed)
	waitFor(t, func() bool { return coordinator.Status().State == StateMonitoring && applies.Load() == 1 })
	if stops.Load() != 2 {
		t.Fatalf("stop attempts = %d", stops.Load())
	}
}

func TestCoordinatorIgnoresTUNAndMetricOnlyChanges(t *testing.T) {
	var stops atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error { stops.Add(1); return nil }, func(context.Context, interfacemanager.Snapshot) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	initial := snapshot(true, 1)
	coordinator.NetworkChanged(initial)
	coordinator.SetDesired(true)
	changed := snapshot(true, 2)
	changed.InterfaceA.Match.Adapter.IPv4Metric = 999
	changed.Routes = append(changed.Routes, routes.Route{Prefix: "0.0.0.0/0", InterfaceIndex: 42})
	coordinator.NetworkChanged(changed)
	time.Sleep(30 * time.Millisecond)
	if stops.Load() != 0 || coordinator.Status().State != StateMonitoring {
		t.Fatalf("unexpected recovery: stops=%d status=%#v", stops.Load(), coordinator.Status())
	}
}

func TestCoordinatorIgnoresUnselectedVirtualInterfaceChanges(t *testing.T) {
	var stops atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error { stops.Add(1); return nil }, func(context.Context, interfacemanager.Snapshot) error { return nil })
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	initial := snapshot(true, 1)
	coordinator.NetworkChanged(initial)
	coordinator.SetDesired(true)
	changed := snapshot(true, 2)
	changed.Adapters = append(changed.Adapters, interfaces.Adapter{GUID: "{VIRTUAL}", FriendlyName: "vEthernet (WSL)", Kind: interfaces.KindWSL, Status: "up", Candidate: false})
	coordinator.NetworkChanged(changed)
	time.Sleep(30 * time.Millisecond)
	if stops.Load() != 0 || coordinator.Status().State != StateMonitoring {
		t.Fatalf("unexpected recovery after virtual interface change: stops=%d status=%#v", stops.Load(), coordinator.Status())
	}
}

func TestCoordinatorRecoversWhenRuntimeDisappearsWithoutNetworkChange(t *testing.T) {
	var stops, applies atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error {
		stops.Add(1)
		return nil
	}, func(context.Context, interfacemanager.Snapshot) error {
		applies.Add(1)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	current := snapshot(true, 1)
	coordinator.NetworkChanged(current)
	coordinator.SetDesired(true)
	coordinator.RuntimeFailed(current, errors.New("broken pipe"))
	waitFor(t, func() bool { return coordinator.Status().State == StateMonitoring && applies.Load() == 1 })
	if stops.Load() != 0 {
		t.Fatalf("runtime loss attempted stop: %d", stops.Load())
	}
}

func TestCoordinatorIgnoresRuntimeFailureAfterManualStop(t *testing.T) {
	var applies atomic.Int32
	coordinator := newTestCoordinator(t, func(context.Context) error { return nil }, func(context.Context, interfacemanager.Snapshot) error {
		applies.Add(1)
		return nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go coordinator.Run(ctx)
	current := snapshot(true, 1)
	coordinator.NetworkChanged(current)
	coordinator.SetDesired(true)
	coordinator.SetDesired(false)
	coordinator.RuntimeFailed(current, errors.New("late failure"))
	time.Sleep(30 * time.Millisecond)
	if applies.Load() != 0 || coordinator.Status().State != StateIdle {
		t.Fatalf("manual stop was not respected: applies=%d status=%#v", applies.Load(), coordinator.Status())
	}
}

func newTestCoordinator(t *testing.T, stop func(context.Context) error, apply func(context.Context, interfacemanager.Snapshot) error) *Coordinator {
	t.Helper()
	coordinator, err := New(Options{Stop: stop, Apply: apply, SettleDelay: 5 * time.Millisecond, RetryDelay: 5 * time.Millisecond, MaxAttempts: 3, OperationTTL: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return coordinator
}

func snapshot(usable bool, sequence uint64) interfacemanager.Snapshot {
	first := interfaces.Adapter{GUID: "{A}", Index: 1, FriendlyName: "Ethernet", Status: "up", Addresses: []interfaces.Address{{IP: "192.0.2.10", PrefixLength: 24}}, Gateways: []string{"192.0.2.1"}}
	second := interfaces.Adapter{GUID: "{B}", Index: 2, FriendlyName: "Wi-Fi", Status: "up", Addresses: []interfaces.Address{{IP: "198.51.100.10", PrefixLength: 24}}, Gateways: []string{"198.51.100.1"}}
	value := interfacemanager.Snapshot{Sequence: sequence,
		InterfaceA: interfacemanager.ResolvedSelection{Status: "resolved", Match: &interfaces.Match{Adapter: first}},
		InterfaceB: interfacemanager.ResolvedSelection{Status: "resolved", Match: &interfaces.Match{Adapter: second}},
		Routes:     []routes.Route{{Prefix: "0.0.0.0/0", InterfaceIndex: 1}, {Prefix: "0.0.0.0/0", InterfaceIndex: 2}},
	}
	if usable {
		value.TUN = &structAllocation
	}
	return value
}

var structAllocation tunprefix.Allocation

func waitStatus(t *testing.T, coordinator *Coordinator, state State) {
	t.Helper()
	waitFor(t, func() bool { return coordinator.Status().State == state })
}
func waitFor(t *testing.T, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition() {
		if time.Now().After(deadline) {
			t.Fatal("timed out waiting for condition")
		}
		time.Sleep(time.Millisecond)
	}
}

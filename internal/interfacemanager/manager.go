package interfacemanager

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"sync"
	"time"

	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
	"winrouter/internal/tunprefix"
)

type EnumerateInterfaces func() ([]interfaces.Adapter, error)
type EnumerateRoutes func() ([]routes.Route, error)

type Options struct {
	StatePath           string
	TUNPool             []string
	PollInterval        time.Duration
	EnumerateInterfaces EnumerateInterfaces
	EnumerateRoutes     EnumerateRoutes
}

type Manager struct {
	mu          sync.RWMutex
	options     Options
	state       State
	snapshot    Snapshot
	sequence    uint64
	subscribers map[chan Event]struct{}
}

func New(options Options) (*Manager, error) {
	if options.EnumerateInterfaces == nil {
		options.EnumerateInterfaces = interfaces.Enumerate
	}
	if options.EnumerateRoutes == nil {
		options.EnumerateRoutes = routes.Enumerate
	}
	if len(options.TUNPool) == 0 {
		options.TUNPool = append([]string(nil), tunprefix.DefaultPool...)
	}
	if options.PollInterval <= 0 {
		options.PollInterval = time.Second
	}
	state := State{SchemaVersion: StateSchemaVersion, IPv6Policy: IPv6PolicyBlock}
	var err error
	if options.StatePath != "" {
		state, err = LoadState(options.StatePath)
		if err != nil {
			return nil, err
		}
	}
	return &Manager{options: options, state: state, subscribers: make(map[chan Event]struct{})}, nil
}

func (m *Manager) SetIPv6Policy(policy string) (Snapshot, error) {
	if policy != IPv6PolicyBlock && policy != IPv6PolicySplit {
		return Snapshot{}, fmt.Errorf("unsupported IPv6 policy %q", policy)
	}
	m.mu.Lock()
	previous := m.state
	m.state.IPv6Policy = policy
	state := m.state
	m.mu.Unlock()
	if m.options.StatePath != "" {
		if err := SaveState(m.options.StatePath, state); err != nil {
			m.mu.Lock()
			m.state = previous
			m.mu.Unlock()
			return Snapshot{}, err
		}
	}
	return m.Refresh()
}

func (m *Manager) Refresh() (Snapshot, error) {
	adapters, err := m.options.EnumerateInterfaces()
	if err != nil {
		return Snapshot{}, fmt.Errorf("enumerate interfaces: %w", err)
	}
	routeTable, err := m.options.EnumerateRoutes()
	if err != nil {
		return Snapshot{}, fmt.Errorf("enumerate routes: %w", err)
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.sequence++
	snapshot := buildSnapshot(m.sequence, m.state, adapters, routeTable, m.options.TUNPool)
	if snapshot.TUN != nil && snapshot.TUN.Prefix != m.state.TUNPrefix {
		updated := m.state
		updated.TUNPrefix = snapshot.TUN.Prefix
		if m.options.StatePath != "" {
			if err := SaveState(m.options.StatePath, updated); err != nil {
				return Snapshot{}, err
			}
		}
		m.state = updated
	}
	m.snapshot = snapshot
	return cloneSnapshot(snapshot), nil
}

func (m *Manager) Select(interfaceA, interfaceB interfaces.Adapter) (Snapshot, error) {
	if interfaceA.GUID == "" || interfaceB.GUID == "" {
		return Snapshot{}, ErrSelectionInvalid
	}
	if equalGUID(interfaceA.GUID, interfaceB.GUID) {
		return Snapshot{}, ErrSameAdapter
	}
	if !interfaceA.Candidate || !interfaceB.Candidate {
		return Snapshot{}, ErrSelectionInvalid
	}
	m.mu.Lock()
	previous := m.state
	m.state.InterfaceA = interfaces.IdentityFromAdapter(interfaceA)
	m.state.InterfaceB = interfaces.IdentityFromAdapter(interfaceB)
	state := m.state
	m.mu.Unlock()
	if m.options.StatePath != "" {
		if err := SaveState(m.options.StatePath, state); err != nil {
			m.mu.Lock()
			m.state = previous
			m.mu.Unlock()
			return Snapshot{}, err
		}
	}
	return m.Refresh()
}

func (m *Manager) Snapshot() Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return cloneSnapshot(m.snapshot)
}

func (m *Manager) IPv6Policy() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.state.IPv6Policy
}

func (m *Manager) Subscribe(buffer int) (<-chan Event, func()) {
	if buffer < 1 {
		buffer = 1
	}
	channel := make(chan Event, buffer)
	m.mu.Lock()
	m.subscribers[channel] = struct{}{}
	m.mu.Unlock()
	var once sync.Once
	return channel, func() { once.Do(func() { m.mu.Lock(); delete(m.subscribers, channel); close(channel); m.mu.Unlock() }) }
}

func (m *Manager) Run(ctx context.Context) error {
	initial, err := m.Refresh()
	if err != nil {
		return err
	}
	lastFingerprint := fingerprint(initial)
	ticker := time.NewTicker(m.options.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			snapshot, refreshErr := m.Refresh()
			if refreshErr != nil {
				m.publish(Event{Reason: "refresh-error", Snapshot: m.Snapshot()})
				continue
			}
			current := fingerprint(snapshot)
			if current != lastFingerprint {
				lastFingerprint = current
				m.publish(Event{Reason: "network-change", Snapshot: snapshot})
			}
		}
	}
}

func (m *Manager) publish(event Event) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	for subscriber := range m.subscribers {
		select {
		case subscriber <- event:
		default:
		}
	}
}

func buildSnapshot(sequence uint64, state State, adapters []interfaces.Adapter, routeTable []routes.Route, pool []string) Snapshot {
	topology := interfaces.BuildTopology(adapters)
	snapshot := Snapshot{Sequence: sequence, Adapters: adapters, Routes: routeTable, Candidates: BuildCandidates(adapters), Topology: topology, Diagnostics: make([]Diagnostic, 0)}
	snapshot.InterfaceA = resolveSelection("A", state.InterfaceA, adapters)
	snapshot.InterfaceB = resolveSelection("B", state.InterfaceB, adapters)
	if snapshot.InterfaceA.Match != nil && snapshot.InterfaceB.Match != nil && equalGUID(snapshot.InterfaceA.Match.Adapter.GUID, snapshot.InterfaceB.Match.Adapter.GUID) {
		snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "same-interface", Severity: "error", Message: ErrSameAdapter.Error()})
	}
	for _, selection := range []ResolvedSelection{snapshot.InterfaceA, snapshot.InterfaceB} {
		if selection.Status == "resolved" && selection.Match != nil && !hasIPv4DefaultRoute(routeTable, selection.Match.Adapter) {
			snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "missing-default-route-" + strings.ToLower(selection.Role), Severity: "error", Message: fmt.Sprintf("selected interface %s (%s) has no IPv4 default route", selection.Role, selection.Match.Adapter.FriendlyName)})
		}
	}
	for _, overlap := range topology.Overlaps {
		if overlap.RequiresSelection && overlapBlocksPolicy(overlap, state.IPv6Policy) {
			snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "prefix-overlap", Severity: "error", Message: fmt.Sprintf("%s on %s overlaps %s on %s", overlap.First.Prefix, overlap.First.AdapterName, overlap.Second.Prefix, overlap.Second.AdapterName)})
		}
	}
	if state.IPv6Policy == IPv6PolicySplit {
		for _, selection := range []ResolvedSelection{snapshot.InterfaceA, snapshot.InterfaceB} {
			if selection.Status != "resolved" || selection.Match == nil {
				continue
			}
			if !hasUsableIPv6(selection.Match.Adapter) {
				snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "missing-ipv6-address-" + strings.ToLower(selection.Role), Severity: "error", Message: fmt.Sprintf("selected interface %s (%s) has no usable IPv6 address", selection.Role, selection.Match.Adapter.FriendlyName)})
			}
			if !hasIPv6DefaultRoute(routeTable, selection.Match.Adapter) {
				snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "missing-ipv6-default-route-" + strings.ToLower(selection.Role), Severity: "error", Message: fmt.Sprintf("selected interface %s (%s) has no IPv6 default route", selection.Role, selection.Match.Adapter.FriendlyName)})
			}
		}
	}
	allocation, err := tunprefix.Allocate(pool, state.TUNPrefix, adapters, routeTable)
	if err == nil {
		snapshot.TUN = &allocation
	} else {
		severity := "error"
		code := "tun-prefix"
		if errors.Is(err, tunprefix.ErrPoolExhausted) {
			code = "tun-pool-exhausted"
		}
		snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: code, Severity: severity, Message: err.Error()})
	}
	return snapshot
}

func overlapBlocksPolicy(overlap interfaces.PrefixOverlap, policy string) bool {
	if policy == IPv6PolicySplit {
		return true
	}
	first, firstErr := netip.ParsePrefix(overlap.First.Prefix)
	second, secondErr := netip.ParsePrefix(overlap.Second.Prefix)
	return firstErr == nil && secondErr == nil && first.Addr().Is4() && second.Addr().Is4()
}

func hasIPv4DefaultRoute(routeTable []routes.Route, adapter interfaces.Adapter) bool {
	for _, route := range routeTable {
		if route.Prefix == "0.0.0.0/0" && route.InterfaceIndex == adapter.Index {
			return true
		}
	}
	return false
}

func hasIPv6DefaultRoute(routeTable []routes.Route, adapter interfaces.Adapter) bool {
	for _, route := range routeTable {
		if route.Prefix == "::/0" && route.InterfaceIndex == adapter.Index {
			return true
		}
	}
	return false
}

func hasUsableIPv6(adapter interfaces.Adapter) bool {
	for _, address := range adapter.Addresses {
		parsed, err := netip.ParseAddr(address.IP)
		if err == nil && parsed.Is6() && !parsed.IsUnspecified() && !parsed.IsLoopback() && !parsed.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

func resolveSelection(role string, identity interfaces.Identity, adapters []interfaces.Adapter) ResolvedSelection {
	result := ResolvedSelection{Role: role, Saved: identity, Status: "unselected"}
	if identity.GUID == "" {
		return result
	}
	match, err := interfaces.Resolve(identity, adapters)
	if err != nil {
		result.Status = "selection-required"
		result.Error = err.Error()
		return result
	}
	result.Match = &match
	if match.Adapter.Status != "up" {
		result.Status = "unavailable"
		result.Error = "selected interface is not up"
	} else {
		result.Status = "resolved"
	}
	return result
}

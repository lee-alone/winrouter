package interfacemanager

import (
	"context"
	"fmt"
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
			_ = SaveState(m.options.StatePath, updated)
		}
		m.state = updated
	}
	m.snapshot = snapshot
	return cloneSnapshot(snapshot), nil
}

func (m *Manager) Mode() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.state.Mode == "" {
		if m.state.InterfaceB.GUID != "" {
			return ModeDual
		}
		return ModeSingle
	}
	return m.state.Mode
}

func (m *Manager) SetMode(mode string) (Snapshot, error) {
	if mode != ModeSingle && mode != ModeDual {
		return Snapshot{}, fmt.Errorf("unsupported mode %q", mode)
	}
	m.mu.Lock()
	previous := m.state
	m.state.Mode = mode
	if mode == ModeSingle {
		m.state.InterfaceB = interfaces.Identity{}
	}
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

func (m *Manager) SelectSingle(interfaceA interfaces.Adapter) (Snapshot, error) {
	if interfaceA.GUID == "" || !interfaceA.Candidate {
		return Snapshot{}, ErrSelectionInvalid
	}
	m.mu.Lock()
	previous := m.state
	m.state.Mode = ModeSingle
	m.state.InterfaceA = interfaces.IdentityFromAdapter(interfaceA)
	m.state.InterfaceB = interfaces.Identity{}
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
	m.state.Mode = ModeDual
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

func (m *Manager) ClearSelection() (Snapshot, error) {
	m.mu.Lock()
	previous := m.state
	m.state.InterfaceA = interfaces.Identity{}
	m.state.InterfaceB = interfaces.Identity{}
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


package recovery

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"sort"
	"sync"
	"time"

	"winrouter/internal/interfacemanager"
)

type State string

const (
	StateIdle       State = "idle"
	StateMonitoring State = "monitoring"
	StateStopping   State = "stopping"
	StateWaiting    State = "waiting-for-network"
	StateRecovering State = "recovering"
	StateFailed     State = "failed"
)

type Status struct {
	State       State  `json:"state"`
	Desired     bool   `json:"desired_running"`
	Attempts    int    `json:"attempts"`
	LastError   string `json:"last_error,omitempty"`
	LastChange  string `json:"last_change,omitempty"`
	SnapshotSeq uint64 `json:"snapshot_sequence,omitempty"`
}

type Options struct {
	Stop         func(context.Context) error
	Apply        func(context.Context, interfacemanager.Snapshot) error
	OnChange     func(Status)
	SettleDelay  time.Duration
	RetryDelay   time.Duration
	MaxAttempts  int
	OperationTTL time.Duration
}

type command struct {
	desired      *bool
	snapshot     *interfacemanager.Snapshot
	runtimeError error
	done         chan struct{}
}

type Coordinator struct {
	mu       sync.RWMutex
	options  Options
	status   Status
	commands chan command
	done     chan struct{}
}

func New(options Options) (*Coordinator, error) {
	if options.Stop == nil || options.Apply == nil {
		return nil, errors.New("recovery stop and apply operations are required")
	}
	if options.SettleDelay <= 0 {
		options.SettleDelay = 3 * time.Second
	}
	if options.RetryDelay <= 0 {
		options.RetryDelay = 2 * time.Second
	}
	if options.MaxAttempts <= 0 {
		options.MaxAttempts = 3
	}
	if options.OperationTTL <= 0 {
		options.OperationTTL = 20 * time.Second
	}
	return &Coordinator{options: options, status: Status{State: StateIdle}, commands: make(chan command, 8), done: make(chan struct{})}, nil
}

func (c *Coordinator) Run(ctx context.Context) {
	defer close(c.done)
	var latest interfacemanager.Snapshot
	var suspended bool
	var baseline [sha256.Size]byte
	var hasBaseline bool
	var timer *time.Timer
	var timerC <-chan time.Time
	stopTimer := func() {
		if timer != nil && !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer = nil
		timerC = nil
	}
	schedule := func(delay time.Duration) {
		stopTimer()
		timer = time.NewTimer(delay)
		timerC = timer.C
	}
	defer stopTimer()

	for {
		select {
		case <-ctx.Done():
			return
		case message := <-c.commands:
			if message.desired != nil {
				if !*message.desired {
					stopTimer()
					suspended = false
					hasBaseline = false
					c.update(Status{State: StateIdle})
				} else {
					if Usable(latest) {
						baseline = physicalFingerprint(latest)
						hasBaseline = true
					}
					c.update(Status{State: StateMonitoring, Desired: true, SnapshotSeq: latest.Sequence})
				}
				if message.done != nil {
					close(message.done)
				}
				continue
			}
			if message.runtimeError != nil {
				latest = *message.snapshot
				current := c.Status()
				if current.Desired && !suspended {
					suspended = true
					c.update(Status{State: StateWaiting, Desired: true, LastError: fmt.Sprintf("core unavailable: %v", message.runtimeError), SnapshotSeq: latest.Sequence})
					if Usable(latest) {
						schedule(c.options.SettleDelay)
					}
				}
				continue
			}
			if message.snapshot == nil {
				continue
			}
			latest = *message.snapshot
			current := c.Status()
			if !current.Desired {
				continue
			}
			if !Usable(latest) {
				stopTimer()
				if !suspended {
					c.update(Status{State: StateStopping, Desired: true, SnapshotSeq: latest.Sequence})
					err := c.runOperation(ctx, c.options.Stop)
					if err != nil {
						c.update(Status{State: StateFailed, Desired: true, LastError: fmt.Sprintf("safe stop: %v", err), SnapshotSeq: latest.Sequence})
						continue
					}
					suspended = true
				}
				c.update(Status{State: StateWaiting, Desired: true, SnapshotSeq: latest.Sequence})
				continue
			}
			if suspended {
				c.update(Status{State: StateWaiting, Desired: true, SnapshotSeq: latest.Sequence})
				schedule(c.options.SettleDelay)
				continue
			}
			currentFingerprint := physicalFingerprint(latest)
			if !hasBaseline {
				baseline = currentFingerprint
				hasBaseline = true
				continue
			}
			if currentFingerprint != baseline {
				stopTimer()
				c.update(Status{State: StateStopping, Desired: true, SnapshotSeq: latest.Sequence})
				err := c.runOperation(ctx, c.options.Stop)
				if err != nil {
					c.update(Status{State: StateFailed, Desired: true, LastError: fmt.Sprintf("safe stop after network change: %v", err), SnapshotSeq: latest.Sequence})
					continue
				}
				suspended = true
				c.update(Status{State: StateWaiting, Desired: true, SnapshotSeq: latest.Sequence})
				schedule(c.options.SettleDelay)
			}
		case <-timerC:
			timer = nil
			timerC = nil
			current := c.Status()
			if !current.Desired || !suspended || !Usable(latest) {
				continue
			}
			attempt := current.Attempts + 1
			c.update(Status{State: StateRecovering, Desired: true, Attempts: attempt, SnapshotSeq: latest.Sequence})
			err := c.runOperation(ctx, func(operationCtx context.Context) error { return c.options.Apply(operationCtx, latest) })
			if err == nil {
				suspended = false
				baseline = physicalFingerprint(latest)
				hasBaseline = true
				c.update(Status{State: StateMonitoring, Desired: true, SnapshotSeq: latest.Sequence})
				continue
			}
			failed := Status{State: StateFailed, Desired: true, Attempts: attempt, LastError: fmt.Sprintf("restore: %v", err), SnapshotSeq: latest.Sequence}
			c.update(failed)
			if attempt < c.options.MaxAttempts {
				schedule(c.options.RetryDelay * time.Duration(attempt))
			}
		}
	}
}

func (c *Coordinator) SetDesired(running bool) {
	done := make(chan struct{})
	if !c.enqueue(command{desired: &running, done: done}) {
		return
	}
	select {
	case <-done:
	case <-c.done:
	}
}

func (c *Coordinator) NetworkChanged(snapshot interfacemanager.Snapshot) {
	clone := snapshot
	c.enqueue(command{snapshot: &clone})
}

// RuntimeFailed requests a rebuild when the runtime disappears without a
// corresponding physical topology change, such as across system sleep.
func (c *Coordinator) RuntimeFailed(snapshot interfacemanager.Snapshot, err error) {
	if err == nil {
		err = errors.New("core runtime became unavailable")
	}
	clone := snapshot
	c.enqueue(command{snapshot: &clone, runtimeError: err})
}

func (c *Coordinator) Status() Status {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.status
}

func (c *Coordinator) enqueue(message command) bool {
	select {
	case c.commands <- message:
		return true
	case <-c.done:
		return false
	}
}

func (c *Coordinator) runOperation(parent context.Context, operation func(context.Context) error) error {
	ctx, cancel := context.WithTimeout(parent, c.options.OperationTTL)
	defer cancel()
	return operation(ctx)
}

func (c *Coordinator) update(status Status) {
	status.LastChange = time.Now().UTC().Format(time.RFC3339Nano)
	c.mu.Lock()
	c.status = status
	c.mu.Unlock()
	if c.options.OnChange != nil {
		c.options.OnChange(status)
	}
}

func Usable(snapshot interfacemanager.Snapshot) bool {
	if snapshot.InterfaceA.Status != "resolved" || snapshot.InterfaceB.Status != "resolved" || snapshot.TUN == nil {
		return false
	}
	for _, diagnostic := range snapshot.Diagnostics {
		if diagnostic.Severity == "error" {
			return false
		}
	}
	return true
}

func physicalFingerprint(snapshot interfacemanager.Snapshot) [sha256.Size]byte {
	type adapterState struct {
		GUID       string
		Name       string
		Addresses  []string
		Gateways   []string
		HasDefault bool
	}
	stateFor := func(selection interfacemanager.ResolvedSelection) adapterState {
		if selection.Match == nil {
			return adapterState{}
		}
		adapter := selection.Match.Adapter
		state := adapterState{GUID: adapter.GUID, Name: adapter.FriendlyName}
		for _, address := range adapter.Addresses {
			if parsed, err := netip.ParseAddr(address.IP); err == nil && parsed.Is4() {
				state.Addresses = append(state.Addresses, netip.PrefixFrom(parsed, int(address.PrefixLength)).String())
			}
		}
		state.Gateways = append(state.Gateways, adapter.Gateways...)
		for _, route := range snapshot.Routes {
			if route.Prefix == "0.0.0.0/0" && (route.InterfaceIndex == adapter.Index || (route.InterfaceLUID != 0 && adapter.LUID != 0 && route.InterfaceLUID == adapter.LUID)) {
				state.HasDefault = true
				break
			}
		}
		sort.Strings(state.Addresses)
		sort.Strings(state.Gateways)
		return state
	}
	value := [2]adapterState{stateFor(snapshot.InterfaceA), stateFor(snapshot.InterfaceB)}
	data, _ := json.Marshal(value)
	return sha256.Sum256(data)
}

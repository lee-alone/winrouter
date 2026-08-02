package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Process interface {
	PID() int
	Done() <-chan struct{}
	ExitError() error
	Stop(context.Context) error
	Kill() error
}

type LaunchFunc func(context.Context, string) (Process, error)
type ValidateFunc func(context.Context, []byte) error
type HealthFunc func(context.Context, Process, []byte) error

type RestartPolicy struct {
	MaxAttempts    int
	InitialBackoff time.Duration
	MaxBackoff     time.Duration
}

type ControllerOptions struct {
	StateDirectory string
	Validate       ValidateFunc
	Launch         LaunchFunc
	Health         HealthFunc
	Restart        RestartPolicy
}

type RuntimeState string

const (
	StateStopped  RuntimeState = "stopped"
	StateStarting RuntimeState = "starting"
	StateRunning  RuntimeState = "running"
	StateStopping RuntimeState = "stopping"
	StateFailed   RuntimeState = "failed"
)

type Status struct {
	State           RuntimeState `json:"state"`
	PID             int          `json:"pid,omitempty"`
	Generation      uint64       `json:"generation"`
	RestartAttempts int          `json:"restart_attempts"`
	Abnormal        bool         `json:"abnormal"`
	LastError       string       `json:"last_error,omitempty"`
	CoreLog         string       `json:"core_log,omitempty"`
}

type Controller struct {
	mu      sync.Mutex
	options ControllerOptions
	status  Status
	process Process
	config  []byte
	closed  bool
}

func NewController(options ControllerOptions) (*Controller, error) {
	if options.StateDirectory == "" {
		return nil, errors.New("core state directory is required")
	}
	if options.Validate == nil || options.Launch == nil || options.Health == nil {
		return nil, errors.New("core validate, launch, and health functions are required")
	}
	if options.Restart.MaxAttempts < 0 {
		return nil, errors.New("restart attempts cannot be negative")
	}
	if options.Restart.InitialBackoff <= 0 {
		options.Restart.InitialBackoff = 500 * time.Millisecond
	}
	if options.Restart.MaxBackoff < options.Restart.InitialBackoff {
		options.Restart.MaxBackoff = 5 * time.Second
	}
	if err := os.MkdirAll(options.StateDirectory, 0o700); err != nil {
		return nil, fmt.Errorf("create core state directory: %w", err)
	}
	controller := &Controller{options: options, status: Status{State: StateStopped}}
	if data, err := os.ReadFile(filepath.Join(options.StateDirectory, "last-good.json")); err == nil {
		controller.config = data
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read last-good configuration: %w", err)
	}
	if _, err := os.Stat(filepath.Join(options.StateDirectory, "abnormal.json")); err == nil {
		controller.status.Abnormal = true
		controller.status.LastError = "previous helper session ended abnormally"
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("inspect abnormal marker: %w", err)
	}
	for _, name := range []string{"candidate.json", "rollback.json"} {
		_ = os.Remove(filepath.Join(options.StateDirectory, name))
	}
	return controller, nil
}

func (c *Controller) Validate(ctx context.Context, config []byte) error {
	if len(config) == 0 {
		return errors.New("core configuration is empty")
	}
	return c.options.Validate(ctx, append([]byte(nil), config...))
}

func (c *Controller) Apply(ctx context.Context, config []byte) error {
	if err := c.Validate(ctx, config); err != nil {
		return fmt.Errorf("validate candidate configuration: %w", err)
	}
	candidatePath := filepath.Join(c.options.StateDirectory, "candidate.json")
	if err := writeRestrictedFile(candidatePath, config); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed {
		return errors.New("core controller is closed")
	}
	previousConfig := append([]byte(nil), c.config...)
	c.status.Generation++
	generation := c.status.Generation
	if err := c.stopLocked(ctx); err != nil {
		return fmt.Errorf("stop previous core: %w", err)
	}
	c.status.State = StateStarting
	c.status.LastError = ""
	c.status.RestartAttempts = 0
	process, err := c.options.Launch(ctx, candidatePath)
	if err == nil {
		err = c.options.Health(ctx, process, config)
	}
	if err != nil {
		if process != nil {
			_ = terminateProcess(ctx, process)
		}
		c.status.State = StateFailed
		c.status.Abnormal = true
		c.status.LastError = err.Error()
		_ = c.writeMarkerLocked("apply-failed")
		if len(previousConfig) > 0 {
			if rollbackErr := c.startRollbackLocked(ctx, previousConfig, generation); rollbackErr != nil {
				return fmt.Errorf("start candidate: %w; rollback: %v", err, rollbackErr)
			}
		}
		return fmt.Errorf("start candidate: %w", err)
	}
	activePath := filepath.Join(c.options.StateDirectory, "active.json")
	if err := replaceRestrictedFile(candidatePath, activePath); err != nil {
		_ = terminateProcess(ctx, process)
		return err
	}
	if err := writeRestrictedFile(filepath.Join(c.options.StateDirectory, "last-good.json"), config); err != nil {
		_ = terminateProcess(ctx, process)
		return err
	}
	c.process = process
	c.config = append([]byte(nil), config...)
	c.status.State = StateRunning
	c.status.PID = process.PID()
	c.status.Abnormal = false
	_ = os.Remove(filepath.Join(c.options.StateDirectory, "abnormal.json"))
	go c.monitor(process, generation)
	return nil
}

func (c *Controller) Stop(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.status.Generation++
	return c.stopLocked(ctx)
}

func (c *Controller) Close(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.closed = true
	c.status.Generation++
	return c.stopLocked(ctx)
}

func (c *Controller) Status() Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.captureCoreLogLocked()
	return c.status
}

// FaultTerminateCore terminates only the core process currently owned by this
// controller. It is intended for explicitly enabled acceptance testing.
func (c *Controller) FaultTerminateCore() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.closed || c.status.State != StateRunning || c.process == nil {
		return errors.New("no running core process to terminate")
	}
	return c.process.Kill()
}

func (c *Controller) captureCoreLogLocked() {
	if output, ok := c.process.(interface{ Output() string }); ok {
		c.status.CoreLog = output.Output()
	}
}

func (c *Controller) stopLocked(ctx context.Context) error {
	if c.process == nil {
		c.status.State = StateStopped
		c.status.PID = 0
		return nil
	}
	c.status.State = StateStopping
	err := terminateProcess(ctx, c.process)
	c.captureCoreLogLocked()
	if err != nil {
		c.status.State = StateFailed
		c.status.LastError = err.Error()
		return err
	}
	c.process = nil
	c.status.PID = 0
	c.status.State = StateStopped
	return nil
}

func terminateProcess(ctx context.Context, process Process) error {
	if err := process.Stop(ctx); err == nil {
		return nil
	}
	return process.Kill()
}

func (c *Controller) startRollbackLocked(ctx context.Context, config []byte, generation uint64) error {
	path := filepath.Join(c.options.StateDirectory, "rollback.json")
	if err := writeRestrictedFile(path, config); err != nil {
		return err
	}
	process, err := c.options.Launch(ctx, path)
	if err == nil {
		err = c.options.Health(ctx, process, config)
	}
	if err != nil {
		if process != nil {
			_ = terminateProcess(ctx, process)
		}
		return err
	}
	c.process = process
	c.config = append([]byte(nil), config...)
	c.status.State = StateRunning
	c.status.PID = process.PID()
	c.status.LastError = "candidate failed; previous configuration restored"
	go c.monitor(process, generation)
	return nil
}

func (c *Controller) monitor(process Process, generation uint64) {
	<-process.Done()
	err := process.ExitError()
	c.mu.Lock()
	if c.closed || c.process != process || c.status.Generation != generation || c.status.State != StateRunning {
		c.mu.Unlock()
		return
	}
	c.process = nil
	c.status.PID = 0
	c.status.State = StateFailed
	c.status.Abnormal = true
	if err != nil {
		c.status.LastError = err.Error()
	} else {
		c.status.LastError = "core exited unexpectedly"
	}
	_ = c.writeMarkerLocked("unexpected-exit")
	config := append([]byte(nil), c.config...)
	policy := c.options.Restart
	c.mu.Unlock()
	backoff := policy.InitialBackoff
	for attempt := 1; attempt <= policy.MaxAttempts; attempt++ {
		timer := time.NewTimer(backoff)
		<-timer.C
		c.mu.Lock()
		if c.closed || c.status.Generation != generation {
			c.mu.Unlock()
			return
		}
		c.status.RestartAttempts = attempt
		path := filepath.Join(c.options.StateDirectory, "active.json")
		restarted, launchErr := c.options.Launch(context.Background(), path)
		if launchErr == nil {
			launchErr = c.options.Health(context.Background(), restarted, config)
		}
		if launchErr == nil {
			c.process = restarted
			c.status.State = StateRunning
			c.status.PID = restarted.PID()
			c.mu.Unlock()
			go c.monitor(restarted, generation)
			return
		}
		if restarted != nil {
			_ = terminateProcess(context.Background(), restarted)
		}
		c.status.LastError = launchErr.Error()
		_ = c.writeMarkerLocked("restart-failed")
		c.mu.Unlock()
		backoff *= 2
		if backoff > policy.MaxBackoff {
			backoff = policy.MaxBackoff
		}
	}
}

func (c *Controller) writeMarkerLocked(reason string) error {
	data, _ := json.MarshalIndent(map[string]any{"reason": reason, "time": time.Now().UTC(), "status": c.status}, "", "  ")
	return writeRestrictedFile(filepath.Join(c.options.StateDirectory, "abnormal.json"), append(data, '\n'))
}

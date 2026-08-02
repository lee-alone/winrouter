package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeProcess struct {
	pid     int
	done    chan struct{}
	once    sync.Once
	mu      sync.Mutex
	exitErr error
	stopErr error
	killErr error
}

func newFakeProcess(pid int) *fakeProcess    { return &fakeProcess{pid: pid, done: make(chan struct{})} }
func (p *fakeProcess) PID() int              { return p.pid }
func (p *fakeProcess) Done() <-chan struct{} { return p.done }
func (p *fakeProcess) ExitError() error      { p.mu.Lock(); defer p.mu.Unlock(); return p.exitErr }
func (p *fakeProcess) Stop(context.Context) error {
	if p.stopErr != nil {
		return p.stopErr
	}
	p.exit(nil)
	return nil
}
func (p *fakeProcess) Kill() error {
	if p.killErr != nil {
		return p.killErr
	}
	p.exit(errors.New("killed"))
	return nil
}
func (p *fakeProcess) exit(err error) {
	p.once.Do(func() { p.mu.Lock(); p.exitErr = err; p.mu.Unlock(); close(p.done) })
}

func TestControllerAppliesCommitsAndStops(t *testing.T) {
	directory := t.TempDir()
	process := newFakeProcess(42)
	controller := newFakeController(t, directory, func(context.Context, string) (Process, error) { return process, nil }, func(context.Context, Process, []byte) error { return nil })
	config := []byte(`{"version":1}`)
	if err := controller.Apply(context.Background(), config); err != nil {
		t.Fatal(err)
	}
	status := controller.Status()
	if status.State != StateRunning || status.PID != 42 || status.Abnormal {
		t.Fatalf("status = %#v", status)
	}
	for _, name := range []string{"active.json", "last-good.json"} {
		data, err := os.ReadFile(filepath.Join(directory, name))
		if err != nil || string(data) != string(config) {
			t.Fatalf("%s = %q, %v", name, data, err)
		}
	}
	if err := controller.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if status := controller.Status(); status.State != StateStopped || status.PID != 0 {
		t.Fatalf("status = %#v", status)
	}
}

func TestControllerRollsBackFailedCandidate(t *testing.T) {
	directory := t.TempDir()
	first := newFakeProcess(1)
	failed := newFakeProcess(2)
	rollback := newFakeProcess(3)
	var launches atomic.Int32
	controller := newFakeController(t, directory, func(context.Context, string) (Process, error) {
		switch launches.Add(1) {
		case 1:
			return first, nil
		case 2:
			return failed, nil
		default:
			return rollback, nil
		}
	}, func(_ context.Context, process Process, config []byte) error {
		if process.PID() == 2 {
			return errors.New("unhealthy")
		}
		return nil
	})
	if err := controller.Apply(context.Background(), []byte(`{"config":"good"}`)); err != nil {
		t.Fatal(err)
	}
	if err := controller.Apply(context.Background(), []byte(`{"config":"bad"}`)); err == nil {
		t.Fatal("bad candidate succeeded")
	}
	status := controller.Status()
	if status.State != StateRunning || status.PID != 3 || status.LastError == "" {
		t.Fatalf("rollback status = %#v", status)
	}
	if string(controller.config) != `{"config":"good"}` {
		t.Fatalf("active config = %s", controller.config)
	}
}

func TestControllerRetainsFailedStopForRetry(t *testing.T) {
	directory := t.TempDir()
	process := newFakeProcess(11)
	process.stopErr = errors.New("stop failed")
	process.killErr = errors.New("kill failed")
	controller := newFakeController(t, directory, func(context.Context, string) (Process, error) {
		return process, nil
	}, func(context.Context, Process, []byte) error { return nil })
	if err := controller.Apply(context.Background(), []byte(`{"config":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := controller.Stop(context.Background()); err == nil {
		t.Fatal("expected stop failure")
	}
	if status := controller.Status(); status.State != StateFailed || status.PID != process.PID() {
		t.Fatalf("failed stop lost process ownership: %#v", status)
	}

	process.stopErr = nil
	if err := controller.Stop(context.Background()); err != nil {
		t.Fatal(err)
	}
	if status := controller.Status(); status.State != StateStopped || status.PID != 0 {
		t.Fatalf("retry stop status = %#v", status)
	}
}

func TestControllerRestartsWithBoundedBackoff(t *testing.T) {
	directory := t.TempDir()
	initial := newFakeProcess(1)
	restarted := newFakeProcess(2)
	var launches atomic.Int32
	controller := newFakeController(t, directory, func(context.Context, string) (Process, error) {
		if launches.Add(1) == 1 {
			return initial, nil
		}
		return restarted, nil
	}, func(context.Context, Process, []byte) error { return nil })
	controller.options.Restart = RestartPolicy{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: 2 * time.Millisecond}
	if err := controller.Apply(context.Background(), []byte(`{"config":1}`)); err != nil {
		t.Fatal(err)
	}
	initial.exit(errors.New("crash"))
	waitStatus(t, controller, func(status Status) bool { return status.State == StateRunning && status.PID == 2 })
	if controller.Status().RestartAttempts != 1 {
		t.Fatalf("status = %#v", controller.Status())
	}
	if _, err := os.Stat(filepath.Join(directory, "abnormal.json")); err != nil {
		t.Fatalf("abnormal marker: %v", err)
	}
}

func TestControllerStopsAfterRestartLimit(t *testing.T) {
	directory := t.TempDir()
	initial := newFakeProcess(1)
	var launches atomic.Int32
	controller := newFakeController(t, directory, func(context.Context, string) (Process, error) {
		if launches.Add(1) == 1 {
			return initial, nil
		}
		return nil, errors.New("launch failed")
	}, func(context.Context, Process, []byte) error { return nil })
	controller.options.Restart = RestartPolicy{MaxAttempts: 2, InitialBackoff: time.Millisecond, MaxBackoff: time.Millisecond}
	if err := controller.Apply(context.Background(), []byte(`{"config":1}`)); err != nil {
		t.Fatal(err)
	}
	initial.exit(errors.New("crash"))
	waitStatus(t, controller, func(status Status) bool { return status.State == StateFailed && status.RestartAttempts == 2 })
	if launches.Load() != 3 {
		t.Fatalf("launches = %d", launches.Load())
	}
}

func TestControllerFaultTerminateCoreKillsOwnedProcess(t *testing.T) {
	process := newFakeProcess(7)
	controller := newFakeController(t, t.TempDir(), func(context.Context, string) (Process, error) {
		return process, nil
	}, func(context.Context, Process, []byte) error { return nil })
	if err := controller.Apply(context.Background(), []byte(`{"config":1}`)); err != nil {
		t.Fatal(err)
	}
	if err := controller.FaultTerminateCore(); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, controller, func(status Status) bool { return status.State == StateFailed && status.Abnormal })
	if err := controller.FaultTerminateCore(); err == nil {
		t.Fatal("fault termination succeeded without a running owned process")
	}
}

func TestControllerRestoresOwnedStateOnly(t *testing.T) {
	directory := t.TempDir()
	good := []byte(`{"last":"good"}`)
	if err := os.WriteFile(filepath.Join(directory, "last-good.json"), good, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "candidate.json"), []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	foreign := filepath.Join(directory, "user-file.txt")
	if err := os.WriteFile(foreign, []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}
	controller := newFakeController(t, directory, func(context.Context, string) (Process, error) { return newFakeProcess(1), nil }, func(context.Context, Process, []byte) error { return nil })
	if string(controller.config) != string(good) {
		t.Fatalf("restored config = %s", controller.config)
	}
	if _, err := os.Stat(filepath.Join(directory, "candidate.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("candidate still exists: %v", err)
	}
	if data, err := os.ReadFile(foreign); err != nil || string(data) != "keep" {
		t.Fatalf("foreign file changed: %q %v", data, err)
	}
}

func newFakeController(t *testing.T, directory string, launch LaunchFunc, health HealthFunc) *Controller {
	t.Helper()
	controller, err := NewController(ControllerOptions{StateDirectory: directory, Validate: func(context.Context, []byte) error { return nil }, Launch: launch, Health: health, Restart: RestartPolicy{MaxAttempts: 0}})
	if err != nil {
		t.Fatal(err)
	}
	return controller
}

func waitStatus(t *testing.T, controller *Controller, condition func(Status) bool) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for !condition(controller.Status()) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out, status = %#v", controller.Status())
		}
		time.Sleep(time.Millisecond)
	}
}

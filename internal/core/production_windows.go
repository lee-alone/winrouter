//go:build windows

package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"

	"winrouter/internal/interfaces"
)

type ProductionOptions struct {
	CorePath       string
	StateDirectory string
	Restart        RestartPolicy
}

func NewProductionController(options ProductionOptions) (*Controller, error) {
	if options.CorePath == "" {
		return nil, errors.New("locked core path is required")
	}
	launcher := &windowsLauncher{executable: options.CorePath}
	return NewController(ControllerOptions{
		StateDirectory: options.StateDirectory,
		Validate: func(ctx context.Context, data []byte) error {
			_, err := ValidateCore(ctx, options.CorePath, data)
			return err
		},
		Launch: launcher.Start, Health: waitForCoreHealth, Restart: options.Restart,
	})
}

type windowsLauncher struct{ executable string }

func (l *windowsLauncher) Start(_ context.Context, configPath string) (Process, error) {
	command := exec.Command(l.executable, "run", "-c", configPath)
	command.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NEW_PROCESS_GROUP}
	output := &boundedBuffer{limit: 256 * 1024}
	command.Stdout = output
	command.Stderr = output
	job, err := createKillOnCloseJob()
	if err != nil {
		return nil, err
	}
	if err := command.Start(); err != nil {
		windows.CloseHandle(job)
		return nil, fmt.Errorf("start locked core: %w", err)
	}
	if err := assignProcessToJob(job, command.Process.Pid); err != nil {
		_ = command.Process.Kill()
		_ = command.Wait()
		windows.CloseHandle(job)
		return nil, err
	}
	process := &windowsProcess{command: command, job: job, done: make(chan struct{}), output: output}
	go process.wait()
	return process, nil
}

type windowsProcess struct {
	command *exec.Cmd
	job     windows.Handle
	done    chan struct{}
	output  *boundedBuffer
	mu      sync.RWMutex
	exitErr error
}

func (p *windowsProcess) PID() int              { return p.command.Process.Pid }
func (p *windowsProcess) Done() <-chan struct{} { return p.done }
func (p *windowsProcess) ExitError() error      { p.mu.RLock(); defer p.mu.RUnlock(); return p.exitErr }
func (p *windowsProcess) Output() string        { return p.output.String() }
func (p *windowsProcess) wait() {
	err := p.command.Wait()
	p.mu.Lock()
	p.exitErr = err
	p.mu.Unlock()
	windows.CloseHandle(p.job)
	close(p.done)
}
func (p *windowsProcess) Stop(ctx context.Context) error {
	if err := windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, uint32(p.PID())); err != nil {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(5 * time.Second):
		return errors.New("core ignored CTRL_BREAK")
	}
}
func (p *windowsProcess) Kill() error {
	select {
	case <-p.done:
		return nil
	default:
	}
	if err := p.command.Process.Kill(); err != nil {
		return err
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(5 * time.Second):
		return errors.New("core did not exit after kill")
	}
}

func waitForCoreHealth(ctx context.Context, process Process, data []byte) error {
	var expected struct {
		Inbounds []struct {
			Type          string   `json:"type"`
			InterfaceName string   `json:"interface_name"`
			Address       []string `json:"address"`
		} `json:"inbounds"`
	}
	if err := json.Unmarshal(data, &expected); err != nil {
		return fmt.Errorf("decode health configuration: %w", err)
	}
	interfaceName, address := "", ""
	for _, inbound := range expected.Inbounds {
		if inbound.Type == "tun" && len(inbound.Address) > 0 {
			interfaceName, address = inbound.InterfaceName, strings.Split(inbound.Address[0], "/")[0]
			break
		}
	}
	if interfaceName == "" {
		return errors.New("health configuration has no TUN inbound")
	}
	ticker := time.NewTicker(200 * time.Millisecond)
	defer ticker.Stop()
	timeout := time.NewTimer(15 * time.Second)
	defer timeout.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout.C:
			return fmt.Errorf("TUN interface %q did not become healthy", interfaceName)
		case <-process.Done():
			return fmt.Errorf("core exited during health check: %v", process.ExitError())
		case <-ticker.C:
			adapters, err := interfaces.Enumerate()
			if err != nil {
				continue
			}
			for _, adapter := range adapters {
				if adapter.FriendlyName != interfaceName || adapter.Status != "up" {
					continue
				}
				for _, item := range adapter.Addresses {
					if item.IP == address {
						return nil
					}
				}
			}
		}
	}
}

type boundedBuffer struct {
	mu    sync.Mutex
	data  []byte
	limit int
}

func (b *boundedBuffer) Write(value []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.data = append(b.data, value...)
	if len(b.data) > b.limit {
		b.data = append([]byte(nil), b.data[len(b.data)-b.limit:]...)
	}
	return len(value), nil
}
func (b *boundedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(append([]byte(nil), b.data...))
}

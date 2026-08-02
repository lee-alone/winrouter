//go:build windows

package helperclient

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"golang.org/x/sys/windows"

	"winrouter/internal/helperipc"
)

type Session struct {
	Client    helperipc.Client
	cancel    context.CancelFunc
	done      chan struct{}
	closeOnce sync.Once
}

func Launch(ctx context.Context) (*Session, error) {
	return launch(ctx, false)
}

func LaunchFaultTest(ctx context.Context) (*Session, error) {
	return launch(ctx, true)
}

func launch(ctx context.Context, allowFaultTermination bool) (*Session, error) {
	if helperipc.IsElevated() {
		return nil, errors.New("UI process must not run elevated")
	}
	token, err := helperipc.GenerateToken()
	if err != nil {
		return nil, err
	}
	executable, err := os.Executable()
	if err != nil {
		return nil, err
	}
	helperPath := filepath.Join(filepath.Dir(executable), "WinRouter-helper.exe")
	if info, err := os.Stat(helperPath); err != nil || info.IsDir() {
		return nil, fmt.Errorf("locate fixed helper executable: %w", err)
	}
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(helperPath)
	argumentText := "--auth-token " + token
	if allowFaultTermination {
		argumentText += " --allow-test-core-termination"
	}
	arguments, _ := windows.UTF16PtrFromString(argumentText)
	directory, _ := windows.UTF16PtrFromString(filepath.Dir(helperPath))
	if err := windows.ShellExecute(0, verb, file, arguments, directory, windows.SW_HIDE); err != nil {
		return nil, fmt.Errorf("start elevated helper: %w", err)
	}
	client := helperipc.Client{AuthToken: token, Dial: helperipc.DefaultDialer(time.Second)}
	deadline := time.Now().Add(15 * time.Second)
	for {
		var heartbeat helperipc.Heartbeat
		if err := client.Call(helperipc.MethodHeartbeat, nil, &heartbeat); err == nil {
			break
		}
		if time.Now().After(deadline) {
			return nil, errors.New("elevated helper did not become ready")
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(200 * time.Millisecond):
		}
	}
	heartbeatCtx, cancel := context.WithCancel(context.Background())
	session := &Session{Client: client, cancel: cancel, done: make(chan struct{})}
	go session.heartbeat(heartbeatCtx)
	return session, nil
}

func (s *Session) heartbeat(ctx context.Context) {
	defer close(s.done)
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			var result helperipc.Heartbeat
			_ = s.Client.Call(helperipc.MethodHeartbeat, nil, &result)
		}
	}
}

func (s *Session) Close() {
	s.closeOnce.Do(func() {
		_ = s.Client.Call(helperipc.MethodShutdown, nil, nil)
		s.cancel()
		<-s.done
	})
}

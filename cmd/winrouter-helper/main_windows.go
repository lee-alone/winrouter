//go:build windows

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"winrouter/internal/core"
	"winrouter/internal/helperipc"
)

var faultInjectionBuild = "false"

func main() {
	authToken := flag.String("auth-token", "", "one-time UI authentication token")
	allowFaultTermination := flag.Bool("allow-test-core-termination", false, "enable constrained core crash acceptance testing")
	flag.Parse()
	if *allowFaultTermination && faultInjectionBuild != "true" {
		fmt.Fprintln(os.Stderr, "core fault injection is not available in this build")
		os.Exit(2)
	}
	if !helperipc.IsElevated() {
		fmt.Fprintln(os.Stderr, "winrouter-helper requires an elevated token")
		os.Exit(2)
	}
	if len(*authToken) < 32 {
		fmt.Fprintln(os.Stderr, "a high-entropy authentication token is required")
		os.Exit(2)
	}
	corePath, err := core.Locate()
	if err != nil {
		fail(fmt.Errorf("locate sing-box core: %w", err))
	}
	programData := os.Getenv("ProgramData")
	if programData == "" {
		fail(fmt.Errorf("ProgramData is not defined"))
	}
	sid, err := helperipc.CurrentUserSID()
	if err != nil {
		fail(err)
	}
	controller, err := core.NewProductionController(core.ProductionOptions{
		CorePath: corePath, StateDirectory: filepath.Join(programData, "WinRouter", "state", sid),
		Restart: core.RestartPolicy{MaxAttempts: 3, InitialBackoff: time.Second, MaxBackoff: 4 * time.Second},
	})
	if err != nil {
		fail(err)
	}
	service := helperipc.ControllerService{Controller: controller, Timeout: 30 * time.Second, AllowFaultTermination: *allowFaultTermination}
	server, err := helperipc.NewServer(*authToken, service)
	if err != nil {
		fail(err)
	}
	listener, _, err := helperipc.ListenCurrentUserPipe()
	if err != nil {
		fail(err)
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if time.Since(server.LastActivity()) > 20*time.Second {
					cancel()
					return
				}
			}
		}
	}()
	go func() {
		select {
		case <-ctx.Done():
		case <-server.ShutdownRequested():
			cancel()
		}
	}()
	go func() { <-ctx.Done(); _ = listener.Close() }()
	if err := server.Serve(listener); err != nil {
		fail(err)
	}
	shutdown, stop := context.WithTimeout(context.Background(), 10*time.Second)
	defer stop()
	if err := controller.Close(shutdown); err != nil {
		fail(err)
	}
}

func fail(err error) { fmt.Fprintln(os.Stderr, err); os.Exit(1) }

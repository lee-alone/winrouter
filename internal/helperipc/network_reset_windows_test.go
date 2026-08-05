//go:build windows

package helperipc

import (
	"context"
	"errors"
	"testing"

	"winrouter/internal/core"
)

func TestResetNetworkStackUsesFixedCommandSequence(t *testing.T) {
	controller, err := core.NewController(core.ControllerOptions{
		StateDirectory: t.TempDir(),
		Validate:       func(context.Context, []byte) error { return nil },
		Launch:         func(context.Context, string) (core.Process, error) { return nil, errors.New("must not launch") },
		Health:         func(context.Context, core.Process, []byte) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	original := runNetworkResetCommand
	defer func() { runNetworkResetCommand = original }()
	var commands []networkResetCommand
	runNetworkResetCommand = func(command networkResetCommand) NetworkResetStep {
		commands = append(commands, command)
		return NetworkResetStep{Command: command.name, Success: true}
	}
	result, err := (ControllerService{Controller: controller}).ResetNetworkStack()
	if err != nil {
		t.Fatal(err)
	}
	if !result.Success || !result.RestartRequired || len(commands) != 3 {
		t.Fatalf("result=%#v commands=%#v", result, commands)
	}
	want := []string{"netsh winsock reset", "netsh int ip reset", "ipconfig /flushdns"}
	for index := range want {
		if commands[index].name != want[index] {
			t.Fatalf("command %d = %q, want %q", index, commands[index].name, want[index])
		}
	}
}

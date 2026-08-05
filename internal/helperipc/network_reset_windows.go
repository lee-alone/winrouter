//go:build windows

package helperipc

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows"
)

const maxNetworkResetOutput = 16 << 10

type networkResetCommand struct {
	name string
	path string
	args []string
}

var runNetworkResetCommand = executeNetworkResetCommand

func (s ControllerService) ResetNetworkStack() (NetworkResetResult, error) {
	if s.Controller.Status().State != "stopped" {
		return NetworkResetResult{}, errors.New("routing core must be stopped before resetting the network stack")
	}
	systemDirectory, err := windows.GetSystemDirectory()
	if err != nil {
		return NetworkResetResult{}, err
	}
	commands := []networkResetCommand{
		{name: "netsh winsock reset", path: filepath.Join(systemDirectory, "netsh.exe"), args: []string{"winsock", "reset"}},
		{name: "netsh int ip reset", path: filepath.Join(systemDirectory, "netsh.exe"), args: []string{"int", "ip", "reset"}},
		{name: "ipconfig /flushdns", path: filepath.Join(systemDirectory, "ipconfig.exe"), args: []string{"/flushdns"}},
	}
	result := NetworkResetResult{Success: true, RestartRequired: true, Steps: make([]NetworkResetStep, 0, len(commands))}
	for _, command := range commands {
		step := runNetworkResetCommand(command)
		result.Steps = append(result.Steps, step)
		result.Success = result.Success && step.Success
	}
	return result, nil
}

func executeNetworkResetCommand(command networkResetCommand) NetworkResetStep {
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	process := exec.CommandContext(ctx, command.path, command.args...)
	output, err := process.CombinedOutput()
	step := NetworkResetStep{Command: command.name, Success: err == nil, ExitCode: 0, Output: limitedCommandOutput(output)}
	if err == nil {
		return step
	}
	step.ExitCode = -1
	var exitError *exec.ExitError
	if errors.As(err, &exitError) {
		step.ExitCode = exitError.ExitCode()
	} else if step.Output == "" {
		step.Output = err.Error()
	}
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		step.Output = strings.TrimSpace(step.Output + "\ncommand timed out")
	}
	return step
}

func limitedCommandOutput(output []byte) string {
	if len(output) > maxNetworkResetOutput {
		output = output[:maxNetworkResetOutput]
	}
	return strings.TrimSpace(strings.ToValidUTF8(string(output), "?"))
}

package autostart

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const ValueName = "WinRouter"

type Status struct {
	Enabled           bool   `json:"enabled"`
	RegisteredCommand string `json:"registered_command,omitempty"`
}

func Get() (Status, error) {
	command, found, err := readCommand()
	if err != nil {
		return Status{}, err
	}
	expected, err := currentCommand()
	if err != nil {
		return Status{}, err
	}
	return Status{Enabled: found && strings.EqualFold(command, expected), RegisteredCommand: command}, nil
}

func Set(enabled bool) (Status, error) {
	if !enabled {
		if err := deleteCommand(); err != nil {
			return Status{}, fmt.Errorf("disable autostart: %w", err)
		}
		return Get()
	}
	command, err := currentCommand()
	if err != nil {
		return Status{}, err
	}
	if err := writeCommand(command); err != nil {
		return Status{}, fmt.Errorf("enable autostart: %w", err)
	}
	status, err := Get()
	if err != nil {
		return Status{}, err
	}
	if !status.Enabled {
		return Status{}, errors.New("autostart registration could not be verified")
	}
	return status, nil
}

func currentCommand() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("locate application executable: %w", err)
	}
	executable, err = filepath.Abs(executable)
	if err != nil {
		return "", fmt.Errorf("resolve application executable: %w", err)
	}
	if !strings.EqualFold(filepath.Ext(executable), ".exe") {
		return "", fmt.Errorf("application executable must be an .exe: %s", executable)
	}
	return commandForExecutable(executable), nil
}

func commandForExecutable(executable string) string {
	return `"` + executable + `" --autostart`
}

//go:build !windows

package autostart

import "errors"

func readCommand() (string, bool, error) { return "", false, nil }
func writeCommand(string) error          { return errors.New("autostart is only supported on Windows") }
func deleteCommand() error               { return nil }

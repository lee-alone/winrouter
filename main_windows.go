//go:build windows

package main

import (
	"os"
	"strings"

	"golang.org/x/sys/windows"

	"winrouter/internal/helperipc"
)

func ensureElevated() {
	if helperipc.IsElevated() {
		return
	}
	for _, arg := range os.Args[1:] {
		if arg == "--no-elevate" || arg == "--elevated-attempt" {
			return
		}
	}
	executable, err := os.Executable()
	if err != nil {
		return
	}
	cwd, _ := os.Getwd()
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(executable)
	dir, _ := windows.UTF16PtrFromString(cwd)

	forwardArgs := append([]string(nil), os.Args[1:]...)
	forwardArgs = append(forwardArgs, "--elevated-attempt")

	var quoted []string
	for _, a := range forwardArgs {
		if strings.ContainsAny(a, " \t\"") {
			escaped := strings.ReplaceAll(a, `"`, `\"`)
			quoted = append(quoted, `"`+escaped+`"`)
		} else {
			quoted = append(quoted, a)
		}
	}
	argText := strings.Join(quoted, " ")
	args, _ := windows.UTF16PtrFromString(argText)

	if err := windows.ShellExecute(0, verb, file, args, dir, windows.SW_NORMAL); err == nil {
		os.Exit(0)
	}
}

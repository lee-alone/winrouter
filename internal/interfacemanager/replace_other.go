//go:build !windows

package interfacemanager

import "os"

func replaceStateFile(source, destination string) error { return os.Rename(source, destination) }

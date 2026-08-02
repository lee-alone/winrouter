//go:build !windows

package core

import "os"

func replaceCoreFile(source, destination string) error { return os.Rename(source, destination) }

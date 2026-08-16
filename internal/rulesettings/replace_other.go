//go:build !windows

package rulesettings

import "os"

func replaceFile(source, destination string) error { return os.Rename(source, destination) }

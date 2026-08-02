//go:build !windows

package nodes

import "os"

func replaceFile(source, target string) error { return os.Rename(source, target) }

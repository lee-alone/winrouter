//go:build !windows

package trafficbudget

import "os"

func replaceFile(source, target string) error { return os.Rename(source, target) }

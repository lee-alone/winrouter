//go:build !windows

package subscriptions

import "os"

func replaceFile(source, target string) error { return os.Rename(source, target) }

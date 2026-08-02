//go:build !windows

package rulesets

import "os"

func replaceFile(source, target string) error { return os.Rename(source, target) }

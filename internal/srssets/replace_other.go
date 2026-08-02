//go:build !windows

package srssets

import (
	"os"
	"path/filepath"
)

func replaceFile(source, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}
	return os.Rename(source, target)
}

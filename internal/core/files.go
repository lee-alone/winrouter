package core

import (
	"fmt"
	"os"
	"path/filepath"
)

func writeRestrictedFile(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".winrouter-*.tmp")
	if err != nil {
		return fmt.Errorf("create state file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(data)
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write state file: %w", err)
	}
	if err := replaceCoreFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace state file: %w", err)
	}
	return nil
}

func replaceRestrictedFile(source, destination string) error {
	if err := replaceCoreFile(source, destination); err != nil {
		return fmt.Errorf("commit state file: %w", err)
	}
	return nil
}

package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestValidateRejectsHashMismatchBeforeExecution(t *testing.T) {
	path := filepath.Join(t.TempDir(), "not-a-core.exe")
	if err := os.WriteFile(path, []byte("not an executable"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, err := Validate(context.Background(), path, LockedVersion, strings.Repeat("0", 64), []byte("{}"))
	if err == nil || !strings.Contains(err.Error(), "SHA-256 mismatch") {
		t.Fatalf("Validate() error = %v, want SHA-256 mismatch", err)
	}
}

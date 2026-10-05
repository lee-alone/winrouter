package configdir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveCustomEnv(t *testing.T) {
	temp := t.TempDir()
	customDir := filepath.Join(temp, "custom-config")
	t.Setenv("WINROUTER_CONFIG_DIR", customDir)

	info, err := Resolve()
	if err != nil {
		t.Fatalf("Resolve() returned error: %v", err)
	}
	if info.Path != customDir {
		t.Fatalf("expected path %q, got %q", customDir, info.Path)
	}
	if !info.IsPortable {
		t.Fatalf("expected isPortable=true")
	}
}

func TestDirectoryWritable(t *testing.T) {
	temp := t.TempDir()
	if !isDirectoryWritable(temp) {
		t.Fatalf("expected temp dir to be writable")
	}
}

func TestCopyFile(t *testing.T) {
	temp := t.TempDir()
	src := filepath.Join(temp, "src.txt")
	dst := filepath.Join(temp, "dst.txt")

	if err := os.WriteFile(src, []byte("hello world"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile failed: %v", err)
	}
	data, err := os.ReadFile(dst)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "hello world" {
		t.Fatalf("unexpected content: %s", string(data))
	}
}

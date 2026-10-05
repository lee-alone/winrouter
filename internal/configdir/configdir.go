package configdir

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

const (
	ConfigFolderName = "config"
	AppFolderName    = "WinRouter"
)

// Info contains details about resolved configuration directory.
type Info struct {
	Path       string `json:"path"`
	IsPortable bool   `json:"is_portable"`
	Migrated   bool   `json:"migrated"`
}

// Resolve determines the appropriate configuration directory.
// Priority:
// 1. Environment variable WINROUTER_CONFIG_DIR (if set and usable)
// 2. Portable directory: "<exe_dir>/config" if writable
// 3. Fallback: "%APPDATA%/WinRouter" (or system user config dir)
func Resolve() (Info, error) {
	if custom := os.Getenv("WINROUTER_CONFIG_DIR"); custom != "" {
		if err := os.MkdirAll(custom, 0o700); err == nil && isDirectoryWritable(custom) {
			return Info{Path: custom, IsPortable: true}, nil
		}
	}

	exePath, err := os.Executable()
	if err == nil {
		exeDir := filepath.Dir(exePath)
		portableDir := filepath.Join(exeDir, ConfigFolderName)
		if err := os.MkdirAll(portableDir, 0o700); err == nil && isDirectoryWritable(portableDir) {
			migrated := tryMigrateLegacyConfig(portableDir)
			return Info{Path: portableDir, IsPortable: true, Migrated: migrated}, nil
		}
	}

	// Fallback to system user config dir
	systemRoot, err := os.UserConfigDir()
	if err != nil {
		return Info{}, err
	}
	systemDir := filepath.Join(systemRoot, AppFolderName)
	if err := os.MkdirAll(systemDir, 0o700); err != nil {
		return Info{}, err
	}
	return Info{Path: systemDir, IsPortable: false}, nil
}

func isDirectoryWritable(dir string) bool {
	testFile := filepath.Join(dir, ".writetest")
	f, err := os.OpenFile(testFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return false
	}
	_ = f.Close()
	_ = os.Remove(testFile)
	return true
}

func tryMigrateLegacyConfig(targetDir string) bool {
	// If target directory already contains config files, no need to migrate
	marker := filepath.Join(targetDir, "interfaces.json")
	if _, err := os.Stat(marker); err == nil {
		return false
	}

	systemRoot, err := os.UserConfigDir()
	if err != nil {
		return false
	}
	legacyDir := filepath.Join(systemRoot, AppFolderName)
	if _, err := os.Stat(filepath.Join(legacyDir, "interfaces.json")); err != nil {
		return false
	}

	files := []string{
		"interfaces.json",
		"nodes.json",
		"subscriptions.json",
		"rule-settings.json",
		"dns-settings.json",
		"srs-sources.json",
		"traffic-budget.json",
	}

	migratedAny := false
	for _, name := range files {
		src := filepath.Join(legacyDir, name)
		dst := filepath.Join(targetDir, name)
		if copyFile(src, dst) == nil {
			migratedAny = true
		}
	}
	return migratedAny
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()

	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Sync()
}

// EnsureExists ensures the specified configuration directory exists and is writable.
func EnsureExists(dir string) error {
	if dir == "" {
		return errors.New("configuration directory is empty")
	}
	return os.MkdirAll(dir, 0o700)
}

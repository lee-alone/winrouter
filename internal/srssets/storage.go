package srssets

import (
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

//go:embed embedded/*.srs
var embeddedRules embed.FS

func (m *Manager) cachePath(id string) string {
	return filepath.Join(filepath.Dir(m.path), "geo", id+".srs")
}

func migrateLegacySRS(dir string) {
	legacyDir := filepath.Join(dir, "srs")
	geoDir := filepath.Join(dir, "geo")
	entries, err := os.ReadDir(legacyDir)
	if err != nil || len(entries) == 0 {
		return
	}
	_ = os.MkdirAll(geoDir, 0o700)
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".srs") {
			continue
		}
		src := filepath.Join(legacyDir, entry.Name())
		dst := filepath.Join(geoDir, entry.Name())
		if _, err := os.Stat(dst); os.IsNotExist(err) {
			if data, err := os.ReadFile(src); err == nil {
				_ = os.WriteFile(dst, data, 0o600)
			}
		}
	}
}

func (m *Manager) seedDefaults() {
	exeDir := ""
	if exe, err := os.Executable(); err == nil {
		exeDir = filepath.Dir(exe)
	}
	geoDir := filepath.Join(filepath.Dir(m.path), "geo")
	_ = os.MkdirAll(geoDir, 0o700)

	stateDirty := false
	for index, source := range m.state.Sources {
		targetFile := m.cachePath(source.ID)
		baseName := strings.TrimPrefix(source.ID, "sagernet-") + ".srs"

		if _, err := os.Stat(targetFile); os.IsNotExist(err) {
			var seeded []byte
			if exeDir != "" {
				candidates := []string{
					filepath.Join(exeDir, "geo", source.ID+".srs"),
					filepath.Join(exeDir, "geo", baseName),
					filepath.Join(exeDir, "resources", "geo", source.ID+".srs"),
					filepath.Join(exeDir, "resources", "geo", baseName),
				}
				for _, candidate := range candidates {
					if data, err := os.ReadFile(candidate); err == nil && len(data) >= 3 && string(data[:3]) == "SRS" {
						seeded = data
						break
					}
				}
			}
			if len(seeded) == 0 {
				for _, name := range []string{"embedded/" + baseName, "embedded/" + source.ID + ".srs"} {
					if data, err := embeddedRules.ReadFile(name); err == nil && len(data) >= 3 && string(data[:3]) == "SRS" {
						seeded = data
						break
					}
				}
			}
			if len(seeded) > 0 {
				_ = replaceFileFromBytes(seeded, targetFile)
			}
		}

		if data, err := os.ReadFile(targetFile); err == nil && len(data) >= 3 && string(data[:3]) == "SRS" {
			digest := sha256.Sum256(data)
			actual := hex.EncodeToString(digest[:])
			if source.AppliedSHA256 == "" || source.Size == 0 {
				source.AppliedSHA256 = actual
				source.Size = int64(len(data))
				if source.UpdatedAt.IsZero() {
					source.UpdatedAt = time.Now().UTC()
				}
				m.state.Sources[index] = source
				stateDirty = true
			}
		}
	}
	if stateDirty {
		_ = m.save(m.state)
	}
}

func replaceFileFromBytes(data []byte, target string) error {
	dir := filepath.Dir(target)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".srs-seed-*.tmp")
	if err != nil {
		return err
	}
	tmpName := f.Name()
	defer os.Remove(tmpName)
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return replaceFile(tmpName, target)
}

func (m *Manager) save(next state) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(m.path, append(data, '\n'))
}

func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".srs-state-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	_ = file.Chmod(0o600)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(name, path)
}

func newID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("custom-%d", time.Now().UnixNano())
	}
	return "custom-" + hex.EncodeToString(value[:])
}

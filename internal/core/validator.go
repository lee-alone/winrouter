package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type Validation struct {
	VersionOutput string `json:"version_output"`
	SHA256        string `json:"sha256"`
	CheckOutput   string `json:"check_output"`
}

func Validate(ctx context.Context, executable, expectedVersion, expectedSHA256 string, config []byte) (Validation, error) {
	binary, err := os.ReadFile(executable)
	if err != nil {
		return Validation{}, fmt.Errorf("read core executable: %w", err)
	}
	digest := sha256.Sum256(binary)
	hash := strings.ToUpper(hex.EncodeToString(digest[:]))
	if !strings.EqualFold(hash, expectedSHA256) {
		return Validation{}, fmt.Errorf("core SHA-256 mismatch: got %s, want %s", hash, expectedSHA256)
	}

	versionOutput, err := exec.CommandContext(ctx, executable, "version").CombinedOutput()
	if err != nil {
		return Validation{}, fmt.Errorf("query core version: %w: %s", err, strings.TrimSpace(string(versionOutput)))
	}
	if !strings.Contains(string(versionOutput), "sing-box version "+expectedVersion) {
		return Validation{}, fmt.Errorf("core version mismatch: expected %s", expectedVersion)
	}

	file, err := os.CreateTemp("", "winrouter-sing-box-*.json")
	if err != nil {
		return Validation{}, fmt.Errorf("create temporary config: %w", err)
	}
	path := file.Name()
	defer os.Remove(path)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return Validation{}, fmt.Errorf("restrict temporary config: %w", err)
	}
	if _, err := file.Write(config); err != nil {
		file.Close()
		return Validation{}, fmt.Errorf("write temporary config: %w", err)
	}
	if err := file.Close(); err != nil {
		return Validation{}, fmt.Errorf("close temporary config: %w", err)
	}

	checkOutput, err := exec.CommandContext(ctx, executable, "check", "-c", path).CombinedOutput()
	if err != nil {
		return Validation{}, fmt.Errorf("sing-box check: %w: %s", err, strings.TrimSpace(string(checkOutput)))
	}
	return Validation{
		VersionOutput: strings.TrimSpace(string(versionOutput)), SHA256: hash,
		CheckOutput: strings.TrimSpace(string(checkOutput)),
	}, nil
}

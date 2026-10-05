package core

import (
	"errors"
	"os"
	"path/filepath"
)

// Locate finds the sing-box core executable using standard candidate paths.
// Priority:
// 1. Environment variable WINROUTER_CORE_PATH (if set and exists)
// 2. <exe_dir>/sing-box.exe (user supplied core next to main executable)
// 3. <exe_dir>/resources/core/sing-box.exe (bundled core next to main executable)
// 4. ./sing-box.exe (working directory)
// 5. ./resources/core/sing-box.exe (working directory resources)
func Locate() (string, error) {
	if custom := os.Getenv("WINROUTER_CORE_PATH"); custom != "" {
		if info, err := os.Stat(custom); err == nil && !info.IsDir() {
			return filepath.Abs(custom)
		}
	}

	executable, err := os.Executable()
	var candidates []string
	if err == nil {
		exeDir := filepath.Dir(executable)
		candidates = append(candidates,
			filepath.Join(exeDir, "sing-box.exe"),
			filepath.Join(exeDir, "resources", "core", "sing-box.exe"),
		)
	}
	candidates = append(candidates,
		"sing-box.exe",
		filepath.Join("resources", "core", "sing-box.exe"),
	)

	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absolute); err == nil && !info.IsDir() {
			return absolute, nil
		}
	}

	return "", errors.New("locate sing-box core executable (looked in exe directory and resources/core)")
}

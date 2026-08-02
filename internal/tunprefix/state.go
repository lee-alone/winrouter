package tunprefix

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
)

type State struct {
	Prefix string `json:"prefix"`
}

func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read TUN prefix state: %w", err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, fmt.Errorf("decode TUN prefix state: %w", err)
	}
	if state.Prefix != "" {
		prefix, err := netip.ParsePrefix(state.Prefix)
		if err != nil || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() || prefix.Bits() > 30 {
			return State{}, fmt.Errorf("invalid persisted TUN prefix %q", state.Prefix)
		}
		state.Prefix = prefix.Masked().String()
	}
	return state, nil
}

func SaveState(path, prefixValue string) error {
	prefix, err := netip.ParsePrefix(prefixValue)
	if err != nil || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() || prefix.Bits() > 30 {
		return fmt.Errorf("invalid TUN prefix %q", prefixValue)
	}
	prefix = prefix.Masked()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create TUN state directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".tun-prefix-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary TUN state: %w", err)
	}
	temporaryPath := file.Name()
	defer os.Remove(temporaryPath)
	if err := file.Chmod(0o600); err != nil {
		file.Close()
		return fmt.Errorf("restrict TUN state: %w", err)
	}
	data, err := json.MarshalIndent(State{Prefix: prefix.String()}, "", "  ")
	if err == nil {
		_, err = file.Write(append(data, '\n'))
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write TUN state: %w", err)
	}
	if err := replaceFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace TUN state: %w", err)
	}
	return nil
}

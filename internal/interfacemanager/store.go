package interfacemanager

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
)

func LoadState(path string) (State, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{SchemaVersion: StateSchemaVersion}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read interface state: %w", err)
	}
	state, migrated, err := decodeAndMigrateState(data)
	if err != nil {
		return State{}, err
	}
	if migrated {
		if err := SaveState(path, state); err != nil {
			return State{}, fmt.Errorf("commit migrated interface state: %w", err)
		}
	}
	return state, nil
}

func decodeAndMigrateState(data []byte) (State, bool, error) {
	var envelope struct {
		SchemaVersion int `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return State{}, false, fmt.Errorf("decode interface state: %w", err)
	}
	if envelope.SchemaVersion != legacyStateSchemaVersion && envelope.SchemaVersion != StateSchemaVersion {
		return State{}, false, fmt.Errorf("unsupported interface state schema %d", envelope.SchemaVersion)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return State{}, false, fmt.Errorf("decode interface state schema %d: %w", envelope.SchemaVersion, err)
	}
	migrated := state.SchemaVersion != StateSchemaVersion
	if migrated {
		state.SchemaVersion = StateSchemaVersion
	}
	if err := normalizeAndValidateState(&state); err != nil {
		return State{}, false, fmt.Errorf("validate interface state schema %d: %w", envelope.SchemaVersion, err)
	}
	return state, migrated, nil
}

func normalizeAndValidateState(state *State) error {
	if state.SchemaVersion != StateSchemaVersion {
		return fmt.Errorf("unsupported interface state schema %d", state.SchemaVersion)
	}
	state.InterfaceA.GUID = strings.TrimSpace(state.InterfaceA.GUID)
	state.InterfaceB.GUID = strings.TrimSpace(state.InterfaceB.GUID)
	state.TUNPrefix = strings.TrimSpace(state.TUNPrefix)
	if state.TUNPrefix != "" {
		prefix, err := netip.ParsePrefix(state.TUNPrefix)
		if err != nil || !prefix.Addr().Is4() || prefix.Bits() != 30 || prefix != prefix.Masked() {
			return fmt.Errorf("invalid TUN prefix %q", state.TUNPrefix)
		}
	}
	return nil
}

func SaveState(path string, state State) error {
	if path == "" {
		return errors.New("interface state path is empty")
	}
	state.SchemaVersion = StateSchemaVersion
	if err := normalizeAndValidateState(&state); err != nil {
		return fmt.Errorf("validate interface state: %w", err)
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode interface state: %w", err)
	}
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create interface state directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".interfaces-*.tmp")
	if err != nil {
		return fmt.Errorf("create interface state temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(append(data, '\n'))
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write interface state: %w", err)
	}
	if err := replaceStateFile(temporaryPath, path); err != nil {
		return fmt.Errorf("replace interface state: %w", err)
	}
	return nil
}

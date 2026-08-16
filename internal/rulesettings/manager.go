package rulesettings

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
)

const SchemaVersion = 1

var ruleIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9:_-]{0,127}$`)

type Rule struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Value   string `json:"value"`
	Action  string `json:"action"`
	Enabled bool   `json:"enabled"`
}

type Settings struct {
	SchemaVersion        int      `json:"schema_version"`
	Initialized          bool     `json:"initialized"`
	LegacyRemoteMigrated bool     `json:"legacy_remote_migrated,omitempty"`
	DefaultOutbound      string   `json:"default_outbound"`
	Rules                []Rule   `json:"rules"`
	RuleOrder            []string `json:"rule_order"`
}

func Defaults() Settings {
	return Settings{SchemaVersion: SchemaVersion, DefaultOutbound: "b", Rules: []Rule{}, RuleOrder: []string{}}
}

type Manager struct {
	mu       sync.Mutex
	path     string
	settings Settings
}

func New(path string) (*Manager, error) {
	if strings.TrimSpace(path) == "" {
		return nil, errors.New("rule settings path is required")
	}
	m := &Manager{path: path, settings: Defaults()}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read rule settings: %w", err)
	}
	var stored Settings
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("decode rule settings: %w", err)
	}
	if err := Validate(stored); err != nil {
		return nil, err
	}
	m.settings = clone(stored)
	return m, nil
}

func (m *Manager) Get() Settings {
	m.mu.Lock()
	defer m.mu.Unlock()
	return clone(m.settings)
}

func (m *Manager) Configure(value Settings) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.configureLocked(value)
}

func (m *Manager) MigrateLegacyRemote(imported []Rule) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.settings.LegacyRemoteMigrated {
		return clone(m.settings), nil
	}
	if len(imported) == 0 {
		return Settings{}, errors.New("verified legacy remote rules are required")
	}
	if len(m.settings.Rules)+len(imported) > 200 {
		return Settings{}, fmt.Errorf("legacy remote migration would exceed the limit of 200 rules (%d existing + %d imported)", len(m.settings.Rules), len(imported))
	}
	next := clone(m.settings)
	next.Rules = append(next.Rules, imported...)
	for _, rule := range imported {
		next.RuleOrder = append(next.RuleOrder, rule.ID)
	}
	next.LegacyRemoteMigrated = true
	return m.configureLocked(next)
}

func (m *Manager) configureLocked(value Settings) (Settings, error) {
	value.SchemaVersion = SchemaVersion
	value.Initialized = true
	if err := Validate(value); err != nil {
		return Settings{}, err
	}
	value = clone(value)
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return Settings{}, err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return Settings{}, err
	}
	file, err := os.CreateTemp(filepath.Dir(m.path), ".rule-settings-*.tmp")
	if err != nil {
		return Settings{}, err
	}
	temporary := file.Name()
	defer os.Remove(temporary)
	_ = file.Chmod(0o600)
	if _, err = file.Write(append(data, '\n')); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return Settings{}, err
	}
	if err := replaceFile(temporary, m.path); err != nil {
		return Settings{}, err
	}
	m.settings = value
	return clone(value), nil
}

func Validate(value Settings) error {
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported rule settings schema_version %d", value.SchemaVersion)
	}
	if value.DefaultOutbound != "a" && value.DefaultOutbound != "b" {
		return errors.New("default_outbound must be a or b")
	}
	if len(value.Rules) > 200 {
		return errors.New("rules exceed the limit of 200")
	}
	ids := make(map[string]struct{}, len(value.Rules))
	matches := make(map[string]string, len(value.Rules))
	for index, rule := range value.Rules {
		if !ruleIDPattern.MatchString(rule.ID) {
			return fmt.Errorf("rule %d has an invalid id", index+1)
		}
		if _, exists := ids[rule.ID]; exists {
			return fmt.Errorf("duplicate rule id %q", rule.ID)
		}
		ids[rule.ID] = struct{}{}
		if strings.TrimSpace(rule.Name) == "" || len([]rune(rule.Name)) > 80 || strings.TrimSpace(rule.Value) == "" {
			return fmt.Errorf("rule %q requires a name and value", rule.ID)
		}
		switch rule.Type {
		case "domain", "ip", "process-name", "process-path":
		default:
			return fmt.Errorf("rule %q has unsupported type %q", rule.ID, rule.Type)
		}
		switch rule.Action {
		case "a", "b", "final", "reject":
		default:
			return fmt.Errorf("rule %q has unsupported action %q", rule.ID, rule.Action)
		}
		matchKey, err := normalizedMatchKey(rule)
		if err != nil {
			return fmt.Errorf("rule %q: %w", rule.ID, err)
		}
		if rule.Enabled {
			if previous, exists := matches[matchKey]; exists && previous != rule.Action {
				return fmt.Errorf("rule %q has conflicting actions for %q", rule.ID, rule.Value)
			}
			matches[matchKey] = rule.Action
		}
	}
	if len(value.RuleOrder) > 1000 {
		return errors.New("rule order exceeds the limit of 1000")
	}
	order := make(map[string]struct{}, len(value.RuleOrder))
	for _, key := range value.RuleOrder {
		if !ruleIDPattern.MatchString(key) {
			return fmt.Errorf("invalid rule order key %q", key)
		}
		if _, exists := order[key]; exists {
			return fmt.Errorf("duplicate rule order key %q", key)
		}
		order[key] = struct{}{}
	}
	return nil
}

func normalizedMatchKey(rule Rule) (string, error) {
	value := strings.TrimSpace(rule.Value)
	switch rule.Type {
	case "domain":
		value = strings.ToLower(strings.Trim(value, "."))
		if value == "" || strings.ContainsAny(value, " /\\") {
			return "", errors.New("invalid domain suffix")
		}
	case "ip":
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return "", errors.New("invalid IP prefix")
		}
		value = prefix.Masked().String()
	case "process-name":
		value = strings.ToLower(value)
		if strings.ContainsAny(value, `/\\`) || !strings.HasSuffix(value, ".exe") {
			return "", errors.New("invalid process name")
		}
	case "process-path":
		value = strings.ToLower(value)
	}
	return rule.Type + ":" + value, nil
}

func clone(value Settings) Settings {
	// Keep empty collections non-nil so Wails exposes [] instead of null.
	value.Rules = append([]Rule{}, value.Rules...)
	value.RuleOrder = append([]string{}, value.RuleOrder...)
	return value
}

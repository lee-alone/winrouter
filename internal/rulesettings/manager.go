package rulesettings

import (
	"bytes"
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

const SchemaVersion = 2

var ruleIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9:_-]{0,127}$`)

type Rule struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Values  []string `json:"values"`
	Action  string   `json:"action"`
	Enabled bool     `json:"enabled"`
}

type Settings struct {
	SchemaVersion   int      `json:"schema_version"`
	Initialized     bool     `json:"initialized"`
	DefaultOutbound string   `json:"default_outbound"`
	Rules           []Rule   `json:"rules"`
	RuleOrder       []string `json:"rule_order"`
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
	var envelope struct {
		Rules []struct {
			Value json.RawMessage `json:"value"`
		} `json:"rules"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("decode rule settings: %w", err)
	}
	for _, rule := range envelope.Rules {
		if len(rule.Value) > 0 && string(rule.Value) != "null" {
			return nil, errors.New("legacy rule settings format is unsupported; use schema_version 2 with values")
		}
	}
	var stored Settings
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&stored); err != nil {
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

func (m *Manager) configureLocked(value Settings) (Settings, error) {
	value.SchemaVersion = SchemaVersion
	value.Initialized = true
	for ruleIndex := range value.Rules {
		for valueIndex, raw := range value.Rules[ruleIndex].Values {
			normalized, err := normalizeValue(value.Rules[ruleIndex].Type, raw)
			if err != nil {
				return Settings{}, fmt.Errorf("rule %q: %w", value.Rules[ruleIndex].ID, err)
			}
			value.Rules[ruleIndex].Values[valueIndex] = normalized
		}
	}
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
	if value.DefaultOutbound != "a" && value.DefaultOutbound != "b" && value.DefaultOutbound != "c" {
		return errors.New("default_outbound must be a, b, or c")
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
		if strings.TrimSpace(rule.Name) == "" || len([]rune(rule.Name)) > 80 || len(rule.Values) == 0 {
			return fmt.Errorf("rule %q requires a name and at least one value", rule.ID)
		}
		switch rule.Type {
		case "domain", "domain-suffix", "ip", "process-name", "process-path":
		default:
			return fmt.Errorf("rule %q has unsupported type %q", rule.ID, rule.Type)
		}
		switch rule.Action {
		case "a", "b", "c", "final", "reject":
		default:
			return fmt.Errorf("rule %q has unsupported action %q", rule.ID, rule.Action)
		}
		seenValues := make(map[string]struct{}, len(rule.Values))
		for _, value := range rule.Values {
			matchKey, err := normalizedMatchKey(rule.Type, value)
			if err != nil {
				return fmt.Errorf("rule %q: %w", rule.ID, err)
			}
			if _, exists := seenValues[matchKey]; exists {
				return fmt.Errorf("rule %q contains duplicate value %q", rule.ID, value)
			}
			seenValues[matchKey] = struct{}{}
			if rule.Enabled {
				if previous, exists := matches[matchKey]; exists && previous != rule.Action {
					return fmt.Errorf("rule %q has conflicting actions for %q", rule.ID, value)
				}
				matches[matchKey] = rule.Action
			}
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

func normalizedMatchKey(ruleType, raw string) (string, error) {
	value, err := normalizeValue(ruleType, raw)
	if err != nil {
		return "", err
	}
	return ruleType + ":" + value, nil
}

func normalizeValue(ruleType, raw string) (string, error) {
	value := strings.TrimSpace(raw)
	switch ruleType {
	case "domain":
		fallthrough
	case "domain-suffix":
		value = strings.ToLower(strings.Trim(value, "."))
		if value == "" || strings.ContainsAny(value, " /\\") {
			return "", errors.New("invalid domain suffix")
		}
	case "ip":
		if addr, err := netip.ParseAddr(value); err == nil {
			if addr.IsUnspecified() || addr.IsMulticast() {
				return "", errors.New("invalid IP or CIDR")
			}
			value = netip.PrefixFrom(addr, addr.BitLen()).String()
		} else if prefix, err := netip.ParsePrefix(value); err == nil {
			if prefix.Addr().IsUnspecified() || prefix.Addr().IsMulticast() {
				return "", errors.New("invalid IP or CIDR")
			}
			value = prefix.Masked().String()
		} else {
			return "", errors.New("invalid IP or CIDR")
		}
	case "process-name":
		value = strings.ToLower(value)
		if strings.ContainsAny(value, `/\\`) || !strings.HasSuffix(value, ".exe") {
			return "", errors.New("invalid process name")
		}
	case "process-path":
		value = strings.ToLower(value)
		if value == "" {
			return "", errors.New("invalid process path")
		}
	}
	return value, nil
}

func clone(value Settings) Settings {
	// Keep empty collections non-nil so Wails exposes [] instead of null.
	value.Rules = append([]Rule{}, value.Rules...)
	for i := range value.Rules {
		value.Rules[i].Values = append([]string{}, value.Rules[i].Values...)
	}
	value.RuleOrder = append([]string{}, value.RuleOrder...)
	return value
}

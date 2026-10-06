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

const SchemaVersion = 3

func defaultPrivateLANRule() Rule {
	return DefaultPrivateLANRule()
}

var ruleIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9:_!-]{0,127}$`)

type Rule struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Type    string   `json:"type"`
	Values  []string `json:"values"`
	Action  string   `json:"action"`
	Enabled bool     `json:"enabled"`
}

type Settings struct {
	SchemaVersion      int                `json:"schema_version"`
	Initialized        bool               `json:"initialized"`
	ActiveMode         string             `json:"active_mode,omitempty"`
	Profiles           map[string]Profile `json:"profiles,omitempty"`
	DefaultOutbound    string             `json:"default_outbound,omitempty"`
	RuleUpdateOutbound string             `json:"rule_update_outbound,omitempty"`
	Rules              []Rule             `json:"rules,omitempty"`
	RuleOrder          []string           `json:"rule_order,omitempty"`
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
			return nil, errors.New("legacy rule settings format is unsupported; use schema_version 2/3 with values")
		}
	}

	var stored Settings
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&stored); err != nil {
		return nil, fmt.Errorf("decode rule settings: %w", err)
	}

	if stored.RuleUpdateOutbound == "" {
		stored.RuleUpdateOutbound = "auto"
	}
	ensureDefaultPrivateLAN(&stored)

	// Schema migration: If Profiles is empty (e.g. from Schema 2), migrate legacy fields into dual profile
	if len(stored.Profiles) == 0 {
		dualProfile := DefaultDualProfile()
		dualProfile.Rules = append([]Rule(nil), stored.Rules...)
		dualProfile.RuleOrder = append([]string(nil), stored.RuleOrder...)
		if stored.DefaultOutbound != "" {
			dualProfile.DefaultOutbound = stored.DefaultOutbound
		}
		if stored.RuleUpdateOutbound != "" {
			dualProfile.RuleUpdateOutbound = stored.RuleUpdateOutbound
		}
		singleProfile := DefaultSingleProfile()
		// Single profile also inherits custom rules mapped safely
		for _, r := range stored.Rules {
			if r.ID == DefaultPrivateLANRuleID {
				continue
			}
			singleRule := r
			if singleRule.Action == "b" {
				singleRule.Action = "a"
			}
			singleProfile.Rules = append(singleProfile.Rules, singleRule)
		}
		stored.Profiles = map[string]Profile{
			ModeSingle: singleProfile,
			ModeDual:   dualProfile,
		}
		stored.SchemaVersion = SchemaVersion
		stored.ActiveMode = ModeDual
	} else {
		if _, ok := stored.Profiles[ModeSingle]; !ok {
			stored.Profiles[ModeSingle] = DefaultSingleProfile()
		}
		if _, ok := stored.Profiles[ModeDual]; !ok {
			stored.Profiles[ModeDual] = DefaultDualProfile()
		}
		for mode, p := range stored.Profiles {
			ensureProfileDefaultLAN(&p)
			stored.Profiles[mode] = p
		}
	}

	if stored.ActiveMode == "" {
		stored.ActiveMode = ModeDual
	}
	syncLegacyFields(&stored, stored.ActiveMode)

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

func (m *Manager) GetProfile(mode string) Profile {
	m.mu.Lock()
	defer m.mu.Unlock()
	mode = normalizeMode(mode)
	if m.settings.Profiles != nil {
		if p, ok := m.settings.Profiles[mode]; ok {
			return p.Clone()
		}
	}
	if mode == ModeSingle {
		return DefaultSingleProfile()
	}
	return DefaultDualProfile()
}

func (m *Manager) SetProfile(mode string, profile Profile) (Profile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	mode = normalizeMode(mode)

	for ruleIndex := range profile.Rules {
		for valueIndex, raw := range profile.Rules[ruleIndex].Values {
			normalized, err := normalizeValue(profile.Rules[ruleIndex].Type, raw)
			if err != nil {
				return Profile{}, fmt.Errorf("rule %q: %w", profile.Rules[ruleIndex].ID, err)
			}
			profile.Rules[ruleIndex].Values[valueIndex] = normalized
		}
	}

	ensureProfileDefaultLAN(&profile)
	if err := profile.Validate(mode); err != nil {
		return Profile{}, err
	}

	if m.settings.Profiles == nil {
		m.settings.Profiles = make(map[string]Profile)
	}
	m.settings.Profiles[mode] = profile.Clone()
	m.settings.ActiveMode = mode
	m.settings.Initialized = true
	m.settings.SchemaVersion = SchemaVersion
	syncLegacyFields(&m.settings, mode)

	if err := m.saveLocked(m.settings); err != nil {
		return Profile{}, err
	}
	return profile.Clone(), nil
}

func (m *Manager) Configure(value Settings) (Settings, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.configureLocked(value)
}

func (m *Manager) configureLocked(value Settings) (Settings, error) {
	value.SchemaVersion = SchemaVersion
	value.Initialized = true
	if value.RuleUpdateOutbound == "" {
		value.RuleUpdateOutbound = "auto"
	}
	if value.ActiveMode == "" {
		value.ActiveMode = ModeDual
	}
	targetMode := normalizeMode(value.ActiveMode)

	// If top-level rules were provided, normalize them first
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

	if value.Profiles == nil {
		value.Profiles = make(map[string]Profile)
	}

	// Always sync top-level rules to the target mode Profile
	p, ok := value.Profiles[targetMode]
	if !ok {
		if targetMode == ModeSingle {
			p = DefaultSingleProfile()
		} else {
			p = DefaultDualProfile()
		}
	}
	if value.DefaultOutbound != "" {
		p.DefaultOutbound = value.DefaultOutbound
	}
	if value.RuleUpdateOutbound != "" {
		p.RuleUpdateOutbound = value.RuleUpdateOutbound
	}
	p.Rules = append([]Rule(nil), value.Rules...)
	p.RuleOrder = append([]string(nil), value.RuleOrder...)
	value.Profiles[targetMode] = p

	// Ensure the other profile exists
	otherMode := ModeSingle
	if targetMode == ModeSingle {
		otherMode = ModeDual
	}
	if _, exists := value.Profiles[otherMode]; !exists {
		if m.settings.Profiles != nil {
			if existing, ok := m.settings.Profiles[otherMode]; ok {
				value.Profiles[otherMode] = existing.Clone()
			}
		}
		if _, exists := value.Profiles[otherMode]; !exists {
			if otherMode == ModeSingle {
				value.Profiles[otherMode] = DefaultSingleProfile()
			} else {
				value.Profiles[otherMode] = DefaultDualProfile()
			}
		}
	}

	syncLegacyFields(&value, targetMode)

	if err := m.saveLocked(value); err != nil {
		return Settings{}, err
	}
	m.settings = clone(value)
	return clone(value), nil
}

func (m *Manager) saveLocked(value Settings) error {
	value = clone(value)
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(m.path), ".rule-settings-*.tmp")
	if err != nil {
		return err
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
		return err
	}
	if err := replaceFile(temporary, m.path); err != nil {
		return err
	}
	return nil
}

func Validate(value Settings) error {
	if value.SchemaVersion != 2 && value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported rule settings schema_version %d", value.SchemaVersion)
	}
	if value.DefaultOutbound != "a" && value.DefaultOutbound != "b" && value.DefaultOutbound != "c" {
		return errors.New("default_outbound must be a, b, or c")
	}
	if value.RuleUpdateOutbound != "" && value.RuleUpdateOutbound != "auto" && value.RuleUpdateOutbound != "a" && value.RuleUpdateOutbound != "b" && value.RuleUpdateOutbound != "c" {
		return errors.New("rule_update_outbound must be auto, a, b, or c")
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
	out := value
	if value.Profiles != nil {
		out.Profiles = make(map[string]Profile, len(value.Profiles))
		for k, v := range value.Profiles {
			out.Profiles[k] = v.Clone()
		}
	}
	out.Rules = append([]Rule{}, value.Rules...)
	for i := range out.Rules {
		out.Rules[i].Values = append([]string{}, value.Rules[i].Values...)
	}
	out.RuleOrder = append([]string{}, value.RuleOrder...)
	return out
}

func isLegacyPrivateLANValues(values []string) bool {
	if len(values) == 2 {
		return (values[0] == "192.168.0.0/16" && values[1] == "10.110.0.0/16") ||
			(values[0] == "10.110.0.0/16" && values[1] == "192.168.0.0/16")
	}
	return false
}

func ensureDefaultPrivateLAN(s *Settings) {
	if len(s.Rules) == 0 && !s.Initialized {
		s.Rules = Defaults().Rules
		s.RuleOrder = Defaults().RuleOrder
		return
	}
	hasPrivateLAN := false
	for i, rule := range s.Rules {
		if rule.ID == DefaultPrivateLANRuleID {
			hasPrivateLAN = true
			if isLegacyPrivateLANValues(rule.Values) {
				s.Rules[i].Values = defaultPrivateLANRule().Values
			}
			break
		}
	}
	if !hasPrivateLAN {
		s.Rules = append(s.Rules, defaultPrivateLANRule())
	}
	newOrder := make([]string, 0, len(s.RuleOrder)+1)
	newOrder = append(newOrder, DefaultPrivateLANRuleID)
	for _, key := range s.RuleOrder {
		if key != DefaultPrivateLANRuleID {
			newOrder = append(newOrder, key)
		}
	}
	s.RuleOrder = newOrder
}

func ensureProfileDefaultLAN(p *Profile) {
	hasPrivateLAN := false
	for i, rule := range p.Rules {
		if rule.ID == DefaultPrivateLANRuleID {
			hasPrivateLAN = true
			if isLegacyPrivateLANValues(rule.Values) {
				p.Rules[i].Values = defaultPrivateLANRule().Values
			}
			break
		}
	}
	if !hasPrivateLAN {
		p.Rules = append(p.Rules, defaultPrivateLANRule())
	}
	newOrder := make([]string, 0, len(p.RuleOrder)+1)
	newOrder = append(newOrder, DefaultPrivateLANRuleID)
	for _, key := range p.RuleOrder {
		if key != DefaultPrivateLANRuleID {
			newOrder = append(newOrder, key)
		}
	}
	p.RuleOrder = newOrder
}

func syncLegacyFields(s *Settings, mode string) {
	mode = normalizeMode(mode)
	if s.Profiles != nil {
		if p, ok := s.Profiles[mode]; ok {
			s.DefaultOutbound = p.DefaultOutbound
			s.RuleUpdateOutbound = p.RuleUpdateOutbound
			s.Rules = append([]Rule(nil), p.Rules...)
			s.RuleOrder = append([]string(nil), p.RuleOrder...)
			return
		}
		if p, ok := s.Profiles[ModeDual]; ok {
			s.DefaultOutbound = p.DefaultOutbound
			s.RuleUpdateOutbound = p.RuleUpdateOutbound
			s.Rules = append([]Rule(nil), p.Rules...)
			s.RuleOrder = append([]string(nil), p.RuleOrder...)
		}
	}
}

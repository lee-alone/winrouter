package rulesettings

import (
	"errors"
	"fmt"
	"strings"
)

// Mode constants representing network adapter modes.
const (
	ModeSingle = "single"
	ModeDual   = "dual"
)

// Profile represents a complete rule configuration for a specific network mode (single or dual).
type Profile struct {
	DefaultOutbound    string            `json:"default_outbound"`
	RuleUpdateOutbound string            `json:"rule_update_outbound,omitempty"`
	Rules              []Rule            `json:"rules"`
	RuleOrder          []string          `json:"rule_order"`
	SRSActions         map[string]string `json:"srs_actions,omitempty"`
	SRSEnabled         map[string]bool   `json:"srs_enabled,omitempty"`
}

// Clone returns a deep copy of the Profile.
func (p Profile) Clone() Profile {
	out := Profile{
		DefaultOutbound:    p.DefaultOutbound,
		RuleUpdateOutbound: p.RuleUpdateOutbound,
		Rules:              make([]Rule, len(p.Rules)),
		RuleOrder:          append([]string(nil), p.RuleOrder...),
		SRSActions:         make(map[string]string, len(p.SRSActions)),
		SRSEnabled:         make(map[string]bool, len(p.SRSEnabled)),
	}
	for i, r := range p.Rules {
		out.Rules[i] = Rule{
			ID:      r.ID,
			Name:    r.Name,
			Type:    r.Type,
			Values:  append([]string(nil), r.Values...),
			Action:  r.Action,
			Enabled: r.Enabled,
		}
	}
	for k, v := range p.SRSActions {
		out.SRSActions[k] = v
	}
	for k, v := range p.SRSEnabled {
		out.SRSEnabled[k] = v
	}
	return out
}

// Validate validates the profile against rules for the specified mode.
func (p Profile) Validate(mode string) error {
	mode = normalizeMode(mode)
	if mode == ModeSingle {
		if p.DefaultOutbound != "a" && p.DefaultOutbound != "c" {
			return fmt.Errorf("single-NIC default_outbound must be 'a' (direct) or 'c' (proxy), got %q", p.DefaultOutbound)
		}
	} else {
		if p.DefaultOutbound != "a" && p.DefaultOutbound != "b" && p.DefaultOutbound != "c" {
			return fmt.Errorf("dual-NIC default_outbound must be 'a', 'b', or 'c', got %q", p.DefaultOutbound)
		}
	}
	if p.RuleUpdateOutbound != "" && p.RuleUpdateOutbound != "auto" && p.RuleUpdateOutbound != "a" && p.RuleUpdateOutbound != "b" && p.RuleUpdateOutbound != "c" {
		return errors.New("rule_update_outbound must be auto, a, b, or c")
	}
	if len(p.Rules) > 200 {
		return errors.New("rules exceed the limit of 200")
	}

	ids := make(map[string]struct{}, len(p.Rules))
	matches := make(map[string]string, len(p.Rules))
	for index, rule := range p.Rules {
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

		if mode == ModeSingle && rule.Action == "b" {
			return fmt.Errorf("rule %q targets outbound 'b' which is not supported in single-NIC mode", rule.ID)
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

	if len(p.RuleOrder) > 1000 {
		return errors.New("rule order exceeds the limit of 1000")
	}
	order := make(map[string]struct{}, len(p.RuleOrder))
	for _, key := range p.RuleOrder {
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

func normalizeMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == ModeSingle {
		return ModeSingle
	}
	return ModeDual
}

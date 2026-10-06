package app

import (
	"errors"
	"strings"

	"winrouter/internal/rulesettings"
)

// GetRuleProfile returns the rule configuration profile for the specified mode ("single" or "dual").
func (a *App) GetRuleProfile(mode string) (rulesettings.Profile, error) {
	a.mu.RLock()
	manager, loadErr := a.ruleSettings, a.ruleSettingsError
	a.mu.RUnlock()
	if loadErr != nil {
		return rulesettings.Profile{}, loadErr
	}
	if manager == nil {
		return rulesettings.Profile{}, errors.New("rule settings manager is not ready")
	}
	return manager.GetProfile(mode), nil
}

// SetRuleProfile saves the rule configuration profile for the specified mode ("single" or "dual").
func (a *App) SetRuleProfile(mode string, profile rulesettings.Profile) (rulesettings.Profile, error) {
	a.mu.RLock()
	manager, loadErr := a.ruleSettings, a.ruleSettingsError
	a.mu.RUnlock()
	if loadErr != nil {
		return rulesettings.Profile{}, loadErr
	}
	if manager == nil {
		return rulesettings.Profile{}, errors.New("rule settings manager is not ready")
	}
	return manager.SetProfile(mode, profile)
}

// resolveMode returns the effective network mode, defaulting to interface manager's current mode.
func (a *App) resolveMode(mode string) string {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == rulesettings.ModeSingle || mode == rulesettings.ModeDual {
		return mode
	}
	if manager, err := a.getInterfaceManager(); err == nil {
		return manager.Mode()
	}
	return rulesettings.ModeDual
}

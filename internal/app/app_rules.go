package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"time"

	"winrouter/internal/config"
	"winrouter/internal/observability"
	"winrouter/internal/processrules"
	"winrouter/internal/rulesettings"
	"winrouter/internal/srssets"
)

func (a *App) PreviewCoreRules(input config.MVPConfig) ([]config.RulePreview, error) {
	if input.Mode == "" {
		if manager, err := a.getInterfaceManager(); err == nil {
			input.Mode = manager.Mode()
		}
	}
	var err error
	input, err = a.withRuleSettings(input)
	if err != nil {
		return nil, err
	}
	input, err = a.withDNSSettings(input)
	if err != nil {
		return nil, err
	}
	input, err = a.withSelectedProxy(input)
	if err != nil {
		return nil, err
	}
	input, err = a.withSRSRuleSets(input)
	if err != nil {
		return nil, err
	}
	return config.PreviewMVPRules(input)
}

func (a *App) InspectProcessRules(input config.MVPConfig) ([]processrules.Status, error) {
	identities := make([]processrules.Identity, 0)
	for _, rule := range input.CustomRules {
		if rule.Type == "process-name" || rule.Type == "process-path" {
			identities = append(identities, processrules.Identity{Type: rule.Type, Value: rule.Value})
		}
	}
	return processrules.Inspect(identities)
}

func (a *App) GetRuleSettings() (rulesettings.Settings, error) {
	a.mu.RLock()
	manager, loadErr := a.ruleSettings, a.ruleSettingsError
	a.mu.RUnlock()
	if loadErr != nil {
		return rulesettings.Settings{}, loadErr
	}
	if manager == nil {
		return rulesettings.Settings{}, errors.New("rule settings manager is not ready")
	}
	return manager.Get(), nil
}

func (a *App) SetRuleSettings(settings rulesettings.Settings) (rulesettings.Settings, error) {
	a.mu.RLock()
	manager, loadErr := a.ruleSettings, a.ruleSettingsError
	a.mu.RUnlock()
	if loadErr != nil {
		return rulesettings.Settings{}, loadErr
	}
	if manager == nil {
		return rulesettings.Settings{}, errors.New("rule settings manager is not ready")
	}
	return manager.Configure(settings)
}

func (a *App) withRuleSettings(input config.MVPConfig) (config.MVPConfig, error) {
	settings, err := a.GetRuleSettings()
	if err != nil {
		return input, err
	}
	return applyRuleSettings(input, settings), nil
}

func applyRuleSettings(input config.MVPConfig, settings rulesettings.Settings) config.MVPConfig {
	if !settings.Initialized {
		return input
	}
	input.DefaultOutbound = settings.DefaultOutbound
	input.RuleOrder = append([]string(nil), settings.RuleOrder...)
	input.CustomRules = make([]config.MVPCustomRule, 0, len(settings.Rules))
	for _, rule := range settings.Rules {
		if !rule.Enabled {
			continue
		}
		for _, value := range rule.Values {
			input.CustomRules = append(input.CustomRules, config.MVPCustomRule{ID: rule.ID, Name: rule.Name, Type: rule.Type, Value: value, Action: rule.Action})
		}
	}
	return input
}

func (a *App) GetSRSPresets() []srssets.Preset { return srssets.Presets() }

func (a *App) ListSRSSources() ([]srssets.Source, error) {
	manager, err := a.getSRSManager()
	if err != nil {
		return nil, err
	}
	return manager.List(), nil
}

func (a *App) ConfigureSRSSource(source srssets.Source) (srssets.Source, error) {
	manager, err := a.getSRSManager()
	if err != nil {
		return srssets.Source{}, err
	}
	configured, err := manager.Configure(source)
	if err == nil {
		a.syncAllRuleSetMetadata()
	}
	return configured, err
}

func (a *App) RefreshSRSSource(id string) (srssets.Source, error) {
	manager, err := a.getSRSManager()
	if err != nil {
		return srssets.Source{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 35*time.Second)
	defer cancel()
	client := a.ruleUpdateHTTPClient()
	updated, err := manager.UpdateWithClient(ctx, id, client)
	a.syncAllRuleSetMetadata()
	return updated, err
}

func (a *App) ruleUpdateHTTPClient() *http.Client {
	outbound := "auto"
	a.mu.RLock()
	rs := a.ruleSettings
	a.mu.RUnlock()
	if rs != nil {
		settings := rs.Get()
		if settings.RuleUpdateOutbound != "" {
			outbound = settings.RuleUpdateOutbound
		}
	}
	baseTransport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSHandshakeTimeout: 10 * time.Second,
	}
	switch outbound {
	case "a":
		if src, err := a.interfaceASourceIPv4(); err == nil && src != "" {
			dialer := &net.Dialer{
				LocalAddr: &net.TCPAddr{IP: net.ParseIP(src)},
				Timeout:   15 * time.Second,
			}
			baseTransport.DialContext = dialer.DialContext
		}
	case "b":
		if src, err := a.interfaceBSourceIPv4(); err == nil && src != "" {
			dialer := &net.Dialer{
				LocalAddr: &net.TCPAddr{IP: net.ParseIP(src)},
				Timeout:   15 * time.Second,
			}
			baseTransport.DialContext = dialer.DialContext
		}
	}
	return &http.Client{
		Transport: baseTransport,
		Timeout:   30 * time.Second,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return errors.New("stopped after 5 redirects")
			}
			return nil
		},
	}
}

func (a *App) DeleteSRSSource(id string) error {
	manager, err := a.getSRSManager()
	if err != nil {
		return err
	}
	if err := manager.Delete(id); err != nil {
		return err
	}
	a.syncAllRuleSetMetadata()
	return nil
}

func (a *App) getSRSManager() (*srssets.Manager, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.srsSetError != nil {
		return nil, a.srsSetError
	}
	if a.srsSets == nil {
		return nil, errors.New("SRS source manager is not ready")
	}
	return a.srsSets, nil
}

func (a *App) withSRSRuleSets(input config.MVPConfig) (config.MVPConfig, error) {
	manager, err := a.getSRSManager()
	if err != nil {
		return input, err
	}
	active, err := manager.Active()
	if err != nil {
		return input, fmt.Errorf("load verified SRS rule-sets: %w", err)
	}
	input.RuleSets = nil
	for _, item := range active {
		input.RuleSets = append(input.RuleSets, config.MVPRuleSet{Tag: item.Tag, Kind: item.Kind, Action: item.Action, Path: item.Path})
	}
	return input, nil
}

func (a *App) syncAllRuleSetMetadata() {
	items := make([]observability.RuleSetMetadata, 0)
	if manager, err := a.getSRSManager(); err == nil {
		for _, source := range manager.List() {
			result := "configured"
			if source.AppliedSHA256 != "" {
				result = "validated-and-cached"
			}
			if source.LastError != "" {
				result = "update-failed-last-valid-retained"
			}
			items = append(items, observability.RuleSetMetadata{Name: source.Name, SHA256: source.AppliedSHA256, Source: source.URL, Size: source.Size, LoadResult: result})
		}
	}
	a.observations.SetRuleSets(items)
}

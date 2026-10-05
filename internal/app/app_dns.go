package app

import (
	"context"
	"errors"

	"winrouter/internal/config"
	"winrouter/internal/core"
	"winrouter/internal/dnssettings"
	"winrouter/internal/observability"
)

func (a *App) GetDNSSettings() (dnssettings.Settings, error) {
	a.mu.RLock()
	manager, loadErr := a.dnsSettings, a.dnsSettingsError
	a.mu.RUnlock()
	if loadErr != nil {
		return dnssettings.Settings{}, loadErr
	}
	if manager == nil {
		return dnssettings.Settings{}, errors.New("DNS settings are not initialized")
	}
	return manager.Get(), nil
}

func (a *App) GetDNSPresets() []dnssettings.Preset { return dnssettings.Presets() }

func (a *App) TestDNSServer(server dnssettings.Server) dnssettings.TestResult {
	return dnssettings.Test(context.Background(), server)
}

func (a *App) SetDNSSettings(settings dnssettings.Settings) (dnssettings.Settings, error) {
	a.mu.RLock()
	manager, loadErr := a.dnsSettings, a.dnsSettingsError
	a.mu.RUnlock()
	if loadErr != nil {
		return dnssettings.Settings{}, loadErr
	}
	if manager == nil {
		return dnssettings.Settings{}, errors.New("DNS settings are not initialized")
	}
	result, err := manager.Configure(settings)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "dns", "DNS settings updated", "", map[string]any{
			"domestic_protocol": result.Domestic.Type, "domestic_outbound": "interface-a",
			"global_protocol": result.Global.Type, "global_outbound": "interface-b",
		})
	}
	return result, err
}

func (a *App) withDNSSettings(input config.MVPConfig) (config.MVPConfig, error) {
	settings, err := a.GetDNSSettings()
	if err != nil {
		return config.MVPConfig{}, err
	}
	input.DNS = config.MVPDNS{
		Domestic: config.MVPDNSServer{Type: settings.Domestic.Type, Server: settings.Domestic.Server, Port: settings.Domestic.Port, ServerName: settings.Domestic.ServerName},
		Global:   config.MVPDNSServer{Type: settings.Global.Type, Server: settings.Global.Server, Port: settings.Global.Port, ServerName: settings.Global.ServerName},
	}
	if settings.Proxy != nil && settings.Proxy.Server != "" {
		input.DNS.Proxy = &config.MVPDNSServer{Type: settings.Proxy.Type, Server: settings.Proxy.Server, Port: settings.Proxy.Port, ServerName: settings.Proxy.ServerName}
	}
	return input, nil
}

func (a *App) dnsDiagnosticStatus(coreStatus core.Status) []observability.DNSStatus {
	settings, err := a.GetDNSSettings()
	if err != nil {
		return []observability.DNSStatus{{Role: "domestic", Outbound: "interface-a", Health: "settings-error"}, {Role: "global", Outbound: "interface-b", Health: "settings-error"}}
	}
	health := "not-running"
	if coreStatus.State == core.StateRunning {
		health = "unverified"
	}
	probes := a.observations.Probes()
	for index := len(probes) - 1; index >= 0; index-- {
		if probes[index].Protocol == "dns" {
			if probes[index].Success {
				health = "healthy"
			} else {
				health = "failed"
			}
			break
		}
	}
	return []observability.DNSStatus{{Role: "domestic", Protocol: settings.Domestic.Type, Outbound: "interface-a", Health: health}, {Role: "global", Protocol: settings.Global.Type, Outbound: "interface-b", Health: health}}
}

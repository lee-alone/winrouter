package app

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"winrouter/internal/dnssettings"
	"winrouter/internal/helperipc"
	"winrouter/internal/interfacemanager"
	"winrouter/internal/observability"
	"winrouter/internal/rulesettings"
)

type NetworkResetStep struct {
	Command  string `json:"command"`
	Success  bool   `json:"success"`
	ExitCode int    `json:"exit_code"`
	Output   string `json:"output,omitempty"`
}

type NetworkResetResult struct {
	Success         bool               `json:"success"`
	RestartRequired bool               `json:"restart_required"`
	Steps           []NetworkResetStep `json:"steps"`
}

func newCorrelationID() string { return fmt.Sprintf("WR-%X", time.Now().UnixNano()) }

func (a *App) ResetWindowsNetworkStack() (NetworkResetResult, error) {
	if _, err := a.StopCore(); err != nil {
		return NetworkResetResult{}, fmt.Errorf("stop routing core: %w", err)
	}
	session, err := a.ensureHelper()
	if err != nil {
		return NetworkResetResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 2*time.Minute)
	defer cancel()
	var result helperipc.NetworkResetResult
	if err := session.Client.CallContext(ctx, helperipc.MethodResetNetworkStack, nil, &result); err != nil {
		a.observations.Log(observability.LevelError, "network-reset", "Windows network stack reset failed", newCorrelationID(), map[string]any{"error": err.Error()})
		return NetworkResetResult{}, err
	}
	level := observability.LevelInfo
	if !result.Success {
		level = observability.LevelWarning
	}
	a.observations.Log(level, "network-reset", "Windows network stack reset completed", "", map[string]any{"success": result.Success, "restart_required": result.RestartRequired})
	converted := NetworkResetResult{Success: result.Success, RestartRequired: result.RestartRequired, Steps: make([]NetworkResetStep, len(result.Steps))}
	for index, step := range result.Steps {
		converted.Steps[index] = NetworkResetStep{Command: step.Command, Success: step.Success, ExitCode: step.ExitCode, Output: step.Output}
	}
	return converted, nil
}

func (a *App) ResetApplicationSettings() (string, error) {
	if _, err := a.StopCore(); err != nil {
		return "", fmt.Errorf("stop routing core: %w", err)
	}
	configDir := a.GetApplicationConfigDirectory()
	if configDir == "" {
		return "", errors.New("cannot determine application configuration directory")
	}
	backupDir, err := backupApplicationSettings(configDir, []string{"interfaces.json", "rule-settings.json", "dns-settings.json"})
	if err != nil {
		return "", err
	}
	a.mu.RLock()
	interfaceManager, interfaceErr := a.interfaceManager, a.interfaceError
	ruleManager, ruleErr := a.ruleSettings, a.ruleSettingsError
	dnsManager, dnsErr := a.dnsSettings, a.dnsSettingsError
	a.mu.RUnlock()
	if interfaceErr != nil || interfaceManager == nil || ruleErr != nil || ruleManager == nil || dnsErr != nil || dnsManager == nil {
		return "", errors.New("application settings managers are not ready; backup was created but no settings were changed")
	}
	if _, err := interfaceManager.ClearSelection(); err != nil {
		return "", fmt.Errorf("reset interface selection: %w", err)
	}
	if _, err := interfaceManager.SetIPv6Policy(interfacemanager.IPv6PolicyBlock); err != nil {
		return "", fmt.Errorf("reset IPv6 policy: %w", err)
	}
	if _, err := ruleManager.Configure(rulesettings.Defaults()); err != nil {
		return "", fmt.Errorf("reset rule settings: %w", err)
	}
	if _, err := dnsManager.Configure(dnssettings.Defaults()); err != nil {
		return "", fmt.Errorf("reset DNS settings: %w", err)
	}
	a.mu.Lock()
	a.lastConfig = nil
	a.mu.Unlock()
	a.observations.Log(observability.LevelWarning, "application-reset", "Application settings restored to defaults", "", map[string]any{"backup": backupDir})
	return backupDir, nil
}

// RepairApplicationSettings rebuilds configuration files without depending on
// managers that may have failed to load. The application must be restarted so
// all background coordinators bind to the newly created managers.
func (a *App) RepairApplicationSettings() (string, error) {
	if _, err := a.StopCore(); err != nil {
		return "", fmt.Errorf("stop routing core: %w", err)
	}
	configDir := a.GetApplicationConfigDirectory()
	if configDir == "" {
		return "", errors.New("cannot determine application configuration directory")
	}
	backupDir, err := backupApplicationSettings(configDir, []string{"interfaces.json", "rule-settings.json", "dns-settings.json"})
	if err != nil {
		return "", err
	}
	if err := rebuildApplicationSettings(configDir); err != nil {
		return "", fmt.Errorf("rebuild application settings (backup: %s): %w", backupDir, err)
	}
	a.observations.Log(observability.LevelWarning, "application-reset", "Application settings repaired; restart required", "", map[string]any{"backup": backupDir})
	return backupDir, nil
}

func rebuildApplicationSettings(configDir string) error {
	paths := []string{
		filepath.Join(configDir, "interfaces.json"),
		filepath.Join(configDir, "rule-settings.json"),
		filepath.Join(configDir, "dns-settings.json"),
	}
	for _, path := range paths {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return fmt.Errorf("remove invalid %s: %w", filepath.Base(path), err)
		}
	}
	if err := interfacemanager.SaveState(paths[0], interfacemanager.State{SchemaVersion: interfacemanager.StateSchemaVersion, IPv6Policy: interfacemanager.IPv6PolicyBlock}); err != nil {
		return err
	}
	ruleManager, err := rulesettings.New(paths[1])
	if err != nil {
		return err
	}
	if _, err := ruleManager.Configure(rulesettings.Defaults()); err != nil {
		return err
	}
	dnsManager, err := dnssettings.New(paths[2])
	if err != nil {
		return err
	}
	_, err = dnsManager.Configure(dnssettings.Defaults())
	return err
}

func backupApplicationSettings(configDir string, names []string) (string, error) {
	backupDir := filepath.Join(configDir, "backups", time.Now().Format("20060102-150405.000"))
	if err := os.MkdirAll(backupDir, 0o700); err != nil {
		return "", fmt.Errorf("create settings backup: %w", err)
	}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(configDir, name))
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", fmt.Errorf("back up %s: %w", name, err)
		}
		if err := os.WriteFile(filepath.Join(backupDir, name), data, 0o600); err != nil {
			return "", fmt.Errorf("write backup %s: %w", name, err)
		}
	}
	return backupDir, nil
}

func (a *App) RecordApplicationLog(level, message, correlation string) {
	wanted := observability.Level(level)
	if wanted != observability.LevelWarning && wanted != observability.LevelError {
		wanted = observability.LevelInfo
	}
	a.observations.Log(wanted, "ui", message, correlation, nil)
}

func (a *App) RunHealthProbe(request observability.ProbeRequest) (observability.ProbeResult, error) {
	snapshot, err := a.GetInterfaceSnapshot()
	if err != nil {
		return observability.ProbeResult{}, err
	}
	result := observability.RunProbe(a.ctx, request, snapshot.Adapters)
	a.observations.AddProbe(result)
	level := observability.LevelInfo
	if !result.Success {
		level = observability.LevelWarning
	}
	a.observations.Log(level, "probe", fmt.Sprintf("%s probe %s", strings.ToUpper(result.Protocol), map[bool]string{true: "passed", false: "failed"}[result.Success]), "", map[string]any{"target": result.Target, "source_address": result.SourceAddress, "expected_interface": result.ExpectedInterface, "actual_interface": result.ActualInterface, "duration_ms": result.DurationMS, "error": result.Error})
	return result, nil
}

func (a *App) PreviewDiagnosticBundle() (observability.BundlePreview, error) {
	input, err := a.diagnosticInput()
	if err != nil {
		return observability.BundlePreview{}, err
	}
	return observability.Preview(input), nil
}

func (a *App) ExportDiagnosticBundle() (string, error) {
	input, err := a.diagnosticInput()
	if err != nil {
		return "", err
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{Title: "导出 WinRouter 诊断包", DefaultFilename: fmt.Sprintf("winrouter-diagnostic-%s.zip", time.Now().Format("20060102-150405")), Filters: []runtime.FileFilter{{DisplayName: "ZIP 诊断包 (*.zip)", Pattern: "*.zip"}}})
	if err != nil || path == "" {
		return path, err
	}
	if err = observability.WriteBundle(path, input); err != nil {
		return "", err
	}
	a.observations.Log(observability.LevelInfo, "diagnostics", "Diagnostic bundle exported", "", map[string]any{"path": filepath.Base(path)})
	return path, nil
}

func (a *App) diagnosticInput() (observability.BundleInput, error) {
	interfacesSnapshot, err := a.GetInterfaceSnapshot()
	if err != nil {
		return observability.BundleInput{}, err
	}
	coreStatus, err := a.GetCoreStatus()
	if err != nil {
		return observability.BundleInput{}, err
	}
	counters, err := observability.ReadInterfaceCounters(interfacesSnapshot.Adapters)
	if err != nil {
		return observability.BundleInput{}, err
	}
	return observability.BundleInput{Application: a.GetStatus(), Interfaces: interfacesSnapshot, Core: coreStatus, Logs: a.observations.Logs(), Probes: a.observations.Probes(), Counters: counters, RuleSets: a.observations.RuleSets(), DNS: a.dnsDiagnosticStatus(coreStatus)}, nil
}

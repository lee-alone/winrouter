package app

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"winrouter/internal/config"
	"winrouter/internal/core"
	"winrouter/internal/helperclient"
	"winrouter/internal/helperipc"
	"winrouter/internal/interfacemanager"
	"winrouter/internal/observability"
	"winrouter/internal/recovery"
)

func (a *App) isCoreRunning() bool {
	status, err := a.GetCoreStatus()
	return err == nil && status.State == core.StateRunning
}

func (a *App) ValidateCoreConfiguration(input config.MVPConfig) error {
	if input.Mode == "" {
		if manager, err := a.getInterfaceManager(); err == nil {
			input.Mode = manager.Mode()
		}
	}
	a.applyConnectionObservation(&input)
	var err error
	input, err = a.withRuleSettings(input)
	if err != nil {
		return err
	}
	input, err = a.withDNSSettings(input)
	if err != nil {
		return err
	}
	input, err = a.withSelectedProxy(input)
	if err != nil {
		return err
	}
	if statuses, inspectErr := a.InspectProcessRules(input); inspectErr != nil {
		return inspectErr
	} else {
		for _, status := range statuses {
			if status.State == "ambiguous" {
				return fmt.Errorf("process rule %q is ambiguous: use a full process path", status.Value)
			}
		}
	}
	input, err = a.withSRSRuleSets(input)
	if err != nil {
		return err
	}
	_, err = config.GenerateMVP(input)
	if err != nil {
		return err
	}
	session, err := a.ensureHelper()
	if err != nil {
		return err
	}
	data, err := json.Marshal(input)
	if err != nil {
		return err
	}
	err = session.Client.Call(helperipc.MethodValidate, helperipc.ConfigParams{Config: data}, nil)
	if err != nil {
		a.observations.Log(observability.LevelError, "core", "Core configuration validation failed", newCorrelationID(), map[string]any{"error": err.Error()})
	} else {
		a.observations.Log(observability.LevelInfo, "core", "Core configuration validated", "", nil)
	}
	return err
}

func (a *App) ApplyCoreConfiguration(input config.MVPConfig) (core.Status, error) {
	if input.Mode == "" {
		if manager, err := a.getInterfaceManager(); err == nil {
			input.Mode = manager.Mode()
		}
	}
	a.applyConnectionObservation(&input)
	var err error
	input, err = a.withRuleSettings(input)
	if err != nil {
		return core.Status{}, err
	}
	input, err = a.withDNSSettings(input)
	if err != nil {
		return core.Status{}, err
	}
	input, err = a.withSelectedProxy(input)
	if err != nil {
		return core.Status{}, err
	}
	if statuses, inspectErr := a.InspectProcessRules(input); inspectErr != nil {
		return core.Status{}, inspectErr
	} else {
		for _, status := range statuses {
			if status.State == "ambiguous" {
				return core.Status{}, fmt.Errorf("process rule %q is ambiguous: use a full process path", status.Value)
			}
		}
	}
	input, err = a.withSRSRuleSets(input)
	if err != nil {
		return core.Status{}, err
	}
	_, err = config.GenerateMVP(input)
	if err != nil {
		return core.Status{}, err
	}
	session, err := a.ensureHelper()
	if err != nil {
		return core.Status{}, err
	}
	var status core.Status
	data, err := json.Marshal(input)
	if err != nil {
		return core.Status{}, err
	}
	err = session.Client.Call(helperipc.MethodApply, helperipc.ConfigParams{Config: data}, &status)
	if err != nil {
		a.observations.Log(observability.LevelError, "core", "Core apply failed", newCorrelationID(), map[string]any{"error": err.Error()})
	} else {
		stored := cloneMVPConfig(input)
		a.mu.Lock()
		a.lastConfig = &stored
		coordinator := a.recovery
		a.mu.Unlock()
		if coordinator != nil {
			coordinator.SetDesired(true)
		}
		a.tray.SetStatus("运行中", true)
		a.observations.Log(observability.LevelInfo, "core", "Core configuration applied", "", map[string]any{"pid": status.PID, "generation": status.Generation})
		metadata, _ := json.Marshal(struct{ CIDRs, Domains []string }{input.Domestic.CIDRs, input.Domestic.DomainSuffixes})
		digest := sha256.Sum256(metadata)
		items := make([]observability.RuleSetMetadata, 0)
		if len(input.Domestic.CIDRs)+len(input.Domestic.DomainSuffixes) > 0 {
			items = append(items, observability.RuleSetMetadata{Name: "domestic-policy", Version: fmt.Sprintf("schema-v%d", input.SchemaVersion), SHA256: fmt.Sprintf("%x", digest), Source: "winrouter-config", RuleCount: len(input.Domestic.CIDRs) + len(input.Domestic.DomainSuffixes), Size: int64(len(metadata)), LoadResult: "validated-and-applied"})
		}
		if manager, managerErr := a.getSRSManager(); managerErr == nil {
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
	return status, err
}

func (a *App) StopCore() (core.Status, error) {
	a.mu.RLock()
	coordinator := a.recovery
	a.mu.RUnlock()
	if coordinator != nil {
		coordinator.SetDesired(false)
	}
	session, err := a.currentHelper()
	if err != nil {
		return core.Status{State: core.StateStopped}, nil
	}
	var status core.Status
	err = session.Client.Call(helperipc.MethodStop, nil, &status)
	if err != nil {
		a.observations.Log(observability.LevelError, "core", "Core stop failed", newCorrelationID(), map[string]any{"error": err.Error()})
	} else {
		a.tray.SetStatus("已停止", false)
		a.observations.Log(observability.LevelInfo, "core", "Core stopped", "", map[string]any{"generation": status.Generation})
	}
	return status, err
}

func locateBundledCore() (string, error) {
	return core.Locate()
}

func (a *App) GetCoreStatus() (core.Status, error) {
	session, err := a.currentHelper()
	if err != nil {
		return core.Status{State: core.StateStopped}, nil
	}
	var status core.Status
	err = session.Client.Call(helperipc.MethodStatus, nil, &status)
	return status, err
}

func (a *App) GetRecoveryStatus() recovery.Status {
	a.mu.RLock()
	coordinator := a.recovery
	a.mu.RUnlock()
	if coordinator == nil {
		return recovery.Status{State: recovery.StateIdle}
	}
	return coordinator.Status()
}

func (a *App) stopForRecovery(ctx context.Context) error {
	session, err := a.currentHelper()
	if err != nil {
		return nil
	}
	var status core.Status
	if err := session.Client.CallContext(ctx, helperipc.MethodStop, nil, &status); err != nil {
		return err
	}
	a.observations.Log(observability.LevelWarning, "recovery", "Core stopped because a selected interface became unavailable", "", map[string]any{"generation": status.Generation})
	return nil
}

func (a *App) applyForRecovery(ctx context.Context, snapshot interfacemanager.Snapshot) error {
	a.mu.RLock()
	if a.lastConfig == nil {
		a.mu.RUnlock()
		return errors.New("no previously applied configuration")
	}
	base := cloneMVPConfig(*a.lastConfig)
	a.mu.RUnlock()
	rebuilt, err := recovery.RebuildConfig(base, snapshot)
	if err != nil {
		return err
	}
	data, err := json.Marshal(rebuilt)
	if err != nil {
		return err
	}
	session, err := a.ensureHelper()
	if err != nil {
		return err
	}
	var status core.Status
	if err := session.Client.CallContext(ctx, helperipc.MethodApply, helperipc.ConfigParams{Config: data}, &status); err != nil {
		return err
	}
	stored := cloneMVPConfig(rebuilt)
	a.mu.Lock()
	a.lastConfig = &stored
	a.mu.Unlock()
	a.observations.Log(observability.LevelInfo, "recovery", "Core configuration rebuilt after network recovery", "", map[string]any{"pid": status.PID, "generation": status.Generation, "snapshot_sequence": snapshot.Sequence})
	return nil
}

func cloneMVPConfig(input config.MVPConfig) config.MVPConfig {
	data, _ := json.Marshal(input)
	var clone config.MVPConfig
	_ = json.Unmarshal(data, &clone)
	return clone
}

func (a *App) ensureHelper() (*helperclient.Session, error) {
	if session, err := a.currentHelper(); err == nil {
		return session, nil
	}
	session, err := helperclient.Launch(a.ctx)
	if err != nil {
		return nil, err
	}
	a.mu.Lock()
	if a.helperSession != nil {
		a.mu.Unlock()
		session.Close()
		return a.currentHelper()
	}
	a.helperSession = session
	a.mu.Unlock()
	return session, nil
}

func (a *App) currentHelper() (*helperclient.Session, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.helperSession == nil {
		return nil, errors.New("helper is not running")
	}
	return a.helperSession, nil
}

func (a *App) monitorCoreRuntime(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		a.mu.RLock()
		coordinator := a.recovery
		manager := a.interfaceManager
		session := a.helperSession
		a.mu.RUnlock()
		if coordinator == nil || manager == nil {
			continue
		}
		recoveryStatus := coordinator.Status()
		if !recoveryStatus.Desired || recoveryStatus.State != recovery.StateMonitoring {
			continue
		}
		if session == nil {
			coordinator.RuntimeFailed(manager.Snapshot(), errors.New("helper session is missing"))
			continue
		}
		operationCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		var status core.Status
		err := session.Client.CallContext(operationCtx, helperipc.MethodStatus, nil, &status)
		cancel()
		if err == nil && status.State == core.StateRunning {
			a.tray.SetStatus("运行中", true)
			continue
		}
		a.tray.SetStatus("恢复中", false)
		if err == nil {
			err = fmt.Errorf("core state is %s", status.State)
		} else {
			a.invalidateHelper(session)
		}
		a.observations.Log(observability.LevelWarning, "recovery", "Core runtime became unavailable", "", map[string]any{"error": err.Error()})
		coordinator.RuntimeFailed(manager.Snapshot(), err)
	}
}

func (a *App) invalidateHelper(session *helperclient.Session) {
	a.mu.Lock()
	if a.helperSession == session {
		a.helperSession = nil
	}
	a.mu.Unlock()
	session.Close()
}

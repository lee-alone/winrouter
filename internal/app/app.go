package app

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"winrouter/internal/autostart"
	"winrouter/internal/buildinfo"
	"winrouter/internal/config"
	"winrouter/internal/configdir"
	"winrouter/internal/core"
	"winrouter/internal/dnssettings"
	"winrouter/internal/helperclient"
	"winrouter/internal/helperipc"
	"winrouter/internal/interfacemanager"
	"winrouter/internal/nodes"
	"winrouter/internal/observability"
	"winrouter/internal/recovery"
	"winrouter/internal/rulesettings"
	"winrouter/internal/srssets"
	"winrouter/internal/subscriptions"
	"winrouter/internal/trafficbudget"
	apptray "winrouter/internal/tray"
)

type App struct {
	ctx                   context.Context
	mu                    sync.RWMutex
	interfaceManager      *interfacemanager.Manager
	interfaceError        error
	nodeStore             *nodes.Store
	nodeError             error
	subscriptions         *subscriptions.Manager
	ruleSettings          *rulesettings.Manager
	ruleSettingsError     error
	srsSets               *srssets.Manager
	srsSetError           error
	trafficBudget         *trafficbudget.Manager
	trafficBudgetError    error
	dnsSettings           *dnssettings.Manager
	dnsSettingsError      error
	subscriptionError     error
	helperSession         *helperclient.Session
	observations          *observability.Store
	recovery              *recovery.Coordinator
	lastConfig            *config.MVPConfig
	tray                  apptray.Controller
	exiting               atomic.Bool
	windowVisible         atomic.Bool
	connectionObservation atomic.Bool
	connectionAPISecret   string
	configInfo            configdir.Info
	protector             *nodes.PortableProtector
	detectedCoreVer       string
}

type ApplicationStatus struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	CoreVersion string `json:"coreVersion"`
	Ready       bool   `json:"ready"`
}

func New() *App {
	app := &App{observations: observability.NewStore(500)}
	app.windowVisible.Store(true)
	return app
}

func NewApp() *App {
	return New()
}

func (a *App) Shutdown(context.Context) {
	a.tray.Stop()
	a.mu.Lock()
	session := a.helperSession
	a.helperSession = nil
	coordinator := a.recovery
	a.mu.Unlock()
	if coordinator != nil {
		coordinator.SetDesired(false)
	}
	if session != nil {
		var status core.Status
		_ = session.Client.Call(helperipc.MethodStop, nil, &status)
		session.Close()
	}
}

func (a *App) BeforeClose(ctx context.Context) bool {
	if a.exiting.Load() {
		return false
	}
	runtime.WindowHide(ctx)
	a.windowVisible.Store(false)
	return true
}

func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
	a.observations.Log(observability.LevelInfo, "application", "WinRouter started", "", nil)
	info, err := configdir.Resolve()
	if err != nil {
		a.setInterfaceError(fmt.Errorf("locate configuration directory: %w", err))
		return
	}
	a.mu.Lock()
	a.configInfo = info
	a.mu.Unlock()
	configDirectory := info.Path
	if info.Migrated {
		a.observations.Log(observability.LevelInfo, "application", "Migrated existing configuration to portable directory", "", map[string]any{"path": configDirectory})
	}

	protector, protectorErr := nodes.NewPortableProtector(configDirectory, nodes.DPAPIProtector{})
	a.mu.Lock()
	a.protector = protector
	a.mu.Unlock()
	if protectorErr != nil {
		a.observations.Log(observability.LevelError, "security", "Failed to initialize portable protector", newCorrelationID(), map[string]any{"error": protectorErr.Error()})
	}

	nodeStore, nodeErr := nodes.New(filepath.Join(configDirectory, "nodes.json"), protector)
	a.mu.Lock()
	a.nodeStore = nodeStore
	a.nodeError = nodeErr
	a.mu.Unlock()
	if nodeErr != nil {
		a.observations.Log(observability.LevelError, "proxy", "Proxy node store failed to load", newCorrelationID(), map[string]any{"error": nodeErr.Error()})
	}
	if nodeErr == nil {
		subscriptionManager, subscriptionErr := subscriptions.New(filepath.Join(configDirectory, "subscriptions.json"), protector, nodeStore)
		a.mu.Lock()
		a.subscriptions = subscriptionManager
		a.subscriptionError = subscriptionErr
		a.mu.Unlock()
		if subscriptionErr != nil {
			a.observations.Log(observability.LevelError, "proxy", "Subscription store failed to load", newCorrelationID(), map[string]any{"error": subscriptionErr.Error()})
		}
	}
	ruleSettingsManager, ruleSettingsErr := rulesettings.New(filepath.Join(configDirectory, "rule-settings.json"))
	a.mu.Lock()
	a.ruleSettings, a.ruleSettingsError = ruleSettingsManager, ruleSettingsErr
	a.mu.Unlock()
	if ruleSettingsErr != nil {
		a.observations.Log(observability.LevelError, "rules", "Rule settings store failed to load", newCorrelationID(), map[string]any{"error": ruleSettingsErr.Error()})
	}
	srsPath, srsPathErr := locateBundledCore()
	if srsPath != "" {
		if ver, err := core.DetectCoreVersion(ctx, srsPath); err == nil && ver != "" {
			a.mu.Lock()
			a.detectedCoreVer = ver
			a.mu.Unlock()
		}
	}
	var srsManager *srssets.Manager
	var srsErr error
	if srsPathErr != nil {
		srsErr = srsPathErr
	} else {
		srsManager, srsErr = srssets.New(filepath.Join(configDirectory, "srs-sources.json"), srsPath)
	}
	a.mu.Lock()
	a.srsSets, a.srsSetError = srsManager, srsErr
	a.mu.Unlock()
	if srsErr != nil {
		a.observations.Log(observability.LevelError, "rules", "SRS source store failed to load", newCorrelationID(), map[string]any{"error": srsErr.Error()})
	} else {
		a.syncAllRuleSetMetadata()
	}
	budgetManager, budgetErr := trafficbudget.New(filepath.Join(configDirectory, "traffic-budget.json"))
	a.mu.Lock()
	a.trafficBudget = budgetManager
	a.trafficBudgetError = budgetErr
	a.mu.Unlock()
	if budgetErr != nil {
		a.observations.Log(observability.LevelError, "traffic-budget", "Traffic budget store failed to load", newCorrelationID(), map[string]any{"error": budgetErr.Error()})
	}
	dnsManager, dnsErr := dnssettings.New(filepath.Join(configDirectory, "dns-settings.json"))
	a.mu.Lock()
	a.dnsSettings, a.dnsSettingsError = dnsManager, dnsErr
	a.mu.Unlock()
	if dnsErr != nil {
		a.observations.Log(observability.LevelError, "dns", "DNS settings store failed to load", newCorrelationID(), map[string]any{"error": dnsErr.Error()})
	}
	manager, err := interfacemanager.New(interfacemanager.Options{StatePath: filepath.Join(configDirectory, "interfaces.json")})
	if err != nil {
		a.setInterfaceError(err)
		return
	}
	if _, err := manager.Refresh(); err != nil {
		a.observations.Log(observability.LevelWarning, "interfaces", "Initial interface refresh warning", "", map[string]any{"error": err.Error()})
	}
	a.mu.Lock()
	a.interfaceManager = manager
	a.interfaceError = nil
	a.mu.Unlock()
	coordinator, err := recovery.New(recovery.Options{
		Stop:  a.stopForRecovery,
		Apply: a.applyForRecovery,
		OnChange: func(status recovery.Status) {
			runtime.EventsEmit(ctx, "recovery:changed", status)
			a.tray.SetStatus(trayStatusLabel(status), status.State == recovery.StateMonitoring && status.Desired)
			level := observability.LevelInfo
			if status.State == recovery.StateWaiting || status.State == recovery.StateFailed {
				level = observability.LevelWarning
			}
			a.observations.Log(level, "recovery", "Network recovery state changed", "", map[string]any{"state": status.State, "attempts": status.Attempts, "error": status.LastError})
		},
	})
	if err != nil {
		a.setInterfaceError(err)
		return
	}
	a.mu.Lock()
	a.recovery = coordinator
	a.mu.Unlock()
	go coordinator.Run(ctx)
	events, unsubscribe := manager.Subscribe(1)
	go func() {
		defer unsubscribe()
		for {
			select {
			case <-ctx.Done():
				return
			case event, ok := <-events:
				if !ok {
					return
				}
				coordinator.NetworkChanged(event.Snapshot)
				runtime.EventsEmit(ctx, "interfaces:changed", event)
				a.observations.Log(observability.LevelInfo, "interfaces", "Network interface snapshot changed", "", map[string]any{"reason": event.Reason})
			}
		}
	}()
	go func() {
		if err := manager.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			a.setInterfaceError(err)
			runtime.EventsEmit(ctx, "interfaces:error", err.Error())
		}
	}()
	go a.monitorCoreRuntime(ctx)
}

func (a *App) GetStatus() ApplicationStatus {
	a.mu.RLock()
	coreVer := a.detectedCoreVer
	a.mu.RUnlock()
	if coreVer == "" {
		coreVer = buildinfo.CoreVersion
	}
	return ApplicationStatus{Name: "WinRouter", Version: buildinfo.Version, Commit: buildinfo.Commit, CoreVersion: coreVer, Ready: false}
}

func (a *App) GetApplicationConfigDirectory() string {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.configInfo.Path != "" {
		return a.configInfo.Path
	}
	info, err := configdir.Resolve()
	if err == nil {
		return info.Path
	}
	return ""
}

func (a *App) GetApplicationConfigInfo() configdir.Info {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.configInfo.Path != "" {
		return a.configInfo
	}
	info, err := configdir.Resolve()
	if err == nil {
		return info
	}
	return configdir.Info{}
}

func (a *App) GetAutostartStatus() (autostart.Status, error) { return autostart.Get() }

func (a *App) SetAutostartEnabled(enabled bool) (autostart.Status, error) {
	status, err := autostart.Set(enabled)
	if err != nil {
		a.observations.Log(observability.LevelError, "application", "Autostart update failed", newCorrelationID(), map[string]any{"error": err.Error()})
		return autostart.Status{}, err
	}
	a.observations.Log(observability.LevelInfo, "application", "Autostart setting updated", "", map[string]any{"enabled": status.Enabled})
	return status, nil
}

package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"winrouter/internal/autostart"
	"winrouter/internal/buildinfo"
	"winrouter/internal/clashapi"
	"winrouter/internal/config"
	"winrouter/internal/core"
	"winrouter/internal/dnssettings"
	"winrouter/internal/helperclient"
	"winrouter/internal/helperipc"
	"winrouter/internal/interfacemanager"
	"winrouter/internal/interfaces"
	"winrouter/internal/nodes"
	"winrouter/internal/observability"
	"winrouter/internal/processrules"
	"winrouter/internal/recovery"
	"winrouter/internal/rulesets"
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
	ruleSets              *rulesets.Manager
	ruleSetError          error
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
}

type ApplicationStatus struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Commit      string `json:"commit"`
	Mode        string `json:"mode"`
	CoreVersion string `json:"coreVersion"`
	Ready       bool   `json:"ready"`
}

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

func NewApp() *App {
	app := &App{observations: observability.NewStore(500)}
	app.windowVisible.Store(true)
	return app
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

func (a *App) StartTray(icon []byte) {
	a.tray.Start(icon, apptray.Actions{
		Show:   a.showMainWindow,
		Toggle: a.toggleMainWindow,
		Start: func() {
			a.showMainWindow()
			runtime.EventsEmit(a.ctx, "tray:start-core")
		},
		Stop: func() {
			if _, err := a.StopCore(); err != nil {
				a.observations.Log(observability.LevelError, "tray", "Core stop from tray failed", newCorrelationID(), map[string]any{"error": err.Error()})
			}
		},
		Quit: func() {
			a.exiting.Store(true)
			runtime.Quit(a.ctx)
		},
	})
}

func (a *App) showMainWindow() {
	a.windowVisible.Store(true)
	runtime.WindowShow(a.ctx)
	runtime.WindowUnminimise(a.ctx)
}

func (a *App) toggleMainWindow() {
	if a.windowVisible.CompareAndSwap(true, false) {
		runtime.WindowHide(a.ctx)
		return
	}
	a.showMainWindow()
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
	configDirectory, err := os.UserConfigDir()
	if err != nil {
		a.setInterfaceError(fmt.Errorf("locate user configuration: %w", err))
		return
	}
	nodeStore, nodeErr := nodes.New(filepath.Join(configDirectory, "WinRouter", "nodes.json"), nodes.DPAPIProtector{})
	a.mu.Lock()
	a.nodeStore = nodeStore
	a.nodeError = nodeErr
	a.mu.Unlock()
	if nodeErr != nil {
		a.observations.Log(observability.LevelError, "proxy", "Proxy node store failed to load", newCorrelationID(), map[string]any{"error": nodeErr.Error()})
	}
	if nodeErr == nil {
		subscriptionManager, subscriptionErr := subscriptions.New(filepath.Join(configDirectory, "WinRouter", "subscriptions.json"), nodes.DPAPIProtector{}, nodeStore)
		a.mu.Lock()
		a.subscriptions = subscriptionManager
		a.subscriptionError = subscriptionErr
		a.mu.Unlock()
		if subscriptionErr != nil {
			a.observations.Log(observability.LevelError, "proxy", "Subscription store failed to load", newCorrelationID(), map[string]any{"error": subscriptionErr.Error()})
		}
	}
	ruleSetManager, ruleSetErr := rulesets.New(filepath.Join(configDirectory, "WinRouter", "ruleset.json"))
	a.mu.Lock()
	a.ruleSets = ruleSetManager
	a.ruleSetError = ruleSetErr
	a.mu.Unlock()
	if ruleSetErr != nil {
		a.observations.Log(observability.LevelError, "rules", "Remote rule-set store failed to load", newCorrelationID(), map[string]any{"error": ruleSetErr.Error()})
	} else {
		a.syncRuleSetMetadata(ruleSetManager.Get())
	}
	ruleSettingsManager, ruleSettingsErr := rulesettings.New(filepath.Join(configDirectory, "WinRouter", "rule-settings.json"))
	a.mu.Lock()
	a.ruleSettings, a.ruleSettingsError = ruleSettingsManager, ruleSettingsErr
	a.mu.Unlock()
	if ruleSettingsErr != nil {
		a.observations.Log(observability.LevelError, "rules", "Rule settings store failed to load", newCorrelationID(), map[string]any{"error": ruleSettingsErr.Error()})
	}
	srsPath, srsPathErr := locateBundledCore()
	var srsManager *srssets.Manager
	var srsErr error
	if srsPathErr != nil {
		srsErr = srsPathErr
	} else {
		srsManager, srsErr = srssets.New(filepath.Join(configDirectory, "WinRouter", "srs-sources.json"), srsPath)
	}
	a.mu.Lock()
	a.srsSets, a.srsSetError = srsManager, srsErr
	a.mu.Unlock()
	if srsErr != nil {
		a.observations.Log(observability.LevelError, "rules", "SRS source store failed to load", newCorrelationID(), map[string]any{"error": srsErr.Error()})
	} else {
		a.syncAllRuleSetMetadata()
	}
	budgetManager, budgetErr := trafficbudget.New(filepath.Join(configDirectory, "WinRouter", "traffic-budget.json"))
	a.mu.Lock()
	a.trafficBudget = budgetManager
	a.trafficBudgetError = budgetErr
	a.mu.Unlock()
	if budgetErr != nil {
		a.observations.Log(observability.LevelError, "traffic-budget", "Traffic budget store failed to load", newCorrelationID(), map[string]any{"error": budgetErr.Error()})
	}
	dnsManager, dnsErr := dnssettings.New(filepath.Join(configDirectory, "WinRouter", "dns-settings.json"))
	a.mu.Lock()
	a.dnsSettings, a.dnsSettingsError = dnsManager, dnsErr
	a.mu.Unlock()
	if dnsErr != nil {
		a.observations.Log(observability.LevelError, "dns", "DNS settings store failed to load", newCorrelationID(), map[string]any{"error": dnsErr.Error()})
	}
	manager, err := interfacemanager.New(interfacemanager.Options{StatePath: filepath.Join(configDirectory, "WinRouter", "interfaces.json")})
	if err != nil {
		a.setInterfaceError(err)
		return
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
	return ApplicationStatus{Name: "WinRouter", Version: buildinfo.Version, Commit: buildinfo.Commit, Mode: "direct-split", CoreVersion: buildinfo.CoreVersion, Ready: false}
}

func (a *App) GetApplicationConfigDirectory() string {
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return ""
	}
	return filepath.Join(configRoot, "WinRouter")
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

func (a *App) ListProxyNodes() ([]nodes.Node, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nil, err
	}
	return store.List(), nil
}

func (a *App) AddProxyNode(input nodes.Input) (nodes.Node, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.Node{}, err
	}
	result, err := store.Add(input)
	if err != nil {
		return nodes.Node{}, err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy node added", "", map[string]any{"node_id": result.ID, "type": result.Type})
	return result, nil
}

func (a *App) UpdateProxyNode(input nodes.Input) (nodes.Node, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.Node{}, err
	}
	result, err := store.Update(input)
	if err != nil {
		return nodes.Node{}, err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy node updated", "", map[string]any{"node_id": result.ID, "type": result.Type})
	return result, nil
}

func (a *App) DeleteProxyNode(id string) error {
	store, err := a.getNodeStore()
	if err != nil {
		return err
	}
	if err := store.Delete(id); err != nil {
		return err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy node deleted", "", map[string]any{"node_id": id})
	return nil
}

func (a *App) SelectProxyNode(id string) (nodes.Node, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.Node{}, err
	}
	result, err := store.Select(id)
	if err != nil {
		return nodes.Node{}, err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy node selected", "", map[string]any{"node_id": result.ID, "type": result.Type})
	return result, nil
}

func (a *App) SetProxyNodeFavorite(id string, favorite bool) (nodes.Node, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.Node{}, err
	}
	result, err := store.SetFavorite(id, favorite)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "proxy", "Proxy node favorite updated", "", map[string]any{"node_id": id, "favorite": favorite})
	}
	return result, err
}

func (a *App) SpeedTestProxyNodes(dnsServer string) ([]nodes.TestResult, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nil, err
	}
	items := store.List()
	results := make([]nodes.TestResult, len(items))
	semaphore := make(chan struct{}, 4)
	var group sync.WaitGroup
	for index, item := range items {
		index, item := index, item
		group.Add(1)
		go func() {
			defer group.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			result, testErr := a.TestProxyNode(item.ID, dnsServer)
			if testErr != nil {
				result = nodes.TestResult{NodeID: item.ID, TestedAt: time.Now().UTC(), Error: testErr.Error()}
			}
			results[index] = result
		}()
	}
	group.Wait()
	return results, nil
}

func (a *App) TestProxyNode(id, dnsServer string) (nodes.TestResult, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.TestResult{}, err
	}
	node, credentials, err := store.Credentials(id)
	if err != nil {
		return nodes.TestResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
	defer cancel()
	if net.ParseIP(node.Server) == nil {
		source, sourceErr := a.interfaceBSourceIPv4()
		if sourceErr != nil {
			return nodes.TestResult{}, sourceErr
		}
		resolved, resolveErr := nodes.ResolveIPv4(ctx, node.Server, dnsServer, source)
		if resolveErr != nil {
			return nodes.TestResult{}, fmt.Errorf("proxy DNS via interface B: %w", resolveErr)
		}
		node.ResolvedIP = resolved
	}
	result := nodes.Probe(ctx, node, credentials)
	if result.Available && net.ParseIP(node.Server) == nil && node.ResolvedIP != "" {
		if _, commitErr := store.CommitResolvedIP(node.ID, node.ResolvedIP); commitErr != nil {
			return nodes.TestResult{}, commitErr
		}
	}
	level := observability.LevelInfo
	if !result.Available {
		level = observability.LevelWarning
	}
	a.observations.Log(level, "proxy", "Proxy node availability tested", "", map[string]any{"node_id": node.ID, "available": result.Available, "latency_ms": result.LatencyMS, "status": result.Status})
	return result, nil
}

func (a *App) ValidateSelectedProxyConfiguration(input config.MVPConfig) error {
	enriched, err := a.withSelectedProxy(input)
	if err != nil {
		return err
	}
	return a.ValidateCoreConfiguration(enriched)
}

func (a *App) PreviewCoreRules(input config.MVPConfig) ([]config.RulePreview, error) {
	var err error
	input, err = a.withRuleSettings(input)
	if err != nil {
		return nil, err
	}
	input, err = a.withDNSSettings(input)
	if err != nil {
		return nil, err
	}
	if input.Mode == config.ModeProxySplit {
		input, err = a.withSelectedProxy(input)
		if err != nil {
			return nil, err
		}
	}
	input, err = a.withRemoteRules(input)
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

func (a *App) ApplySelectedProxyConfiguration(input config.MVPConfig) (core.Status, error) {
	enriched, err := a.withSelectedProxy(input)
	if err != nil {
		return core.Status{}, err
	}
	return a.ApplyCoreConfiguration(enriched)
}

func (a *App) withSelectedProxy(input config.MVPConfig) (config.MVPConfig, error) {
	if input.Mode != config.ModeProxySplit {
		return config.MVPConfig{}, errors.New("selected proxy configuration requires proxy-split mode")
	}
	store, err := a.getNodeStore()
	if err != nil {
		return config.MVPConfig{}, err
	}
	node, credentials, err := store.SelectedCredentials()
	if err != nil {
		return config.MVPConfig{}, err
	}
	server := node.Server
	if net.ParseIP(server) == nil {
		if input.DNS.Global.Type != "udp" || input.DNS.Global.Port != 53 {
			return config.MVPConfig{}, errors.New("proxy domain bootstrap currently requires fixed UDP DNS on port 53")
		}
		source, sourceErr := a.interfaceBSourceIPv4()
		if sourceErr != nil {
			return config.MVPConfig{}, sourceErr
		}
		ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
		defer cancel()
		resolved, resolveErr := nodes.ResolveIPv4(ctx, node.Server, input.DNS.Global.Server, source)
		if resolveErr != nil {
			return config.MVPConfig{}, fmt.Errorf("proxy DNS via interface B: %w", resolveErr)
		}
		candidate := node
		candidate.ResolvedIP = resolved
		probe := nodes.Probe(ctx, candidate, credentials)
		if !probe.Available {
			return config.MVPConfig{}, fmt.Errorf("resolved proxy endpoint failed health check: %s", probe.Error)
		}
		if _, commitErr := store.CommitResolvedIP(node.ID, resolved); commitErr != nil {
			return config.MVPConfig{}, commitErr
		}
		server = resolved
	}
	input.Proxy = &config.MVPProxy{Type: node.Type, Server: server, Port: node.Port, Password: credentials.Password}
	if node.Type == nodes.TypeShadowsocks {
		input.Proxy.Method = credentials.Username
	} else {
		input.Proxy.Username = credentials.Username
	}
	return input, nil
}

func (a *App) interfaceBSourceIPv4() (string, error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return "", err
	}
	snapshot := manager.Snapshot()
	if snapshot.InterfaceB.Match == nil {
		return "", errors.New("interface B is not resolved")
	}
	for _, address := range snapshot.InterfaceB.Match.Adapter.Addresses {
		parsed := net.ParseIP(address.IP)
		if parsed != nil && parsed.To4() != nil && !parsed.IsLoopback() && !parsed.IsUnspecified() {
			return parsed.String(), nil
		}
	}
	return "", errors.New("interface B has no usable IPv4 source address")
}

func (a *App) getNodeStore() (*nodes.Store, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.nodeError != nil {
		return nil, a.nodeError
	}
	if a.nodeStore == nil {
		return nil, errors.New("proxy node store is not started")
	}
	return a.nodeStore, nil
}

func (a *App) ListSubscriptions() ([]subscriptions.Subscription, error) {
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return nil, err
	}
	return manager.List(), nil
}

func (a *App) AddSubscription(input subscriptions.Input) (subscriptions.Subscription, error) {
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return subscriptions.Subscription{}, err
	}
	result, err := manager.Add(input)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "proxy", "Subscription added", "", map[string]any{"subscription_id": result.ID, "host": result.Host})
	}
	return result, err
}

func (a *App) UpdateSubscription(input subscriptions.Input) (subscriptions.Subscription, error) {
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return subscriptions.Subscription{}, err
	}
	result, err := manager.UpdateDefinition(input)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "proxy", "Subscription definition updated", "", map[string]any{"subscription_id": result.ID, "host": result.Host})
	}
	return result, err
}

func (a *App) DeleteSubscription(id string) error {
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return err
	}
	if err := manager.Delete(id); err != nil {
		return err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Subscription deleted", "", map[string]any{"subscription_id": id})
	return nil
}

func (a *App) RefreshSubscription(id string) (subscriptions.Subscription, error) {
	manager, err := a.getSubscriptionManager()
	if err != nil {
		return subscriptions.Subscription{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	result, err := manager.Refresh(ctx, id)
	if err != nil {
		a.observations.Log(observability.LevelWarning, "proxy", "Subscription update rejected; previous nodes retained", newCorrelationID(), map[string]any{"subscription_id": id, "error": err.Error()})
		return subscriptions.Subscription{}, err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Subscription updated", "", map[string]any{"subscription_id": id, "node_count": result.NodeCount})
	return result, nil
}

func (a *App) getSubscriptionManager() (*subscriptions.Manager, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.subscriptionError != nil {
		return nil, a.subscriptionError
	}
	if a.subscriptions == nil {
		return nil, errors.New("subscription manager is not started")
	}
	return a.subscriptions, nil
}

func (a *App) GetInterfaceSnapshot() (interfacemanager.Snapshot, error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	snapshot := manager.Snapshot()
	if snapshot.Sequence == 0 {
		return manager.Refresh()
	}
	return snapshot, nil
}

func (a *App) SelectInterfaces(interfaceAGUID, interfaceBGUID string) (interfacemanager.Snapshot, error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	snapshot := manager.Snapshot()
	if snapshot.Sequence == 0 {
		snapshot, err = manager.Refresh()
		if err != nil {
			return interfacemanager.Snapshot{}, err
		}
	}
	interfaceA, foundA := findAdapter(snapshot.Adapters, interfaceAGUID)
	interfaceB, foundB := findAdapter(snapshot.Adapters, interfaceBGUID)
	if !foundA || !foundB {
		return interfacemanager.Snapshot{}, interfaces.ErrAdapterNotFound
	}
	return manager.Select(interfaceA, interfaceB)
}

func (a *App) GetIPv6Policy() (string, error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return "", err
	}
	return manager.IPv6Policy(), nil
}

func (a *App) SetIPv6Policy(policy string) (interfacemanager.Snapshot, error) {
	status, err := a.GetCoreStatus()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	if status.State == "running" {
		return interfacemanager.Snapshot{}, errors.New("stop routing before changing the IPv6 policy")
	}
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	snapshot, err := manager.SetIPv6Policy(policy)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "interfaces", "IPv6 policy updated", "", map[string]any{"policy": policy})
	}
	return snapshot, err
}

func (a *App) getInterfaceManager() (*interfacemanager.Manager, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.interfaceError != nil {
		return nil, a.interfaceError
	}
	if a.interfaceManager == nil {
		return nil, errors.New("interface manager is not started")
	}
	return a.interfaceManager, nil
}

func (a *App) setInterfaceError(err error) {
	a.mu.Lock()
	a.interfaceError = err
	a.mu.Unlock()
}

func findAdapter(adapters []interfaces.Adapter, guid string) (interfaces.Adapter, bool) {
	wanted := strings.Trim(strings.TrimSpace(guid), "{}")
	for _, adapter := range adapters {
		if strings.EqualFold(strings.Trim(adapter.GUID, "{}"), wanted) {
			return adapter, true
		}
	}
	return interfaces.Adapter{}, false
}

func (a *App) ValidateCoreConfiguration(input config.MVPConfig) error {
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
	if statuses, inspectErr := a.InspectProcessRules(input); inspectErr != nil {
		return inspectErr
	} else {
		for _, status := range statuses {
			if status.State == "ambiguous" {
				return fmt.Errorf("process rule %q is ambiguous: use a full process path", status.Value)
			}
		}
	}
	input, err = a.withRemoteRules(input)
	if err != nil {
		return err
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
	if statuses, inspectErr := a.InspectProcessRules(input); inspectErr != nil {
		return core.Status{}, inspectErr
	} else {
		for _, status := range statuses {
			if status.State == "ambiguous" {
				return core.Status{}, fmt.Errorf("process rule %q is ambiguous: use a full process path", status.Value)
			}
		}
	}
	input, err = a.withRemoteRules(input)
	if err != nil {
		return core.Status{}, err
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
		if manager, managerErr := a.getRuleSetManager(); managerErr == nil {
			remote := manager.Get()
			if remote.Name != "" {
				items = append(items, ruleSetMetadata(remote))
			}
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

func (a *App) GetRemoteRuleSet() (rulesets.Source, error) {
	manager, err := a.getRuleSetManager()
	if err != nil {
		return rulesets.Source{}, err
	}
	return manager.Get(), nil
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

func (a *App) MigrateRemoteRuleSet() (rulesettings.Settings, error) {
	ruleSetManager, err := a.getRuleSetManager()
	if err != nil {
		return rulesettings.Settings{}, err
	}
	remote, err := ruleSetManager.Rules()
	if err != nil {
		return rulesettings.Settings{}, fmt.Errorf("load verified remote rules for migration: %w", err)
	}
	if len(remote) == 0 {
		return rulesettings.Settings{}, errors.New("no verified remote rules are available to migrate")
	}
	source := ruleSetManager.Get()
	prefix := source.AppliedSHA256
	if len(prefix) > 12 {
		prefix = prefix[:12]
	}
	imported := make([]rulesettings.Rule, 0, len(remote))
	for index, rule := range remote {
		imported = append(imported, rulesettings.Rule{
			ID:      fmt.Sprintf("legacy-remote-%s-%03d", prefix, index+1),
			Name:    fmt.Sprintf("Imported remote rule %d", index+1),
			Type:    rule.Type,
			Value:   rule.Value,
			Action:  rule.Action,
			Enabled: true,
		})
	}
	a.mu.RLock()
	settingsManager, loadErr := a.ruleSettings, a.ruleSettingsError
	a.mu.RUnlock()
	if loadErr != nil {
		return rulesettings.Settings{}, loadErr
	}
	if settingsManager == nil {
		return rulesettings.Settings{}, errors.New("rule settings manager is not ready")
	}
	settings, err := settingsManager.MigrateLegacyRemote(imported)
	if err != nil {
		return rulesettings.Settings{}, err
	}
	a.observations.Log(observability.LevelInfo, "rules", "Legacy remote rules migrated to unified settings", newCorrelationID(), map[string]any{"rule_count": len(imported), "source_version": source.Version})
	return settings, nil
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
		input.CustomRules = append(input.CustomRules, config.MVPCustomRule{ID: rule.ID, Name: rule.Name, Type: rule.Type, Value: rule.Value, Action: rule.Action})
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
	ctx, cancel := context.WithTimeout(a.ctx, 30*time.Second)
	defer cancel()
	updated, err := manager.Update(ctx, id)
	a.syncAllRuleSetMetadata()
	return updated, err
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

func (a *App) ConfigureRemoteRuleSet(source rulesets.Source) (rulesets.Source, error) {
	manager, err := a.getRuleSetManager()
	if err != nil {
		return rulesets.Source{}, err
	}
	configured, err := manager.Configure(source)
	if err != nil {
		return rulesets.Source{}, err
	}
	a.syncRuleSetMetadata(configured)
	return configured, nil
}

func (a *App) RefreshRemoteRuleSet() (rulesets.Source, error) {
	manager, err := a.getRuleSetManager()
	if err != nil {
		return rulesets.Source{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 20*time.Second)
	defer cancel()
	updated, err := manager.Update(ctx)
	a.syncRuleSetMetadata(updated)
	if err != nil {
		a.observations.Log(observability.LevelWarning, "rules", "Remote rule-set update rejected; last valid cache retained", newCorrelationID(), map[string]any{"error": err.Error()})
		return updated, err
	}
	a.observations.Log(observability.LevelInfo, "rules", "Remote rule-set verified and cached", "", map[string]any{"version": updated.Version, "rule_count": updated.RuleCount})
	return updated, nil
}

func (a *App) getRuleSetManager() (*rulesets.Manager, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.ruleSetError != nil {
		return nil, a.ruleSetError
	}
	if a.ruleSets == nil {
		return nil, errors.New("remote rule-set manager is not ready")
	}
	return a.ruleSets, nil
}

func (a *App) withRemoteRules(input config.MVPConfig) (config.MVPConfig, error) {
	settings, err := a.GetRuleSettings()
	if err != nil {
		return input, err
	}
	if settings.LegacyRemoteMigrated {
		return input, nil
	}
	manager, err := a.getRuleSetManager()
	if err != nil {
		return input, err
	}
	remote, err := manager.Rules()
	if err != nil {
		return input, fmt.Errorf("load verified remote rules: %w", err)
	}
	for index, rule := range remote {
		input.CustomRules = append(input.CustomRules, config.MVPCustomRule{Name: fmt.Sprintf("Remote rule %d", index+1), Type: rule.Type, Value: rule.Value, Action: rule.Action})
	}
	return input, nil
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
	if manager, err := a.getRuleSetManager(); err == nil {
		if source := manager.Get(); source.Name != "" {
			items = append(items, ruleSetMetadata(source))
		}
	}
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

func (a *App) syncRuleSetMetadata(source rulesets.Source) {
	if source.Name == "" {
		return
	}
	a.observations.SetRuleSets([]observability.RuleSetMetadata{ruleSetMetadata(source)})
}

func ruleSetMetadata(source rulesets.Source) observability.RuleSetMetadata {
	result := "configured"
	if source.AppliedSHA256 != "" {
		result = "validated-and-cached"
	}
	if source.LastError != "" {
		result = "update-failed-last-valid-retained"
	}
	return observability.RuleSetMetadata{Name: source.Name, Version: source.Version, SHA256: source.AppliedSHA256, Source: source.URL, RuleCount: source.RuleCount, Size: source.Size, LoadResult: result}
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

func (a *App) ResetInterfaceSelection() (interfacemanager.Snapshot, error) {
	if _, err := a.StopCore(); err != nil {
		return interfacemanager.Snapshot{}, fmt.Errorf("stop routing core: %w", err)
	}
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	snapshot, err := manager.ClearSelection()
	if err == nil {
		a.observations.Log(observability.LevelInfo, "application-reset", "Saved interface selection reset", "", nil)
	}
	return snapshot, err
}

func (a *App) ResetApplicationSettings() (string, error) {
	if _, err := a.StopCore(); err != nil {
		return "", fmt.Errorf("stop routing core: %w", err)
	}
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user configuration: %w", err)
	}
	configDir := filepath.Join(configRoot, "WinRouter")
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
	configRoot, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("locate user configuration: %w", err)
	}
	configDir := filepath.Join(configRoot, "WinRouter")
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

func trayStatusLabel(status recovery.Status) string {
	switch status.State {
	case recovery.StateMonitoring:
		if status.Desired {
			return "运行中"
		}
	case recovery.StateStopping:
		return "正在安全停止"
	case recovery.StateWaiting:
		return "等待网络"
	case recovery.StateRecovering:
		return "正在恢复"
	case recovery.StateFailed:
		return "恢复失败"
	}
	return "已停止"
}

type ObservationSnapshot struct {
	Logs                  []observability.LogEntry         `json:"logs"`
	Probes                []observability.ProbeResult      `json:"probes"`
	Counters              []observability.InterfaceCounter `json:"counters"`
	RuleSets              []observability.RuleSetMetadata  `json:"rule_sets"`
	Connections           observability.ConnectionSummary  `json:"connections"`
	RuleHits              []observability.RuleHit          `json:"rule_hits"`
	ConnectionObservation bool                             `json:"connection_observation"`
	ConnectionEvents      []clashapi.Summary               `json:"connection_events"`
	TrafficBudget         trafficbudget.Status             `json:"traffic_budget"`
}

func (a *App) GetObservations() (ObservationSnapshot, error) {
	snapshot, err := a.GetInterfaceSnapshot()
	if err != nil {
		return ObservationSnapshot{}, err
	}
	counters, err := observability.ReadInterfaceCounters(snapshot.Adapters)
	if err != nil {
		a.observations.Log(observability.LevelWarning, "interfaces", "Interface counter sampling failed", "", map[string]any{"error": err.Error()})
		return ObservationSnapshot{}, err
	}
	connections, err := observability.ReadConnectionSummary()
	if err != nil {
		a.observations.Log(observability.LevelWarning, "connections", "Connection sampling failed", "", map[string]any{"error": err.Error()})
		return ObservationSnapshot{}, err
	}
	coreStatus, _ := a.GetCoreStatus()
	budgetStatus := a.GetTrafficBudgetStatus()
	a.mu.RLock()
	budgetManager := a.trafficBudget
	a.mu.RUnlock()
	if budgetManager != nil && snapshot.InterfaceB.Match != nil {
		guid := snapshot.InterfaceB.Match.Adapter.GUID
		for _, counter := range counters {
			if strings.EqualFold(counter.GUID, guid) {
				var notifications []trafficbudget.Notification
				budgetStatus, notifications, err = budgetManager.Sample(counter.GUID, counter.Received, counter.Transmitted)
				if err != nil {
					a.observations.Log(observability.LevelWarning, "traffic-budget", "Traffic budget sample could not be saved", newCorrelationID(), map[string]any{"error": err.Error()})
				} else {
					for _, notification := range notifications {
						runtime.EventsEmit(a.ctx, "traffic-budget:reached", string(notification), budgetStatus)
						a.observations.Log(observability.LevelWarning, "traffic-budget", "Interface B traffic budget threshold reached", "", map[string]any{"threshold": notification, "used_percent": budgetStatus.UsedPercent})
					}
				}
				break
			}
		}
	}
	events := []clashapi.Summary(nil)
	if a.connectionObservation.Load() && coreStatus.State == "running" {
		a.mu.RLock()
		secret := a.connectionAPISecret
		a.mu.RUnlock()
		ctx, cancel := context.WithTimeout(a.ctx, 2*time.Second)
		result, connectionErr := (clashapi.Client{BaseURL: "http://127.0.0.1:19090", Secret: secret}).ListConnections(ctx)
		cancel()
		if connectionErr == nil {
			events = clashapi.Aggregate(result.Connections)
		}
	}
	return ObservationSnapshot{Logs: a.observations.Logs(), Probes: a.observations.Probes(), Counters: counters, RuleSets: a.observations.RuleSets(), Connections: connections, RuleHits: observability.ParseRuleHits(coreStatus.CoreLog), ConnectionObservation: a.connectionObservation.Load(), ConnectionEvents: events, TrafficBudget: budgetStatus}, nil
}

func (a *App) SetConnectionObservationEnabled(enabled bool) error {
	status, err := a.GetCoreStatus()
	if err != nil {
		return err
	}
	if status.State == "running" {
		return errors.New("stop routing before changing connection observation")
	}
	if enabled {
		a.mu.Lock()
		if a.connectionAPISecret == "" {
			data := make([]byte, 32)
			if _, err := rand.Read(data); err != nil {
				a.mu.Unlock()
				return err
			}
			a.connectionAPISecret = hex.EncodeToString(data)
		}
		a.mu.Unlock()
	}
	a.connectionObservation.Store(enabled)
	return nil
}

func (a *App) GetConnectionObservationEnabled() bool { return a.connectionObservation.Load() }

func (a *App) applyConnectionObservation(input *config.MVPConfig) {
	input.ConnectionObservation = a.connectionObservation.Load()
	if input.ConnectionObservation {
		a.mu.RLock()
		input.ConnectionAPISecret = a.connectionAPISecret
		a.mu.RUnlock()
	} else {
		input.ConnectionAPISecret = ""
	}
}

func (a *App) GetTrafficBudgetStatus() trafficbudget.Status {
	a.mu.RLock()
	manager, loadErr := a.trafficBudget, a.trafficBudgetError
	a.mu.RUnlock()
	if manager == nil || loadErr != nil {
		return trafficbudget.Status{Settings: trafficbudget.Settings{BudgetGB: 100, WarningPercent: 80}}
	}
	return manager.Get()
}

func (a *App) SetTrafficBudget(settings trafficbudget.Settings) (trafficbudget.Status, error) {
	a.mu.RLock()
	manager, loadErr := a.trafficBudget, a.trafficBudgetError
	a.mu.RUnlock()
	if loadErr != nil {
		return trafficbudget.Status{}, loadErr
	}
	if manager == nil {
		return trafficbudget.Status{}, errors.New("traffic budget is not initialized")
	}
	status, err := manager.Configure(settings)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "traffic-budget", "Traffic budget setting updated", "", map[string]any{"enabled": settings.Enabled, "budget_gb": settings.BudgetGB, "warning_percent": settings.WarningPercent})
	}
	return status, err
}

func (a *App) ResetTrafficBudget() (trafficbudget.Status, error) {
	a.mu.RLock()
	manager, loadErr := a.trafficBudget, a.trafficBudgetError
	a.mu.RUnlock()
	if loadErr != nil {
		return trafficbudget.Status{}, loadErr
	}
	if manager == nil {
		return trafficbudget.Status{}, errors.New("traffic budget is not initialized")
	}
	status, err := manager.Reset()
	if err == nil {
		a.observations.Log(observability.LevelInfo, "traffic-budget", "Traffic budget usage reset", "", nil)
	}
	return status, err
}

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
	return input, nil
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

func newCorrelationID() string { return fmt.Sprintf("WR-%X", time.Now().UnixNano()) }

func locateBundledCore() (string, error) {
	executable, err := os.Executable()
	if err != nil {
		return "", err
	}
	candidates := []string{
		filepath.Join(filepath.Dir(executable), "resources", "core", "sing-box.exe"),
		filepath.Join("resources", "core", "sing-box.exe"),
	}
	for _, candidate := range candidates {
		absolute, err := filepath.Abs(candidate)
		if err != nil {
			continue
		}
		if info, err := os.Stat(absolute); err == nil && !info.IsDir() {
			return absolute, nil
		}
	}
	return "", errors.New("locate bundled sing-box for SRS validation")
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

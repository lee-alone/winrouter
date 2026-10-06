package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"winrouter/internal/clashapi"
	"winrouter/internal/config"
	"winrouter/internal/observability"
	"winrouter/internal/trafficbudget"
)

type RealtimeMetrics struct {
	Counters              []observability.InterfaceCounter `json:"counters"`
	Connections           observability.ConnectionSummary  `json:"connections"`
	TrafficBudget         trafficbudget.Status             `json:"traffic_budget"`
	ConnectionObservation bool                             `json:"connection_observation"`
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

func (a *App) GetRealtimeMetrics() (RealtimeMetrics, error) {
	snapshot, err := a.GetInterfaceSnapshot()
	if err != nil {
		return RealtimeMetrics{}, err
	}
	counters, err := observability.ReadInterfaceCounters(snapshot.Adapters)
	if err != nil {
		a.observations.Log(observability.LevelWarning, "interfaces", "Interface counter sampling failed", "", map[string]any{"error": err.Error()})
		return RealtimeMetrics{}, err
	}
	connections, err := observability.ReadConnectionSummary()
	if err != nil {
		a.observations.Log(observability.LevelWarning, "connections", "Connection sampling failed", "", map[string]any{"error": err.Error()})
		return RealtimeMetrics{}, err
	}
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
	return RealtimeMetrics{
		Counters:              counters,
		Connections:           connections,
		TrafficBudget:         budgetStatus,
		ConnectionObservation: a.connectionObservation.Load(),
	}, nil
}

func (a *App) GetObservations() (ObservationSnapshot, error) {
	metrics, err := a.GetRealtimeMetrics()
	if err != nil {
		return ObservationSnapshot{}, err
	}
	coreStatus, _ := a.GetCoreStatus()
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
	return ObservationSnapshot{
		Logs:                  a.observations.Logs(),
		Probes:                a.observations.Probes(),
		Counters:              metrics.Counters,
		RuleSets:              a.observations.RuleSets(),
		Connections:           metrics.Connections,
		RuleHits:              observability.ParseRuleHits(coreStatus.CoreLog),
		ConnectionObservation: metrics.ConnectionObservation,
		ConnectionEvents:      events,
		TrafficBudget:         metrics.TrafficBudget,
	}, nil
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

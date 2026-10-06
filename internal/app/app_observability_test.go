package app

import (
	"path/filepath"
	"testing"

	"winrouter/internal/interfacemanager"
	"winrouter/internal/trafficbudget"
)

func TestGetRealtimeMetrics(t *testing.T) {
	dir := t.TempDir()
	mgr, err := interfacemanager.New(interfacemanager.Options{
		StatePath: filepath.Join(dir, "interfaces.json"),
	})
	if err != nil {
		t.Fatalf("failed to create interface manager: %v", err)
	}
	budget, err := trafficbudget.New(filepath.Join(dir, "traffic-budget.json"))
	if err != nil {
		t.Fatalf("failed to create traffic budget: %v", err)
	}

	app := New()
	app.interfaceManager = mgr
	app.trafficBudget = budget

	metrics, err := app.GetRealtimeMetrics()
	if err != nil {
		t.Fatalf("GetRealtimeMetrics returned error: %v", err)
	}

	if metrics.ConnectionObservation != false {
		t.Fatalf("expected ConnectionObservation false, got true")
	}

	// Verify GetObservations works and reuses RealtimeMetrics
	obs, err := app.GetObservations()
	if err != nil {
		t.Fatalf("GetObservations returned error: %v", err)
	}
	if obs.ConnectionObservation != false {
		t.Fatalf("expected ConnectionObservation false, got true")
	}
}

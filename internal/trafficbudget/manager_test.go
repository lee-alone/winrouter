package trafficbudget

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSampleTracksDeltasAndNotifiesOnce(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "budget.json"))
	if err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC) }
	if _, err = m.Configure(Settings{Enabled: true, BudgetGB: 1, WarningPercent: 50}); err != nil {
		t.Fatal(err)
	}
	if _, notes, err := m.Sample("b", 100, 200); err != nil || len(notes) != 0 {
		t.Fatalf("baseline: notes=%v err=%v", notes, err)
	}
	status, notes, err := m.Sample("b", 600*1024*1024+100, 200)
	if err != nil {
		t.Fatal(err)
	}
	if !status.WarningReached || status.LimitReached || len(notes) != 1 || notes[0] != NotificationWarning {
		t.Fatalf("unexpected warning status: %+v notes=%v", status, notes)
	}
	_, notes, err = m.Sample("b", 700*1024*1024+100, 200)
	if err != nil || len(notes) != 0 {
		t.Fatalf("warning repeated: notes=%v err=%v", notes, err)
	}
}

func TestCounterResetAndInterfaceChangeDoNotAddUsage(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "budget.json"))
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = m.Sample("b", 1000, 1000)
	before, _, _ := m.Sample("b", 2000, 3000)
	after, _, err := m.Sample("other", 900000, 900000)
	if err != nil {
		t.Fatal(err)
	}
	if after.UsedBytes != before.UsedBytes {
		t.Fatalf("interface change added usage: %d -> %d", before.UsedBytes, after.UsedBytes)
	}
	reset, _, _ := m.Sample("other", 1, 1)
	if reset.UsedBytes != before.UsedBytes {
		t.Fatalf("counter reset changed usage: %d -> %d", before.UsedBytes, reset.UsedBytes)
	}
}

func TestConfigureValidation(t *testing.T) {
	m, err := New(filepath.Join(t.TempDir(), "budget.json"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.Configure(Settings{Enabled: true, BudgetGB: 0, WarningPercent: 80}); err == nil {
		t.Fatal("expected budget validation error")
	}
	if _, err := m.Configure(Settings{Enabled: true, BudgetGB: 10, WarningPercent: 101}); err == nil {
		t.Fatal("expected percentage validation error")
	}
}

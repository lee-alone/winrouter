package trafficbudget

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const schemaVersion = 1

type Settings struct {
	Enabled        bool    `json:"enabled"`
	BudgetGB       float64 `json:"budget_gb"`
	WarningPercent int     `json:"warning_percent"`
}

type Status struct {
	Settings
	Period         string  `json:"period"`
	UsedBytes      uint64  `json:"used_bytes"`
	BudgetBytes    uint64  `json:"budget_bytes"`
	UsedPercent    float64 `json:"used_percent"`
	WarningReached bool    `json:"warning_reached"`
	LimitReached   bool    `json:"limit_reached"`
	InterfaceGUID  string  `json:"interface_guid,omitempty"`
}

type state struct {
	SchemaVersion   int      `json:"schema_version"`
	Settings        Settings `json:"settings"`
	Period          string   `json:"period"`
	UsedBytes       uint64   `json:"used_bytes"`
	InterfaceGUID   string   `json:"interface_guid,omitempty"`
	LastReceived    uint64   `json:"last_received,omitempty"`
	LastTransmitted uint64   `json:"last_transmitted,omitempty"`
	WarningNotified bool     `json:"warning_notified,omitempty"`
	LimitNotified   bool     `json:"limit_notified,omitempty"`
}

type Notification string

const (
	NotificationWarning Notification = "warning"
	NotificationLimit   Notification = "limit"
)

type Manager struct {
	mu        sync.Mutex
	path      string
	now       func() time.Time
	lastSaved time.Time
	data      state
}

func New(path string) (*Manager, error) {
	m := &Manager{path: path, now: time.Now, data: state{SchemaVersion: schemaVersion, Settings: Settings{BudgetGB: 100, WarningPercent: 80}}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		m.resetPeriodLocked()
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read traffic budget: %w", err)
	}
	if err := json.Unmarshal(data, &m.data); err != nil {
		return nil, fmt.Errorf("decode traffic budget: %w", err)
	}
	if m.data.SchemaVersion != schemaVersion {
		return nil, fmt.Errorf("unsupported traffic budget schema %d", m.data.SchemaVersion)
	}
	if err := validate(m.data.Settings); err != nil {
		return nil, err
	}
	m.rollPeriodLocked()
	return m, nil
}

func validate(settings Settings) error {
	if settings.BudgetGB < 0.1 || settings.BudgetGB > 100000 {
		return errors.New("traffic budget must be between 0.1 and 100000 GB")
	}
	if settings.WarningPercent < 1 || settings.WarningPercent > 100 {
		return errors.New("traffic budget warning must be between 1 and 100 percent")
	}
	return nil
}

func (m *Manager) Configure(settings Settings) (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if err := validate(settings); err != nil {
		return Status{}, err
	}
	m.rollPeriodLocked()
	m.data.Settings = settings
	m.data.WarningNotified = false
	m.data.LimitNotified = false
	if err := m.saveLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(), nil
}

func (m *Manager) Reset() (Status, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resetPeriodLocked()
	if err := m.saveLocked(); err != nil {
		return Status{}, err
	}
	return m.statusLocked(), nil
}

func (m *Manager) Get() Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.rollPeriodLocked() {
		_ = m.saveLocked()
	}
	return m.statusLocked()
}

func (m *Manager) Sample(guid string, received, transmitted uint64) (Status, []Notification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	changed := m.rollPeriodLocked()
	if guid != m.data.InterfaceGUID {
		m.data.InterfaceGUID, m.data.LastReceived, m.data.LastTransmitted = guid, received, transmitted
		changed = true
	} else {
		if received >= m.data.LastReceived {
			m.data.UsedBytes += received - m.data.LastReceived
		}
		if transmitted >= m.data.LastTransmitted {
			m.data.UsedBytes += transmitted - m.data.LastTransmitted
		}
		m.data.LastReceived, m.data.LastTransmitted = received, transmitted
		changed = true
	}
	status := m.statusLocked()
	var notifications []Notification
	if status.Settings.Enabled && status.WarningReached && !m.data.WarningNotified {
		m.data.WarningNotified = true
		notifications = append(notifications, NotificationWarning)
	}
	if status.Settings.Enabled && status.LimitReached && !m.data.LimitNotified {
		m.data.LimitNotified = true
		notifications = append(notifications, NotificationLimit)
	}
	if changed && (len(notifications) > 0 || m.lastSaved.IsZero() || m.now().Sub(m.lastSaved) >= time.Minute) {
		if err := m.saveLocked(); err != nil {
			return Status{}, nil, err
		}
	}
	return status, notifications, nil
}

func (m *Manager) rollPeriodLocked() bool {
	period := m.now().Format("2006-01")
	if m.data.Period == period {
		return false
	}
	m.resetPeriodLocked()
	return true
}

func (m *Manager) resetPeriodLocked() {
	m.data.Period = m.now().Format("2006-01")
	m.data.UsedBytes = 0
	m.data.InterfaceGUID = ""
	m.data.LastReceived, m.data.LastTransmitted = 0, 0
	m.data.WarningNotified, m.data.LimitNotified = false, false
}

func (m *Manager) statusLocked() Status {
	budget := uint64(m.data.Settings.BudgetGB * 1024 * 1024 * 1024)
	percent := float64(0)
	if budget > 0 {
		percent = float64(m.data.UsedBytes) * 100 / float64(budget)
	}
	return Status{Settings: m.data.Settings, Period: m.data.Period, UsedBytes: m.data.UsedBytes, BudgetBytes: budget, UsedPercent: percent, WarningReached: percent >= float64(m.data.Settings.WarningPercent), LimitReached: percent >= 100, InterfaceGUID: m.data.InterfaceGUID}
}

func (m *Manager) saveLocked() error {
	data, err := json.MarshalIndent(m.data, "", "  ")
	if err != nil {
		return fmt.Errorf("encode traffic budget: %w", err)
	}
	if err := os.MkdirAll(filepath.Dir(m.path), 0o700); err != nil {
		return fmt.Errorf("create traffic budget directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(m.path), ".traffic-budget-*.tmp")
	if err != nil {
		return fmt.Errorf("create traffic budget temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err == nil {
		_, err = temporary.Write(append(data, '\n'))
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return fmt.Errorf("write traffic budget: %w", err)
	}
	if err := replaceFile(temporaryPath, m.path); err != nil {
		return fmt.Errorf("replace traffic budget: %w", err)
	}
	m.lastSaved = m.now()
	return nil
}

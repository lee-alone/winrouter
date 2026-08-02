package observability

import (
	"sync"
	"time"
)

type Store struct {
	mu       sync.RWMutex
	limit    int
	logs     []LogEntry
	probes   []ProbeResult
	ruleSets []RuleSetMetadata
}

func NewStore(limit int) *Store {
	if limit < 1 {
		limit = 200
	}
	return &Store{limit: limit, logs: make([]LogEntry, 0), probes: make([]ProbeResult, 0), ruleSets: make([]RuleSetMetadata, 0)}
}

func (s *Store) Log(level Level, component, message, correlation string, fields map[string]any) LogEntry {
	entry := LogEntry{Time: time.Now().UTC(), Level: level, Component: component, Message: message, CorrelationID: correlation, Fields: fields}
	s.mu.Lock()
	s.logs = append(s.logs, entry)
	if len(s.logs) > s.limit {
		s.logs = append([]LogEntry(nil), s.logs[len(s.logs)-s.limit:]...)
	}
	s.mu.Unlock()
	return entry
}

func (s *Store) Logs() []LogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]LogEntry, len(s.logs))
	copy(result, s.logs)
	return result
}
func (s *Store) AddProbe(value ProbeResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.probes = append(s.probes, value)
	if len(s.probes) > s.limit {
		s.probes = append([]ProbeResult(nil), s.probes[len(s.probes)-s.limit:]...)
	}
}
func (s *Store) Probes() []ProbeResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]ProbeResult, len(s.probes))
	copy(result, s.probes)
	return result
}
func (s *Store) SetRuleSets(values []RuleSetMetadata) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ruleSets = append([]RuleSetMetadata(nil), values...)
}
func (s *Store) RuleSets() []RuleSetMetadata {
	s.mu.RLock()
	defer s.mu.RUnlock()
	result := make([]RuleSetMetadata, len(s.ruleSets))
	copy(result, s.ruleSets)
	return result
}

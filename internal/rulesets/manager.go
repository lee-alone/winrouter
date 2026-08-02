package rulesets

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxDocumentSize = 1 << 20

type Rule struct {
	Type   string `json:"type"`
	Value  string `json:"value"`
	Action string `json:"action"`
}

type Document struct {
	Version string `json:"version"`
	Rules   []Rule `json:"rules"`
}

type Source struct {
	Name           string    `json:"name"`
	URL            string    `json:"url"`
	ExpectedSHA256 string    `json:"expected_sha256"`
	AppliedSHA256  string    `json:"applied_sha256,omitempty"`
	Version        string    `json:"version,omitempty"`
	RuleCount      int       `json:"rule_count"`
	Size           int64     `json:"size"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
}

type state struct {
	SchemaVersion int    `json:"schema_version"`
	Source        Source `json:"source"`
}

type Manager struct {
	mu     sync.Mutex
	path   string
	client *http.Client
	state  state
}

func New(path string) (*Manager, error) {
	return newManager(path, &http.Client{Timeout: 15 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("rule-set redirects are disabled")
	}})
}

func newManager(path string, client *http.Client) (*Manager, error) {
	if strings.TrimSpace(path) == "" || client == nil {
		return nil, errors.New("rule-set path and HTTP client are required")
	}
	m := &Manager{path: path, client: client, state: state{SchemaVersion: 1}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read rule-set state: %w", err)
	}
	if err := json.Unmarshal(data, &m.state); err != nil {
		return nil, fmt.Errorf("decode rule-set state: %w", err)
	}
	if m.state.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported rule-set schema %d", m.state.SchemaVersion)
	}
	if m.state.Source.Name != "" {
		if err := validateSource(m.state.Source); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *Manager) Get() Source { m.mu.Lock(); defer m.mu.Unlock(); return m.state.Source }

func (m *Manager) Configure(source Source) (Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	source.Name, source.URL, source.ExpectedSHA256 = strings.TrimSpace(source.Name), strings.TrimSpace(source.URL), strings.ToLower(strings.TrimSpace(source.ExpectedSHA256))
	if err := validateSource(source); err != nil {
		return Source{}, err
	}
	// Configuring the next candidate must not deactivate the last verified cache.
	source.AppliedSHA256, source.Version, source.RuleCount, source.Size, source.UpdatedAt = m.state.Source.AppliedSHA256, m.state.Source.Version, m.state.Source.RuleCount, m.state.Source.Size, m.state.Source.UpdatedAt
	next := state{SchemaVersion: 1, Source: source}
	if err := m.save(next); err != nil {
		return Source{}, err
	}
	m.state = next
	return source, nil
}

func (m *Manager) Update(ctx context.Context) (Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Source.Name == "" {
		return Source{}, errors.New("no remote rule-set configured")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.state.Source.URL, nil)
	if err != nil {
		return Source{}, err
	}
	req.Header.Set("Accept", "application/json")
	response, err := m.client.Do(req)
	if err != nil {
		return m.fail("download failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return m.fail(fmt.Sprintf("HTTP %d", response.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxDocumentSize+1))
	if err != nil || len(data) > maxDocumentSize {
		return m.fail("invalid response size")
	}
	digest := sha256.Sum256(data)
	actual := hex.EncodeToString(digest[:])
	if actual != m.state.Source.ExpectedSHA256 {
		return m.fail("SHA-256 mismatch")
	}
	document, err := parseDocument(data)
	if err != nil {
		return m.fail("document validation failed")
	}
	cachePath := m.cachePath()
	if err := writeAtomic(cachePath, data); err != nil {
		return Source{}, err
	}
	next := m.state
	next.Source.AppliedSHA256, next.Source.Version = actual, document.Version
	next.Source.RuleCount, next.Source.Size, next.Source.UpdatedAt, next.Source.LastError = len(document.Rules), int64(len(data)), time.Now().UTC(), ""
	if err := m.save(next); err != nil {
		return Source{}, err
	}
	m.state = next
	return next.Source, nil
}

func (m *Manager) Rules() ([]Rule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.state.Source.AppliedSHA256 == "" {
		return nil, nil
	}
	data, err := os.ReadFile(m.cachePath())
	if err != nil {
		return nil, err
	}
	digest := sha256.Sum256(data)
	if hex.EncodeToString(digest[:]) != m.state.Source.AppliedSHA256 {
		return nil, errors.New("cached rule-set hash mismatch")
	}
	document, err := parseDocument(data)
	if err != nil {
		return nil, err
	}
	return document.Rules, nil
}

func validateSource(source Source) error {
	if source.Name == "" || len([]rune(source.Name)) > 80 {
		return errors.New("rule-set name is required and must not exceed 80 characters")
	}
	parsed, err := url.Parse(source.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return errors.New("rule-set URL must be fixed HTTPS without userinfo, query, or fragment")
	}
	decoded, err := hex.DecodeString(source.ExpectedSHA256)
	if err != nil || len(decoded) != sha256.Size {
		return errors.New("expected SHA-256 must be 64 hexadecimal characters")
	}
	return nil
}

func parseDocument(data []byte) (Document, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var document Document
	if err := decoder.Decode(&document); err != nil {
		return Document{}, err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return Document{}, errors.New("trailing JSON value")
	}
	if strings.TrimSpace(document.Version) == "" || len(document.Rules) == 0 || len(document.Rules) > 10000 {
		return Document{}, errors.New("version and 1..10000 rules are required")
	}
	for _, rule := range document.Rules {
		if (rule.Type != "domain" && rule.Type != "ip") || strings.TrimSpace(rule.Value) == "" {
			return Document{}, errors.New("invalid rule match")
		}
		switch rule.Action {
		case "a", "b", "final", "reject":
		default:
			return Document{}, errors.New("invalid rule action")
		}
	}
	return document, nil
}

func (m *Manager) fail(message string) (Source, error) {
	next := m.state
	next.Source.LastError = message
	if err := m.save(next); err == nil {
		m.state = next
	}
	return next.Source, errors.New(message)
}

func (m *Manager) cachePath() string {
	return strings.TrimSuffix(m.path, filepath.Ext(m.path)) + ".cache.json"
}
func (m *Manager) save(next state) error {
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(m.path, append(data, '\n'))
}
func writeAtomic(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".ruleset-*.tmp")
	if err != nil {
		return err
	}
	name := file.Name()
	defer os.Remove(name)
	_ = file.Chmod(0o600)
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	return replaceFile(name, path)
}

package srssets

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const maxSRSSize = 32 << 20

type Preset struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Kind     string `json:"kind"`
	URL      string `json:"url"`
	Upstream string `json:"upstream"`
	License  string `json:"license"`
}

var presets = []Preset{
	{ID: "sagernet-geosite-cn", Name: "中国域名 (geosite-cn)", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cn.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geoip-cn", Name: "中国 IP (geoip-cn)", Kind: "ip", URL: "https://raw.githubusercontent.com/SagerNet/sing-geoip/rule-set/geoip-cn.srs", Upstream: "SagerNet/sing-geoip", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-github", Name: "GitHub", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-github.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
	{ID: "sagernet-geosite-cloudflare", Name: "Cloudflare", Kind: "domain", URL: "https://raw.githubusercontent.com/SagerNet/sing-geosite/rule-set/geosite-cloudflare.srs", Upstream: "SagerNet/sing-geosite", License: "GPL-3.0; data licenses follow upstream"},
}

type Source struct {
	ID             string    `json:"id"`
	Name           string    `json:"name"`
	Kind           string    `json:"kind"`
	PresetID       string    `json:"preset_id,omitempty"`
	URL            string    `json:"url"`
	ExpectedSHA256 string    `json:"expected_sha256,omitempty"`
	AppliedSHA256  string    `json:"applied_sha256,omitempty"`
	Size           int64     `json:"size"`
	UpdatedAt      time.Time `json:"updated_at,omitempty"`
	LastError      string    `json:"last_error,omitempty"`
	Enabled        bool      `json:"enabled"`
	Action         string    `json:"action"`
	Upstream       string    `json:"upstream,omitempty"`
	License        string    `json:"license,omitempty"`
}

type Active struct {
	Tag    string
	Kind   string
	Action string
	Path   string
}

type state struct {
	SchemaVersion int      `json:"schema_version"`
	Sources       []Source `json:"sources"`
}

type Validator func(context.Context, string) error

type Manager struct {
	mu        sync.Mutex
	path      string
	client    *http.Client
	validator Validator
	state     state
}

func Presets() []Preset { return append([]Preset(nil), presets...) }

func New(path, singBoxPath string) (*Manager, error) {
	validator := func(ctx context.Context, sourcePath string) error {
		outputPath := sourcePath + ".json"
		defer os.Remove(outputPath)
		output, err := exec.CommandContext(ctx, singBoxPath, "rule-set", "decompile", "-o", outputPath, sourcePath).CombinedOutput()
		if err != nil {
			return fmt.Errorf("sing-box rule-set validation: %w: %s", err, strings.TrimSpace(string(output)))
		}
		return nil
	}
	return newManager(path, validator, &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error {
		return errors.New("SRS redirects are disabled")
	}})
}

func newManager(path string, validator Validator, client *http.Client) (*Manager, error) {
	if strings.TrimSpace(path) == "" || validator == nil || client == nil {
		return nil, errors.New("SRS state path, validator, and HTTP client are required")
	}
	m := &Manager{path: path, validator: validator, client: client, state: state{SchemaVersion: 1, Sources: defaultSources()}}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := m.save(m.state); err != nil {
			return nil, err
		}
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read SRS state: %w", err)
	}
	if err := json.Unmarshal(data, &m.state); err != nil {
		return nil, fmt.Errorf("decode SRS state: %w", err)
	}
	if m.state.SchemaVersion != 1 {
		return nil, fmt.Errorf("unsupported SRS schema %d", m.state.SchemaVersion)
	}
	// Add newly shipped presets without changing existing user choices or
	// cached data. This makes upgrades additive and preserves custom policy.
	addedPreset := false
	for _, preset := range defaultSources() {
		found := false
		for _, source := range m.state.Sources {
			if source.ID == preset.ID {
				found = true
				break
			}
		}
		if !found {
			m.state.Sources = append(m.state.Sources, preset)
			addedPreset = true
		}
	}
	if addedPreset {
		if err := m.save(m.state); err != nil {
			return nil, fmt.Errorf("save SRS defaults: %w", err)
		}
	}
	for _, source := range m.state.Sources {
		if err := validateSource(source); err != nil {
			return nil, fmt.Errorf("saved SRS source %q: %w", source.Name, err)
		}
	}
	return m, nil
}

func defaultSources() []Source {
	result := make([]Source, 0, len(presets))
	for _, preset := range presets {
		action := "a"
		if preset.ID == "sagernet-geosite-github" || preset.ID == "sagernet-geosite-cloudflare" {
			action = "b"
		}
		result = append(result, Source{ID: preset.ID, Name: preset.Name, Kind: preset.Kind, PresetID: preset.ID, URL: preset.URL, Enabled: true, Action: action, Upstream: preset.Upstream, License: preset.License})
	}
	return result
}

func (m *Manager) List() []Source {
	m.mu.Lock()
	defer m.mu.Unlock()
	return append([]Source(nil), m.state.Sources...)
}

func (m *Manager) Configure(source Source) (Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	source.Name, source.URL = strings.TrimSpace(source.Name), strings.TrimSpace(source.URL)
	source.ExpectedSHA256 = strings.ToLower(strings.TrimSpace(source.ExpectedSHA256))
	if preset, ok := presetByID(source.PresetID); ok {
		source.Name, source.Kind, source.URL = preset.Name, preset.Kind, preset.URL
		source.Upstream, source.License = preset.Upstream, preset.License
		if source.ID == "" {
			source.ID = preset.ID
		}
	}
	if source.ID == "" {
		source.ID = newID()
	}
	if err := validateSource(source); err != nil {
		return Source{}, err
	}
	next := state{SchemaVersion: 1, Sources: append([]Source(nil), m.state.Sources...)}
	replaced := false
	for index, current := range next.Sources {
		if current.ID != source.ID {
			continue
		}
		if current.URL == source.URL && current.ExpectedSHA256 == source.ExpectedSHA256 && current.Kind == source.Kind {
			source.AppliedSHA256, source.Size, source.UpdatedAt = current.AppliedSHA256, current.Size, current.UpdatedAt
		}
		next.Sources[index], replaced = source, true
		break
	}
	if !replaced {
		next.Sources = append(next.Sources, source)
	}
	if err := m.save(next); err != nil {
		return Source{}, err
	}
	m.state = next
	return source, nil
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	next := state{SchemaVersion: 1, Sources: make([]Source, 0, len(m.state.Sources))}
	found := false
	for _, source := range m.state.Sources {
		if source.ID == id {
			found = true
			if source.PresetID != "" {
				return errors.New("built-in SRS presets cannot be deleted")
			}
			continue
		}
		next.Sources = append(next.Sources, source)
	}
	if !found {
		return errors.New("SRS source not found")
	}
	if err := m.save(next); err != nil {
		return err
	}
	m.state = next
	if err := os.Remove(m.cachePath(id)); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (m *Manager) Update(ctx context.Context, id string) (Source, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	index := m.index(id)
	if index < 0 {
		return Source{}, errors.New("SRS source not found")
	}
	source := m.state.Sources[index]
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source.URL, nil)
	if err != nil {
		return Source{}, err
	}
	req.Header.Set("Accept", "application/octet-stream")
	response, err := m.client.Do(req)
	if err != nil {
		return m.fail(index, "download failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return m.fail(index, fmt.Sprintf("HTTP %d", response.StatusCode))
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, maxSRSSize+1))
	if err != nil || len(data) > maxSRSSize {
		return m.fail(index, "invalid response size")
	}
	digest := sha256.Sum256(data)
	actual := hex.EncodeToString(digest[:])
	if source.ExpectedSHA256 != "" && actual != source.ExpectedSHA256 {
		return m.fail(index, "SHA-256 mismatch")
	}
	if len(data) < 3 || string(data[:3]) != "SRS" {
		return m.fail(index, "invalid SRS header")
	}
	temporary, err := os.CreateTemp(filepath.Dir(m.path), ".srs-*.tmp")
	if err != nil {
		return Source{}, err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if _, err = temporary.Write(data); err == nil {
		err = temporary.Sync()
	}
	if closeErr := temporary.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		return Source{}, err
	}
	if err := m.validator(ctx, temporaryPath); err != nil {
		return m.fail(index, err.Error())
	}
	cachePath := m.cachePath(id)
	if err := replaceFile(temporaryPath, cachePath); err != nil {
		return Source{}, err
	}
	next := m.state
	next.Sources = append([]Source(nil), m.state.Sources...)
	next.Sources[index].AppliedSHA256, next.Sources[index].Size = actual, int64(len(data))
	next.Sources[index].UpdatedAt, next.Sources[index].LastError = time.Now().UTC(), ""
	if err := m.save(next); err != nil {
		return Source{}, err
	}
	m.state = next
	return next.Sources[index], nil
}

func (m *Manager) Active() ([]Active, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]Active, 0, len(m.state.Sources))
	for _, source := range m.state.Sources {
		if !source.Enabled || source.AppliedSHA256 == "" {
			continue
		}
		path := m.cachePath(source.ID)
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		digest := sha256.Sum256(data)
		if hex.EncodeToString(digest[:]) != source.AppliedSHA256 {
			return nil, fmt.Errorf("cached SRS %q hash mismatch", source.Name)
		}
		result = append(result, Active{Tag: "winrouter-" + source.ID, Kind: source.Kind, Action: source.Action, Path: path})
	}
	return result, nil
}

func validateSource(source Source) error {
	if !regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`).MatchString(source.ID) {
		return errors.New("SRS id must contain only letters, numbers, underscore, or hyphen")
	}
	if source.Name == "" || len([]rune(source.Name)) > 80 {
		return errors.New("SRS name is required and must not exceed 80 characters")
	}
	if source.Kind != "domain" && source.Kind != "ip" {
		return errors.New("SRS kind must be domain or ip")
	}
	if source.Action != "a" && source.Action != "b" && source.Action != "c" && source.Action != "final" && source.Action != "reject" {
		return errors.New("unsupported SRS action")
	}
	parsed, err := url.Parse(source.URL)
	if err != nil || parsed.Scheme != "https" || parsed.Hostname() == "" || parsed.User != nil || parsed.Fragment != "" || parsed.RawQuery != "" {
		return errors.New("SRS URL must be fixed HTTPS without userinfo, query, or fragment")
	}
	if source.ExpectedSHA256 != "" {
		decoded, err := hex.DecodeString(source.ExpectedSHA256)
		if err != nil || len(decoded) != sha256.Size {
			return errors.New("expected SHA-256 must be 64 hexadecimal characters")
		}
	}
	return nil
}

func presetByID(id string) (Preset, bool) {
	for _, preset := range presets {
		if preset.ID == id {
			return preset, true
		}
	}
	return Preset{}, false
}

func (m *Manager) index(id string) int {
	for index, source := range m.state.Sources {
		if source.ID == id {
			return index
		}
	}
	return -1
}

func (m *Manager) fail(index int, message string) (Source, error) {
	next := m.state
	next.Sources = append([]Source(nil), m.state.Sources...)
	next.Sources[index].LastError = message
	if err := m.save(next); err == nil {
		m.state = next
	}
	return next.Sources[index], errors.New(message)
}

func (m *Manager) cachePath(id string) string {
	return filepath.Join(filepath.Dir(m.path), "srs", id+".srs")
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
	file, err := os.CreateTemp(filepath.Dir(path), ".srs-state-*.tmp")
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

func newID() string {
	var value [8]byte
	if _, err := rand.Read(value[:]); err != nil {
		return fmt.Sprintf("custom-%d", time.Now().UnixNano())
	}
	return "custom-" + hex.EncodeToString(value[:])
}

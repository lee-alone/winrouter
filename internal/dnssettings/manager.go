package dnssettings

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const SchemaVersion = 1

type Server struct {
	PresetID   string `json:"preset_id,omitempty"`
	Type       string `json:"type"`
	Server     string `json:"server"`
	Port       uint16 `json:"port"`
	ServerName string `json:"server_name,omitempty"`
}

type Settings struct {
	SchemaVersion int     `json:"schema_version"`
	Domestic      Server  `json:"domestic"`
	Global        Server  `json:"global"`
	Proxy         *Server `json:"proxy,omitempty"`
}

type Preset struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Scope      string `json:"scope"`
	Type       string `json:"type"`
	Server     string `json:"server"`
	Port       uint16 `json:"port"`
	ServerName string `json:"server_name,omitempty"`
}

var presets = []Preset{
	// 国内主流加密 DNS（阿里 & 腾讯）
	{ID: "aliyun-dot", Name: "AliDNS DoT", Scope: "domestic", Type: "tls", Server: "223.5.5.5", Port: 853, ServerName: "dns.alidns.com"},
	{ID: "aliyun-doh", Name: "AliDNS DoH", Scope: "domestic", Type: "https", Server: "223.5.5.5", Port: 443, ServerName: "dns.alidns.com"},
	{ID: "tencent-dot", Name: "DNSPod DoT", Scope: "domestic", Type: "tls", Server: "1.12.12.12", Port: 853, ServerName: "dot.pub"},
	{ID: "tencent-doh", Name: "DNSPod DoH", Scope: "domestic", Type: "https", Server: "1.12.12.12", Port: 443, ServerName: "doh.pub"},
	{ID: "aliyun-udp", Name: "AliDNS UDP", Scope: "domestic", Type: "udp", Server: "223.5.5.5", Port: 53},
	{ID: "tencent-udp", Name: "DNSPod UDP", Scope: "domestic", Type: "udp", Server: "119.29.29.29", Port: 53},

	// 国际主流加密 DNS（Google & Cloudflare）
	{ID: "cloudflare-dot", Name: "Cloudflare DoT", Scope: "global", Type: "tls", Server: "1.1.1.1", Port: 853, ServerName: "cloudflare-dns.com"},
	{ID: "cloudflare-doh", Name: "Cloudflare DoH", Scope: "global", Type: "https", Server: "1.1.1.1", Port: 443, ServerName: "cloudflare-dns.com"},
	{ID: "google-dot", Name: "Google DoT", Scope: "global", Type: "tls", Server: "8.8.8.8", Port: 853, ServerName: "dns.google"},
	{ID: "google-doh", Name: "Google DoH", Scope: "global", Type: "https", Server: "8.8.8.8", Port: 443, ServerName: "dns.google"},
	{ID: "google-udp", Name: "Google UDP", Scope: "global", Type: "udp", Server: "8.8.8.8", Port: 53},

	// 代理出口预设（出口 C）
	{ID: "proxy-cloudflare-doh", Name: "Cloudflare DoH", Scope: "proxy", Type: "https", Server: "1.1.1.1", Port: 443, ServerName: "cloudflare-dns.com"},
	{ID: "proxy-cloudflare-dot", Name: "Cloudflare DoT", Scope: "proxy", Type: "tls", Server: "1.1.1.1", Port: 853, ServerName: "cloudflare-dns.com"},
	{ID: "proxy-google-doh", Name: "Google DoH", Scope: "proxy", Type: "https", Server: "8.8.8.8", Port: 443, ServerName: "dns.google"},
	{ID: "proxy-google-dot", Name: "Google DoT", Scope: "proxy", Type: "tls", Server: "8.8.8.8", Port: 853, ServerName: "dns.google"},
	{ID: "proxy-google-udp", Name: "Google UDP", Scope: "proxy", Type: "udp", Server: "8.8.8.8", Port: 53},
}

func Defaults() Settings {
	return Settings{
		SchemaVersion: SchemaVersion,
		Domestic:      Server{PresetID: "aliyun-dot", Type: "tls", Server: "223.5.5.5", Port: 853, ServerName: "dns.alidns.com"},
		Global:        Server{PresetID: "tencent-dot", Type: "tls", Server: "1.12.12.12", Port: 853, ServerName: "dot.pub"},
		Proxy:         &Server{PresetID: "proxy-cloudflare-doh", Type: "https", Server: "1.1.1.1", Port: 443, ServerName: "cloudflare-dns.com"},
	}
}

func Presets() []Preset { return append([]Preset(nil), presets...) }

type Manager struct {
	mu       sync.RWMutex
	path     string
	settings Settings
}

func New(path string) (*Manager, error) {
	m := &Manager{path: path, settings: Defaults()}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read DNS settings: %w", err)
	}
	var stored Settings
	if err := json.Unmarshal(data, &stored); err != nil {
		return nil, fmt.Errorf("decode DNS settings: %w", err)
	}
	if err := Validate(stored); err != nil {
		return nil, err
	}
	m.settings = stored
	return m, nil
}

func (m *Manager) Get() Settings { m.mu.RLock(); defer m.mu.RUnlock(); return m.settings }

func (m *Manager) Configure(value Settings) (Settings, error) {
	value.SchemaVersion = SchemaVersion
	if err := Validate(value); err != nil {
		return Settings{}, err
	}
	data, _ := json.MarshalIndent(value, "", "  ")
	if err := os.MkdirAll(filepath.Dir(m.path), 0700); err != nil {
		return Settings{}, err
	}
	temporary := m.path + ".tmp"
	if err := os.WriteFile(temporary, append(data, '\n'), 0600); err != nil {
		return Settings{}, err
	}
	if err := replaceFile(temporary, m.path); err != nil {
		_ = os.Remove(temporary)
		return Settings{}, err
	}
	m.mu.Lock()
	m.settings = value
	m.mu.Unlock()
	return value, nil
}

func Validate(value Settings) error {
	if value.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported DNS settings schema_version %d", value.SchemaVersion)
	}
	if err := validateServer("domestic", value.Domestic); err != nil {
		return err
	}
	if err := validateServer("global", value.Global); err != nil {
		return err
	}
	if value.Proxy != nil && value.Proxy.Server != "" {
		if err := validateServer("proxy", *value.Proxy); err != nil {
			return err
		}
	}
	if value.Domestic.Type == value.Global.Type && value.Domestic.Server == value.Global.Server && value.Domestic.Port == value.Global.Port && strings.EqualFold(value.Domestic.ServerName, value.Global.ServerName) {
		return errors.New("domestic and global DNS upstreams must not be identical")
	}
	return nil
}

func validateServer(label string, value Server) error {
	address, err := netip.ParseAddr(strings.TrimSpace(value.Server))
	if err != nil || address.IsUnspecified() || address.IsLoopback() || address.IsMulticast() {
		return fmt.Errorf("%s DNS server must be a usable fixed IP address", label)
	}
	if value.Port == 0 {
		return fmt.Errorf("%s DNS port is required", label)
	}
	switch value.Type {
	case "udp":
		if value.ServerName != "" {
			return fmt.Errorf("%s UDP DNS must not set server_name", label)
		}
	case "tls", "https":
		name := strings.TrimSpace(value.ServerName)
		if name == "" || strings.ContainsAny(name, "/: \\") {
			return fmt.Errorf("%s encrypted DNS requires a valid server_name", label)
		}
	default:
		return fmt.Errorf("unsupported %s DNS type %q", label, value.Type)
	}
	if value.PresetID != "" {
		found := false
		for _, preset := range presets {
			if preset.ID == value.PresetID {
				found = true
				if preset.Type != value.Type || preset.Server != value.Server || preset.Port != value.Port || preset.ServerName != value.ServerName {
					return fmt.Errorf("DNS preset %q was modified", value.PresetID)
				}
				break
			}
		}
		if !found {
			return fmt.Errorf("unknown DNS preset %q", value.PresetID)
		}
	}
	return nil
}

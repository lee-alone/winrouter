package nodes

import "time"

const (
	SchemaVersion   = 3
	TypeHTTP        = "http"
	TypeShadowsocks = "shadowsocks"
	TypeVMess       = "vmess"
	TypeVLESS       = "vless"
	TypeTrojan      = "trojan"

	EgressA = "a"
	EgressB = "b"

	ProxyModeSingle = "single"
	ProxyModeChain  = "chain"
)

type AuthenticationInput struct {
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
	UUID     string `json:"uuid,omitempty"`
	Method   string `json:"method,omitempty"`
	Flow     string `json:"flow,omitempty"`
}

type TLSInput struct {
	Enabled    bool     `json:"enabled,omitempty"`
	ServerName string   `json:"server_name,omitempty"`
	Insecure   bool     `json:"insecure,omitempty"`
	ALPN       []string `json:"alpn,omitempty"`
}

type TransportInput struct {
	Type string `json:"type,omitempty"`
	Path string `json:"path,omitempty"`
	Host string `json:"host,omitempty"`
}

type Input struct {
	ID             string              `json:"id,omitempty"`
	Name           string              `json:"name"`
	Type           string              `json:"type"`
	Server         string              `json:"server"`
	Port           uint16              `json:"port"`
	Egress         string              `json:"egress,omitempty"`
	Authentication AuthenticationInput `json:"authentication,omitempty"`
	TLS            *TLSInput           `json:"tls,omitempty"`
	Transport      *TransportInput     `json:"transport,omitempty"`
	ClearSecret    bool                `json:"clear_secret,omitempty"`

	// Legacy backward compatibility fields
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	ClearPassword bool   `json:"clear_password,omitempty"`
}

type AuthenticationNode struct {
	Username string `json:"username,omitempty"`
	Method   string `json:"method,omitempty"`
	Flow     string `json:"flow,omitempty"`
}

type TLSNode struct {
	Enabled    bool     `json:"enabled"`
	ServerName string   `json:"server_name,omitempty"`
	Insecure   bool     `json:"insecure,omitempty"`
	ALPN       []string `json:"alpn,omitempty"`
}

type TransportNode struct {
	Type string `json:"type,omitempty"`
	Path string `json:"path,omitempty"`
	Host string `json:"host,omitempty"`
}

type Node struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	Type           string             `json:"type"`
	Server         string             `json:"server"`
	ResolvedIP     string             `json:"resolved_ip,omitempty"`
	Port           uint16             `json:"port"`
	Egress         string             `json:"egress"`
	Authentication AuthenticationNode `json:"authentication,omitempty"`
	TLS            *TLSNode           `json:"tls,omitempty"`
	Transport      *TransportNode     `json:"transport,omitempty"`
	HasSecret      bool               `json:"has_secret"`
	HasPassword    bool               `json:"has_password"`       // Legacy compatibility
	Username       string             `json:"username,omitempty"` // Legacy compatibility
	Selected       bool               `json:"selected"`
	ChainPosition  int                `json:"chain_position,omitempty"`
	SubscriptionID   string             `json:"subscription_id,omitempty"`
	Favorite         bool               `json:"favorite"`
	EgressOverridden bool               `json:"egress_overridden,omitempty"`
}

type Credentials struct {
	Username string
	Password string
	UUID     string
	Method   string
	Flow     string
}

type secretDocument struct {
	Password string `json:"password,omitempty"`
	UUID     string `json:"uuid,omitempty"`
}

type TestResult struct {
	NodeID            string    `json:"node_id"`
	Available         bool      `json:"available"`
	TCPReachable      bool      `json:"tcp_reachable,omitempty"`
	ProtocolAvailable bool      `json:"protocol_available,omitempty"`
	LatencyMS         int64     `json:"latency_ms"`
	TCPMS             int64     `json:"tcp_ms,omitempty"`
	ProtocolMS        int64     `json:"protocol_ms,omitempty"`
	TotalMS           int64     `json:"total_ms,omitempty"`
	Status            int       `json:"status,omitempty"`
	ErrorCategory     string    `json:"error_category,omitempty"`
	Error             string    `json:"error,omitempty"`
	TestedAt          time.Time `json:"tested_at"`
}

type storedNode struct {
	ID               string         `json:"id"`
	Name             string         `json:"name"`
	Type             string         `json:"type"`
	Server           string         `json:"server"`
	ResolvedIP       string         `json:"resolved_ip,omitempty"`
	Port             uint16         `json:"port"`
	Egress           string         `json:"egress"`
	Username         string         `json:"username,omitempty"`
	Method           string         `json:"method,omitempty"`
	Flow             string         `json:"flow,omitempty"`
	TLS              *TLSNode       `json:"tls,omitempty"`
	Transport        *TransportNode `json:"transport,omitempty"`
	ProtectedSecret  string         `json:"protected_secret,omitempty"`
	SubscriptionID   string         `json:"subscription_id,omitempty"`
	Favorite         bool           `json:"favorite,omitempty"`
	EgressOverridden bool           `json:"egress_overridden,omitempty"`

	// Legacy fields for migration
	LegacyProtected string `json:"protected_password,omitempty"`
}

type state struct {
	SchemaVersion int          `json:"schema_version"`
	SelectedID    string       `json:"selected_id,omitempty"`
	Mode          string       `json:"mode,omitempty"`
	SelectedChain []string     `json:"selected_chain,omitempty"`
	Nodes         []storedNode `json:"nodes"`
}

type ProxySelection struct {
	Mode          string   `json:"mode"`
	SelectedID    string   `json:"selected_id"`
	SelectedChain []string `json:"selected_chain"`
}

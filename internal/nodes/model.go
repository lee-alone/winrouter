package nodes

import "time"

const (
	SchemaVersion   = 2
	TypeHTTP        = "http"
	TypeShadowsocks = "shadowsocks"
)

type Input struct {
	ID            string `json:"id,omitempty"`
	Name          string `json:"name"`
	Type          string `json:"type"`
	Server        string `json:"server"`
	Port          uint16 `json:"port"`
	Username      string `json:"username,omitempty"`
	Password      string `json:"password,omitempty"`
	ClearPassword bool   `json:"clear_password,omitempty"`
}

type Node struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Server         string `json:"server"`
	ResolvedIP     string `json:"resolved_ip,omitempty"`
	Port           uint16 `json:"port"`
	Username       string `json:"username,omitempty"`
	HasPassword    bool   `json:"has_password"`
	Selected       bool   `json:"selected"`
	SubscriptionID string `json:"subscription_id,omitempty"`
	Favorite       bool   `json:"favorite"`
}

type Credentials struct {
	Username string
	Password string
}

type TestResult struct {
	NodeID    string    `json:"node_id"`
	Available bool      `json:"available"`
	LatencyMS int64     `json:"latency_ms"`
	Status    int       `json:"status,omitempty"`
	Error     string    `json:"error,omitempty"`
	TestedAt  time.Time `json:"tested_at"`
}

type storedNode struct {
	ID             string `json:"id"`
	Name           string `json:"name"`
	Type           string `json:"type"`
	Server         string `json:"server"`
	ResolvedIP     string `json:"resolved_ip,omitempty"`
	Port           uint16 `json:"port"`
	Username       string `json:"username,omitempty"`
	Protected      string `json:"protected_password,omitempty"`
	SubscriptionID string `json:"subscription_id,omitempty"`
	Favorite       bool   `json:"favorite,omitempty"`
}

type state struct {
	SchemaVersion int          `json:"schema_version"`
	SelectedID    string       `json:"selected_id,omitempty"`
	Nodes         []storedNode `json:"nodes"`
}

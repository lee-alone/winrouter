package subscriptions

import "time"

const SchemaVersion = 1

type Input struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	URL    string `json:"url"`
	Egress string `json:"egress,omitempty"`
}

type Subscription struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Host      string    `json:"host"`
	Egress    string    `json:"egress"`
	NodeCount int       `json:"node_count"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
	LastError string    `json:"last_error,omitempty"`
}

type document struct {
	Version int            `json:"version"`
	Nodes   []documentNode `json:"nodes"`
}

type documentNode struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     uint16 `json:"port"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type storedSubscription struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ProtectedURL string    `json:"protected_url"`
	Host         string    `json:"host"`
	Egress       string    `json:"egress,omitempty"`
	NodeCount    int       `json:"node_count"`
	UpdatedAt    time.Time `json:"updated_at,omitempty"`
	LastError    string    `json:"last_error,omitempty"`
}

type state struct {
	SchemaVersion int                  `json:"schema_version"`
	Subscriptions []storedSubscription `json:"subscriptions"`
}

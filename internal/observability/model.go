package observability

import "time"

type Level string

const (
	LevelInfo    Level = "info"
	LevelWarning Level = "warning"
	LevelError   Level = "error"
)

type LogEntry struct {
	Time          time.Time      `json:"time"`
	Level         Level          `json:"level"`
	Component     string         `json:"component"`
	Message       string         `json:"message"`
	CorrelationID string         `json:"correlation_id,omitempty"`
	Fields        map[string]any `json:"fields,omitempty"`
}

type ProbeRequest struct {
	Protocol          string `json:"protocol"`
	Target            string `json:"target"`
	DNSName           string `json:"dns_name,omitempty"`
	ExpectedInterface string `json:"expected_interface,omitempty"`
	TimeoutMS         int    `json:"timeout_ms,omitempty"`
}

type ProbeResult struct {
	Time              time.Time `json:"time"`
	Protocol          string    `json:"protocol"`
	Target            string    `json:"target"`
	SourceAddress     string    `json:"source_address,omitempty"`
	ExpectedInterface string    `json:"expected_interface,omitempty"`
	ActualInterface   string    `json:"actual_interface,omitempty"`
	DurationMS        int64     `json:"duration_ms"`
	Success           bool      `json:"success"`
	Error             string    `json:"error,omitempty"`
}

type InterfaceCounter struct {
	GUID        string    `json:"guid"`
	Name        string    `json:"name"`
	Received    uint64    `json:"received_bytes"`
	Transmitted uint64    `json:"transmitted_bytes"`
	SampledAt   time.Time `json:"sampled_at"`
}

type ConnectionSummary struct {
	ActiveTCP      int       `json:"active_tcp"`
	EstablishedTCP int       `json:"established_tcp"`
	ListeningTCP   int       `json:"listening_tcp"`
	UDPEndpoints   int       `json:"udp_endpoints"`
	SampledAt      time.Time `json:"sampled_at"`
}

type RuleHit struct {
	Outbound string `json:"outbound"`
	Count    int    `json:"count"`
}

type RuleSetMetadata struct {
	Name       string `json:"name"`
	Version    string `json:"version"`
	SHA256     string `json:"sha256"`
	Source     string `json:"source"`
	RuleCount  int    `json:"rule_count"`
	Size       int64  `json:"size"`
	LoadResult string `json:"load_result"`
}

type DNSStatus struct {
	Role     string `json:"role"`
	Protocol string `json:"protocol"`
	Outbound string `json:"outbound"`
	Health   string `json:"health"`
}

type BundleInput struct {
	Application any                `json:"application"`
	Interfaces  any                `json:"interfaces"`
	Core        any                `json:"core"`
	Logs        []LogEntry         `json:"logs"`
	Probes      []ProbeResult      `json:"probes"`
	Counters    []InterfaceCounter `json:"counters"`
	RuleSets    []RuleSetMetadata  `json:"rule_sets"`
	DNS         []DNSStatus        `json:"dns"`
}

type BundlePreview struct {
	Files          []string `json:"files"`
	LogCount       int      `json:"log_count"`
	ProbeCount     int      `json:"probe_count"`
	RuleSetCount   int      `json:"rule_set_count"`
	SensitiveNotes []string `json:"sensitive_notes"`
}

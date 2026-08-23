package config

const (
	SchemaVersion1 = 1
	IPv6Block      = "block"
	IPv6Split      = "split"

	DefaultIPv6TUNPrefix = "fdfe:dcba:9876::/126"
)

type MVPConfig struct {
	SchemaVersion         int               `json:"schema_version"`
	TUN                   MVPTUN            `json:"tun"`
	InterfaceA            MVPInterface      `json:"interface_a"`
	InterfaceB            MVPInterface      `json:"interface_b"`
	DefaultOutbound       string            `json:"default_outbound,omitempty"`
	DirectPrefixes        []MVPDirectPrefix `json:"direct_prefixes"`
	CustomRules           []MVPCustomRule   `json:"custom_rules,omitempty"`
	RuleOrder             []string          `json:"rule_order,omitempty"`
	RuleSets              []MVPRuleSet      `json:"rule_sets,omitempty"`
	Domestic              MVPDomestic       `json:"domestic"`
	DNS                   MVPDNS            `json:"dns"`
	Proxy                 *MVPProxy         `json:"proxy,omitempty"`
	IPv6                  string            `json:"ipv6"`
	ConnectionObservation bool              `json:"connection_observation,omitempty"`
	ConnectionAPISecret   string            `json:"connection_api_secret,omitempty"`
}

type MVPRuleSet struct {
	Tag    string `json:"tag"`
	Kind   string `json:"kind"`
	Action string `json:"action"`
	Path   string `json:"path"`
}

type MVPCustomRule struct {
	ID     string `json:"id,omitempty"`
	Name   string `json:"name"`
	Type   string `json:"type"`
	Value  string `json:"value"`
	Action string `json:"action"`
}

type MVPProxy struct {
	Type      string             `json:"type"`
	Server    string             `json:"server"`
	Port      uint16             `json:"port"`
	Egress    string             `json:"egress,omitempty"`
	Method    string             `json:"method,omitempty"`
	Username  string             `json:"username,omitempty"`
	Password  string             `json:"password,omitempty"`
	UUID      string             `json:"uuid,omitempty"`
	Flow      string             `json:"flow,omitempty"`
	Security  string             `json:"security,omitempty"`
	TLS       *MVPProxyTLS       `json:"tls,omitempty"`
	Transport *MVPProxyTransport `json:"transport,omitempty"`
}

type MVPProxyTLS struct {
	Enabled    bool     `json:"enabled"`
	ServerName string   `json:"server_name,omitempty"`
	Insecure   bool     `json:"insecure,omitempty"`
	ALPN       []string `json:"alpn,omitempty"`
}

type MVPProxyTransport struct {
	Type string `json:"type"`
	Path string `json:"path,omitempty"`
	Host string `json:"host,omitempty"`
}

type MVPTUN struct {
	Prefix     string `json:"prefix"`
	IPv6Prefix string `json:"ipv6_prefix,omitempty"`
	Stack      string `json:"stack"`
}
type MVPInterface struct {
	GUID          string `json:"guid"`
	BindInterface string `json:"bind_interface"`
}
type MVPDirectPrefix struct {
	Prefix        string `json:"prefix"`
	BindInterface string `json:"bind_interface"`
}
type MVPDomestic struct {
	CIDRs          []string `json:"cidrs"`
	DomainSuffixes []string `json:"domain_suffixes"`
}
type MVPDNS struct {
	Domestic MVPDNSServer  `json:"domestic"`
	Global   MVPDNSServer  `json:"global"`
	Proxy    *MVPDNSServer `json:"proxy,omitempty"`
}
type MVPDNSServer struct {
	Type       string `json:"type"`
	Server     string `json:"server"`
	Port       uint16 `json:"port"`
	ServerName string `json:"server_name,omitempty"`
}

type Generated struct {
	Model              MinimalTUN
	JSON               []byte
	RuleCategories     []string
	ProxyBindInterface string
}

type RulePreview struct {
	Position int      `json:"position"`
	Category string   `json:"category"`
	Match    []string `json:"match"`
	Action   string   `json:"action"`
}

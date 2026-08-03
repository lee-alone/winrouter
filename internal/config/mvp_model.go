package config

const (
	SchemaVersion1  = 1
	ModeDirectSplit = "direct-split"
	ModeProxySplit  = "proxy-split"
	IPv6Block       = "block"
	IPv6Split       = "split"
)

type MVPConfig struct {
	SchemaVersion  int               `json:"schema_version"`
	Mode           string            `json:"mode"`
	TUN            MVPTUN            `json:"tun"`
	InterfaceA     MVPInterface      `json:"interface_a"`
	InterfaceB     MVPInterface      `json:"interface_b"`
	DirectPrefixes []MVPDirectPrefix `json:"direct_prefixes"`
	CustomRules    []MVPCustomRule   `json:"custom_rules,omitempty"`
	RuleSets       []MVPRuleSet      `json:"rule_sets,omitempty"`
	Domestic       MVPDomestic       `json:"domestic"`
	DNS            MVPDNS            `json:"dns"`
	Proxy          *MVPProxy         `json:"proxy,omitempty"`
	IPv6           string            `json:"ipv6"`
}

type MVPRuleSet struct {
	Tag    string `json:"tag"`
	Kind   string `json:"kind"`
	Action string `json:"action"`
	Path   string `json:"path"`
}

type MVPCustomRule struct {
	Name   string `json:"name"`
	Type   string `json:"type"`
	Value  string `json:"value"`
	Action string `json:"action"`
}

type MVPProxy struct {
	Type     string `json:"type"`
	Server   string `json:"server"`
	Port     uint16 `json:"port"`
	Method   string `json:"method,omitempty"`
	Username string `json:"username,omitempty"`
	Password string `json:"password,omitempty"`
}

type MVPTUN struct {
	Prefix string `json:"prefix"`
	Stack  string `json:"stack"`
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
	Domestic MVPDNSServer `json:"domestic"`
	Global   MVPDNSServer `json:"global"`
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
	Mode               string
	ProxyBindInterface string
}

type RulePreview struct {
	Position int      `json:"position"`
	Category string   `json:"category"`
	Match    []string `json:"match"`
	Action   string   `json:"action"`
}

package config

const (
	TUNStackSystem = "system"
	TUNStackGVisor = "gvisor"
	TUNStackMixed  = "mixed"
)

type MinimalTUN struct {
	Log          LogConfig           `json:"log"`
	Experimental *ExperimentalConfig `json:"experimental,omitempty"`
	DNS          DNSConfig           `json:"dns"`
	Inbounds     []TUNInbound        `json:"inbounds"`
	Outbounds    []Outbound          `json:"outbounds"`
	Route        RouteConfig         `json:"route"`
}

type ExperimentalConfig struct {
	ClashAPI *ClashAPIConfig `json:"clash_api,omitempty"`
}
type ClashAPIConfig struct {
	ExternalController string `json:"external_controller"`
	Secret             string `json:"secret,omitempty"`
}

type RuleSet struct {
	Type   string `json:"type"`
	Tag    string `json:"tag"`
	Format string `json:"format"`
	Path   string `json:"path"`
}

type DNSConfig struct {
	Servers          []DNSServer `json:"servers"`
	Rules            []DNSRule   `json:"rules,omitempty"`
	Final            string      `json:"final"`
	Strategy         string      `json:"strategy"`
	IndependentCache bool        `json:"independent_cache,omitempty"`
}

type DNSRule struct {
	Domain       []string `json:"domain,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	RuleSet      []string `json:"rule_set,omitempty"`
	QueryType    []string `json:"query_type,omitempty"`
	Action       string   `json:"action"`
	Server       string   `json:"server,omitempty"`
}

type DNSServer struct {
	Type       string              `json:"type"`
	Tag        string              `json:"tag"`
	Server     string              `json:"server,omitempty"`
	ServerPort uint16              `json:"server_port,omitempty"`
	Detour     string              `json:"detour,omitempty"`
	Path       string              `json:"path,omitempty"`
	TLS        *TLSConfig          `json:"tls,omitempty"`
	Inet4Range string              `json:"inet4_range,omitempty"`
	Inet6Range string              `json:"inet6_range,omitempty"`
	Predefined map[string][]string `json:"predefined,omitempty"`
}

type TLSConfig struct {
	Enabled    bool     `json:"enabled"`
	ServerName string   `json:"server_name,omitempty"`
	Insecure   bool     `json:"insecure,omitempty"`
	ALPN       []string `json:"alpn,omitempty"`
}

type TransportConfig struct {
	Type    string              `json:"type"`
	Path    string              `json:"path,omitempty"`
	Headers map[string][]string `json:"headers,omitempty"`
}

type LogConfig struct {
	Level     string `json:"level"`
	Timestamp bool   `json:"timestamp"`
}

type TUNInbound struct {
	Type          string   `json:"type"`
	Tag           string   `json:"tag"`
	InterfaceName string   `json:"interface_name"`
	Address       []string `json:"address"`
	AutoRoute     bool     `json:"auto_route"`
	StrictRoute   bool     `json:"strict_route"`
	Stack         string   `json:"stack"`
}

type Outbound struct {
	Type           string           `json:"type"`
	Tag            string           `json:"tag"`
	BindInterface  string           `json:"bind_interface,omitempty"`
	Detour         string           `json:"detour,omitempty"`
	Server         string           `json:"server,omitempty"`
	ServerPort     uint16           `json:"server_port,omitempty"`
	DomainResolver string           `json:"domain_resolver,omitempty"`
	Method         string           `json:"method,omitempty"`
	Username       string           `json:"username,omitempty"`
	Password       string           `json:"password,omitempty"`
	UUID           string           `json:"uuid,omitempty"`
	Flow           string           `json:"flow,omitempty"`
	Security       string           `json:"security,omitempty"`
	TLS            *TLSConfig       `json:"tls,omitempty"`
	Transport      *TransportConfig `json:"transport,omitempty"`
}

type RouteConfig struct {
	AutoDetectInterface bool        `json:"auto_detect_interface"`
	DefaultDNSResolver  string      `json:"default_domain_resolver,omitempty"`
	RuleSets            []RuleSet   `json:"rule_set,omitempty"`
	Rules               []RouteRule `json:"rules"`
	Final               string      `json:"final"`
}

type RouteRule struct {
	Protocol     string   `json:"protocol,omitempty"`
	IPCIDR       []string `json:"ip_cidr,omitempty"`
	Domain       []string `json:"domain,omitempty"`
	DomainSuffix []string `json:"domain_suffix,omitempty"`
	IPVersion    int      `json:"ip_version,omitempty"`
	ProcessName  []string `json:"process_name,omitempty"`
	ProcessPath  []string `json:"process_path,omitempty"`
	RuleSet      []string `json:"rule_set,omitempty"`
	Action       string   `json:"action"`
	Outbound     string   `json:"outbound,omitempty"`
}


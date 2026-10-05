package config

import (
	"encoding/json"
	"fmt"
	"net/netip"
)

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

func GenerateIPv6BlockedTUN(networkPrefix, stack, interfaceA, interfaceB string) ([]byte, error) {
	data, err := GenerateDualDNSTUN(networkPrefix, stack, interfaceA, interfaceB)
	if err != nil {
		return nil, err
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode dual DNS TUN config: %w", err)
	}
	model.Inbounds[0].Address = append(model.Inbounds[0].Address, "fdfe:dcba:9876::1/126")
	model.Route.Rules = append(model.Route.Rules, RouteRule{IPVersion: 6, Action: "reject"})
	data, err = json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal IPv6-blocked TUN config: %w", err)
	}
	return append(data, '\n'), nil
}

func GenerateDualDirectTUN(networkPrefix, stack, interfaceA, interfaceB, targetA, targetB, udpTargetA, udpTargetB string) ([]byte, error) {
	if interfaceA == "" || interfaceB == "" || interfaceA == interfaceB {
		return nil, fmt.Errorf("two distinct outbound interfaces are required")
	}
	for label, value := range map[string]string{"target A": targetA, "target B": targetB, "UDP target A": udpTargetA, "UDP target B": udpTargetB} {
		prefix, err := netip.ParsePrefix(value)
		if err != nil || !prefix.Addr().Is4() || prefix.Bits() != 32 {
			return nil, fmt.Errorf("%s must be an IPv4 /32 prefix", label)
		}
	}
	data, err := GenerateMinimalTUN(networkPrefix, stack)
	if err != nil {
		return nil, err
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode base TUN config: %w", err)
	}
	model.Outbounds = []Outbound{
		{Type: "direct", Tag: "a-direct", BindInterface: interfaceA},
		{Type: "direct", Tag: "b-direct", BindInterface: interfaceB},
	}
	model.DNS.Servers[0].Detour = "b-direct"
	model.Route.Rules = []RouteRule{
		{Action: "sniff"},
		{IPCIDR: []string{targetA, udpTargetA}, Action: "route", Outbound: "a-direct"},
		{IPCIDR: []string{targetB, udpTargetB}, Action: "route", Outbound: "b-direct"},
		{Protocol: "dns", Action: "hijack-dns"},
	}
	model.Route.Final = "b-direct"
	data, err = json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal dual direct TUN config: %w", err)
	}
	return append(data, '\n'), nil
}

func GenerateDualLANDirectTUN(networkPrefix, stack, interfaceA, interfaceB, prefixA, prefixB string) ([]byte, error) {
	if interfaceA == "" || interfaceB == "" || interfaceA == interfaceB {
		return nil, fmt.Errorf("two distinct outbound interfaces are required")
	}
	for label, value := range map[string]string{"LAN prefix A": prefixA, "LAN prefix B": prefixB} {
		prefix, err := netip.ParsePrefix(value)
		if err != nil || !prefix.Addr().Is4() || prefix.Bits() == 32 {
			return nil, fmt.Errorf("%s must be an IPv4 network prefix", label)
		}
	}
	data, err := GenerateMinimalTUN(networkPrefix, stack)
	if err != nil {
		return nil, err
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode base TUN config: %w", err)
	}
	model.Outbounds = []Outbound{
		{Type: "direct", Tag: "a-direct", BindInterface: interfaceA},
		{Type: "direct", Tag: "b-direct", BindInterface: interfaceB},
	}
	model.DNS.Servers[0].Detour = "b-direct"
	model.Route.Rules = []RouteRule{
		{Action: "sniff"},
		{IPCIDR: []string{prefixA}, Action: "route", Outbound: "a-direct"},
		{IPCIDR: []string{prefixB}, Action: "route", Outbound: "b-direct"},
		{Protocol: "dns", Action: "hijack-dns"},
	}
	model.Route.Final = "b-direct"
	data, err = json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal dual LAN direct TUN config: %w", err)
	}
	return append(data, '\n'), nil
}

func GenerateDualDNSTUN(networkPrefix, stack, interfaceA, interfaceB string) ([]byte, error) {
	data, err := GenerateDualLANDirectTUN(networkPrefix, stack, interfaceA, interfaceB, "10.12.85.0/24", "192.168.1.0/24")
	if err != nil {
		return nil, err
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode dual direct TUN config: %w", err)
	}
	model.DNS = DNSConfig{
		Servers: []DNSServer{
			{Type: "udp", Tag: "dns-a", Server: "1.1.1.1", ServerPort: 53, Detour: "a-direct"},
			{Type: "udp", Tag: "dns-b", Server: "8.8.8.8", ServerPort: 53, Detour: "b-direct"},
		},
		Rules: []DNSRule{
			{DomainSuffix: []string{"one.one.one.one"}, Action: "route", Server: "dns-a"},
			{DomainSuffix: []string{"dns.google"}, Action: "route", Server: "dns-b"},
		},
		Final:            "dns-b",
		Strategy:         "ipv4_only",
		IndependentCache: true,
	}
	model.Route.DefaultDNSResolver = "dns-b"
	data, err = json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal dual DNS TUN config: %w", err)
	}
	return append(data, '\n'), nil
}

func GenerateEncryptedDNSTUN(networkPrefix, stack, interfaceA, interfaceB string) ([]byte, error) {
	data, err := GenerateDualDNSTUN(networkPrefix, stack, interfaceA, interfaceB)
	if err != nil {
		return nil, err
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode dual DNS TUN config: %w", err)
	}
	model.DNS.Servers = []DNSServer{
		{Type: "tls", Tag: "dns-a", Server: "1.1.1.1", ServerPort: 853, Detour: "a-direct", TLS: &TLSConfig{Enabled: true, ServerName: "cloudflare-dns.com"}},
		{Type: "https", Tag: "dns-b", Server: "8.8.8.8", ServerPort: 443, Detour: "b-direct", Path: "/dns-query", TLS: &TLSConfig{Enabled: true, ServerName: "dns.google"}},
	}
	data, err = json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal encrypted DNS TUN config: %w", err)
	}
	return append(data, '\n'), nil
}

func GenerateFakeIPTUN(networkPrefix, stack, interfaceA, interfaceB string) ([]byte, error) {
	data, err := GenerateDualDNSTUN(networkPrefix, stack, interfaceA, interfaceB)
	if err != nil {
		return nil, err
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode dual DNS TUN config: %w", err)
	}
	model.DNS.Servers = append(model.DNS.Servers, DNSServer{
		Type: "fakeip", Tag: "dns-fakeip", Inet4Range: "198.18.0.0/15", Inet6Range: "fc00::/18",
	})
	model.DNS.Rules = []DNSRule{
		{DomainSuffix: []string{"example.com"}, QueryType: []string{"A"}, Action: "route", Server: "dns-fakeip"},
	}
	data, err = json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal fake-IP TUN config: %w", err)
	}
	return append(data, '\n'), nil
}

func GenerateProxyLoopTUN(networkPrefix, stack, interfaceA, interfaceB, proxyServer string, proxyPort uint16, testTarget string) ([]byte, error) {
	proxyAddress, err := netip.ParseAddr(proxyServer)
	if err != nil || !proxyAddress.Is4() {
		return nil, fmt.Errorf("proxy server must be an IPv4 address")
	}
	targetPrefix, err := netip.ParsePrefix(testTarget)
	if err != nil || !targetPrefix.Addr().Is4() || targetPrefix.Bits() != 32 {
		return nil, fmt.Errorf("proxy test target must be an IPv4 /32 prefix")
	}
	if proxyPort == 0 {
		return nil, fmt.Errorf("proxy port is required")
	}
	data, err := GenerateDualDNSTUN(networkPrefix, stack, interfaceA, interfaceB)
	if err != nil {
		return nil, err
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode dual DNS TUN config: %w", err)
	}
	model.Outbounds = append(model.Outbounds, Outbound{
		Type: "http", Tag: "proxy", Server: proxyAddress.String(), ServerPort: proxyPort,
		BindInterface: interfaceB, DomainResolver: "dns-b",
	})
	model.Route.Rules = []RouteRule{
		{Action: "sniff"},
		{IPCIDR: []string{proxyAddress.String() + "/32"}, Action: "route", Outbound: "b-direct"},
		{IPCIDR: []string{testTarget}, Action: "route", Outbound: "proxy"},
		{Protocol: "dns", Action: "hijack-dns"},
	}
	model.Route.Final = "b-direct"
	data, err = json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal proxy-loop TUN config: %w", err)
	}
	return append(data, '\n'), nil
}

func GenerateDomainProxyLoopTUN(networkPrefix, stack, interfaceA, interfaceB, proxyDomain, proxyAddress string, proxyPort uint16, testTarget string) ([]byte, error) {
	if proxyDomain == "" {
		return nil, fmt.Errorf("proxy domain is required")
	}
	data, err := GenerateProxyLoopTUN(networkPrefix, stack, interfaceA, interfaceB, proxyAddress, proxyPort, testTarget)
	if err != nil {
		return nil, err
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		return nil, fmt.Errorf("decode proxy-loop TUN config: %w", err)
	}
	model.DNS.Servers = append(model.DNS.Servers, DNSServer{
		Type: "hosts", Tag: "proxy-bootstrap", Predefined: map[string][]string{proxyDomain: {proxyAddress}},
	})
	proxy := &model.Outbounds[len(model.Outbounds)-1]
	proxy.Server = proxyDomain
	proxy.DomainResolver = "proxy-bootstrap"
	model.Route.Rules = append([]RouteRule{{DomainSuffix: []string{proxyDomain}, Action: "route", Outbound: "b-direct"}}, model.Route.Rules...)
	data, err = json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal domain proxy-loop TUN config: %w", err)
	}
	return append(data, '\n'), nil
}

func GenerateMinimalTUN(networkPrefix, stack string) ([]byte, error) {
	prefix, err := netip.ParsePrefix(networkPrefix)
	if err != nil {
		return nil, fmt.Errorf("parse TUN prefix: %w", err)
	}
	prefix = prefix.Masked()
	if !prefix.Addr().Is4() || prefix.Bits() > 30 {
		return nil, fmt.Errorf("TUN prefix must be an IPv4 network with at least two host addresses")
	}
	if stack != TUNStackSystem && stack != TUNStackGVisor && stack != TUNStackMixed {
		return nil, fmt.Errorf("unsupported TUN stack %q", stack)
	}
	interfaceAddress := netip.PrefixFrom(prefix.Addr().Next(), prefix.Bits())
	model := MinimalTUN{
		Log: LogConfig{Level: "info", Timestamp: true},
		DNS: DNSConfig{
			Servers: []DNSServer{{Type: "udp", Tag: "dns-direct", Server: "1.1.1.1", ServerPort: 53}},
			Final:   "dns-direct", Strategy: "ipv4_only",
		},
		Inbounds: []TUNInbound{{
			Type: "tun", Tag: "tun-in", InterfaceName: "WinRouter-TUN",
			Address: []string{interfaceAddress.String()}, AutoRoute: true,
			StrictRoute: true, Stack: stack,
		}},
		Outbounds: []Outbound{{Type: "direct", Tag: "direct"}},
		Route: RouteConfig{
			AutoDetectInterface: true,
			Rules: []RouteRule{
				{Action: "sniff"},
				{Protocol: "dns", Action: "hijack-dns"},
			},
			Final: "direct",
		},
	}
	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal minimal TUN config: %w", err)
	}
	return append(data, '\n'), nil
}

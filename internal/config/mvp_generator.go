package config

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/netip"
	"sort"
	"strings"

	"winrouter/internal/policy"
)

func DecodeMVPConfig(data []byte) (MVPConfig, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var input MVPConfig
	if err := decoder.Decode(&input); err != nil {
		return MVPConfig{}, fmt.Errorf("configuration schema: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return MVPConfig{}, fmt.Errorf("configuration schema: trailing JSON value")
	}
	if err := ValidateMVPModel(input); err != nil {
		return MVPConfig{}, err
	}
	return input, nil
}

func ValidateMVPModel(input MVPConfig) error {
	if input.SchemaVersion != SchemaVersion1 {
		return fmt.Errorf("unsupported schema_version %d", input.SchemaVersion)
	}
	if input.Mode != ModeDirectSplit && input.Mode != ModeProxySplit {
		return fmt.Errorf("unsupported mode %q", input.Mode)
	}
	if input.Mode == ModeDirectSplit && input.Proxy != nil {
		return fmt.Errorf("direct-split must not define a proxy")
	}
	if input.Mode == ModeProxySplit {
		if input.Proxy == nil {
			return fmt.Errorf("proxy-split requires a proxy")
		}
		if input.Proxy.Type != "http" && input.Proxy.Type != "shadowsocks" {
			return fmt.Errorf("unsupported proxy type %q", input.Proxy.Type)
		}
		address, err := netip.ParseAddr(input.Proxy.Server)
		if err != nil || !address.Is4() || address.IsUnspecified() || address.IsLoopback() || address.IsMulticast() {
			return fmt.Errorf("proxy server must be a usable fixed IPv4 address")
		}
		if input.Proxy.Port == 0 {
			return fmt.Errorf("proxy port is required")
		}
		if input.Proxy.Type == "http" && input.Proxy.Password != "" && strings.TrimSpace(input.Proxy.Username) == "" {
			return fmt.Errorf("proxy username is required when password is set")
		}
		if input.Proxy.Type == "shadowsocks" && (strings.TrimSpace(input.Proxy.Method) == "" || input.Proxy.Password == "") {
			return fmt.Errorf("Shadowsocks method and password are required")
		}
	}
	if strings.TrimSpace(input.InterfaceA.GUID) == "" || strings.TrimSpace(input.InterfaceB.GUID) == "" {
		return fmt.Errorf("two interface GUIDs are required")
	}
	if strings.EqualFold(strings.Trim(input.InterfaceA.GUID, "{}"), strings.Trim(input.InterfaceB.GUID, "{}")) {
		return fmt.Errorf("interface A and B must be different")
	}
	if strings.TrimSpace(input.InterfaceA.BindInterface) == "" || strings.TrimSpace(input.InterfaceB.BindInterface) == "" || strings.EqualFold(input.InterfaceA.BindInterface, input.InterfaceB.BindInterface) {
		return fmt.Errorf("two distinct bind_interface values are required")
	}
	prefix, err := netip.ParsePrefix(input.TUN.Prefix)
	if err != nil || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() || prefix.Bits() != 30 || prefix != prefix.Masked() {
		return fmt.Errorf("tun.prefix must be a canonical private IPv4 /30")
	}
	if input.TUN.Stack != TUNStackSystem && input.TUN.Stack != TUNStackGVisor && input.TUN.Stack != TUNStackMixed {
		return fmt.Errorf("unsupported TUN stack %q", input.TUN.Stack)
	}
	if input.IPv6 != IPv6Block && input.IPv6 != IPv6Split {
		return fmt.Errorf("unsupported IPv6 policy %q", input.IPv6)
	}
	if err := validateDNSServer("domestic", input.DNS.Domestic); err != nil {
		return err
	}
	if err := validateDNSServer("global", input.DNS.Global); err != nil {
		return err
	}
	if input.DNS.Domestic.Type == input.DNS.Global.Type && input.DNS.Domestic.Server == input.DNS.Global.Server && input.DNS.Domestic.Port == input.DNS.Global.Port && strings.EqualFold(input.DNS.Domestic.ServerName, input.DNS.Global.ServerName) {
		return fmt.Errorf("domestic and global DNS upstreams must be distinct")
	}
	for _, value := range input.Domestic.CIDRs {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return fmt.Errorf("invalid domestic CIDR %q", value)
		}
		if prefix.Addr().IsPrivate() || prefix.Addr().IsLoopback() || prefix.Addr().IsLinkLocalUnicast() || prefix.Addr().IsMulticast() {
			return fmt.Errorf("domestic CIDR %q is not public", value)
		}
	}
	for _, direct := range input.DirectPrefixes {
		if strings.TrimSpace(direct.BindInterface) == "" {
			return fmt.Errorf("direct prefix %q requires bind_interface", direct.Prefix)
		}
		prefix, err := netip.ParsePrefix(direct.Prefix)
		if err != nil || (input.IPv6 == IPv6Block && !prefix.Addr().Is4()) {
			return fmt.Errorf("direct prefix %q is not allowed by IPv6 policy %q", direct.Prefix, input.IPv6)
		}
	}
	if len(input.CustomRules) > 200 {
		return fmt.Errorf("custom rules exceed the limit of 200")
	}
	seenRuleSets := make(map[string]struct{}, len(input.RuleSets))
	for _, ruleSet := range input.RuleSets {
		if strings.TrimSpace(ruleSet.Tag) == "" || strings.TrimSpace(ruleSet.Path) == "" {
			return errors.New("rule-set tag and local path are required")
		}
		if _, exists := seenRuleSets[ruleSet.Tag]; exists {
			return fmt.Errorf("duplicate rule-set tag %q", ruleSet.Tag)
		}
		seenRuleSets[ruleSet.Tag] = struct{}{}
		if ruleSet.Kind != "domain" && ruleSet.Kind != "ip" {
			return fmt.Errorf("rule-set %q has unsupported kind %q", ruleSet.Tag, ruleSet.Kind)
		}
		switch ruleSet.Action {
		case "a", "b", "final", "reject":
		default:
			return fmt.Errorf("rule-set %q has unsupported action %q", ruleSet.Tag, ruleSet.Action)
		}
	}
	for index, custom := range input.CustomRules {
		if strings.TrimSpace(custom.Name) == "" || len([]rune(custom.Name)) > 80 {
			return fmt.Errorf("custom rule %d requires a name of at most 80 characters", index+1)
		}
		if custom.Type != "domain" && custom.Type != "ip" && custom.Type != "process-name" && custom.Type != "process-path" {
			return fmt.Errorf("custom rule %q has unsupported type %q", custom.Name, custom.Type)
		}
		switch custom.Action {
		case "a", "b", "final", "reject":
		default:
			return fmt.Errorf("custom rule %q has unsupported action %q", custom.Name, custom.Action)
		}
	}
	return nil
}

func validateDNSServer(label string, server MVPDNSServer) error {
	if _, err := netip.ParseAddr(server.Server); err != nil {
		return fmt.Errorf("%s DNS server must be a fixed IP address", label)
	}
	if server.Port == 0 {
		return fmt.Errorf("%s DNS port is required", label)
	}
	switch server.Type {
	case "udp":
		if server.ServerName != "" {
			return fmt.Errorf("%s UDP DNS must not set server_name", label)
		}
	case "tls", "https":
		if strings.TrimSpace(server.ServerName) == "" {
			return fmt.Errorf("%s encrypted DNS requires server_name", label)
		}
	default:
		return fmt.Errorf("unsupported %s DNS type %q", label, server.Type)
	}
	return nil
}

func GenerateMVP(input MVPConfig) (Generated, error) {
	if err := ValidateMVPModel(input); err != nil {
		return Generated{}, err
	}
	directPrefixes := append([]MVPDirectPrefix(nil), input.DirectPrefixes...)
	sort.Slice(directPrefixes, func(i, j int) bool {
		if directPrefixes[i].Prefix != directPrefixes[j].Prefix {
			return directPrefixes[i].Prefix < directPrefixes[j].Prefix
		}
		return directPrefixes[i].BindInterface < directPrefixes[j].BindInterface
	})
	outbounds := []Outbound{
		{Type: "direct", Tag: "local-direct"},
		{Type: "direct", Tag: "domestic-direct", BindInterface: input.InterfaceA.BindInterface},
		{Type: "direct", Tag: "foreign-direct", BindInterface: input.InterfaceB.BindInterface},
	}
	finalOutbound := "foreign-direct"
	if input.Mode == ModeProxySplit {
		outbounds = append(outbounds, Outbound{Type: input.Proxy.Type, Tag: "proxy", Server: input.Proxy.Server, ServerPort: input.Proxy.Port, BindInterface: input.InterfaceB.BindInterface, Method: input.Proxy.Method, Username: input.Proxy.Username, Password: input.Proxy.Password})
		finalOutbound = "proxy"
	}
	bindings := make(map[string]string)
	policyPrefixes := make([]policy.DirectPrefix, 0, len(directPrefixes))
	for _, direct := range directPrefixes {
		tag := ""
		switch {
		case strings.EqualFold(direct.BindInterface, input.InterfaceA.BindInterface):
			tag = "domestic-direct"
		case strings.EqualFold(direct.BindInterface, input.InterfaceB.BindInterface):
			tag = "foreign-direct"
		default:
			tag = bindings[direct.BindInterface]
			if tag == "" {
				tag = fmt.Sprintf("bypass-%03d", len(bindings)+1)
				bindings[direct.BindInterface] = tag
				outbounds = append(outbounds, Outbound{Type: "direct", Tag: tag, BindInterface: direct.BindInterface})
			}
		}
		policyPrefixes = append(policyPrefixes, policy.DirectPrefix{Prefix: direct.Prefix, Outbound: tag})
	}
	infrastructureB := []string(nil)
	if input.Mode == ModeProxySplit {
		proxyAddress := netip.MustParseAddr(input.Proxy.Server)
		infrastructureB = []string{netip.PrefixFrom(proxyAddress, proxyAddress.BitLen()).String()}
	}
	customRules := make([]policy.CustomRule, 0, len(input.CustomRules))
	for _, custom := range input.CustomRules {
		mapped := policy.CustomRule{Type: custom.Type, Value: custom.Value}
		switch custom.Action {
		case "reject":
			mapped.Action = "reject"
		case "a":
			mapped.Action, mapped.Outbound = "route", "domestic-direct"
		case "b":
			mapped.Action, mapped.Outbound = "route", "foreign-direct"
		case "final":
			mapped.Action, mapped.Outbound = "route", finalOutbound
		}
		customRules = append(customRules, mapped)
	}
	rules, err := policy.BuildDirectSplit(policy.Request{
		DNSUpstreamA: input.DNS.Domestic.Server, DNSUpstreamB: input.DNS.Global.Server,
		InfrastructureB: infrastructureB,
		DirectPrefixes:  policyPrefixes, DomesticCIDRs: input.Domestic.CIDRs,
		DomesticDomains: input.Domestic.DomainSuffixes, BlockIPv6: input.IPv6 == IPv6Block,
		CustomRules: customRules,
	})
	if err != nil {
		return Generated{}, err
	}
	interfacePrefix := netip.MustParsePrefix(input.TUN.Prefix)
	dnsRules := make([]DNSRule, 0)
	if input.IPv6 == IPv6Block {
		dnsRules = append(dnsRules, DNSRule{QueryType: []string{"AAAA"}, Action: "reject"})
	}
	if domains := normalizedDomainCopy(input.Domestic.DomainSuffixes); len(domains) > 0 {
		dnsRules = append(dnsRules, DNSRule{DomainSuffix: domains, Action: "route", Server: "dns-domestic"})
	}
	for _, ruleSet := range input.RuleSets {
		if ruleSet.Kind == "domain" && ruleSet.Action == "a" {
			dnsRules = append(dnsRules, DNSRule{RuleSet: []string{ruleSet.Tag}, Action: "route", Server: "dns-domestic"})
		}
	}
	routeRuleSets := make([]RuleSet, 0, len(input.RuleSets))
	for _, ruleSet := range input.RuleSets {
		routeRuleSets = append(routeRuleSets, RuleSet{Type: "local", Tag: ruleSet.Tag, Format: "binary", Path: ruleSet.Path})
	}
	model := MinimalTUN{
		Log: LogConfig{Level: "info", Timestamp: true},
		DNS: DNSConfig{
			Servers: []DNSServer{dnsServer("dns-domestic", input.DNS.Domestic, "domestic-direct"), dnsServer("dns-global", input.DNS.Global, "foreign-direct")},
			Rules:   dnsRules,
			Final:   "dns-global", Strategy: dnsStrategy(input.IPv6), IndependentCache: true,
		},
		Inbounds:  []TUNInbound{{Type: "tun", Tag: "tun-in", InterfaceName: "WinRouter-TUN", Address: []string{netip.PrefixFrom(interfacePrefix.Addr().Next(), 30).String(), "fdfe:dcba:9876::1/126"}, AutoRoute: true, StrictRoute: true, Stack: input.TUN.Stack}},
		Outbounds: outbounds,
		Route:     RouteConfig{AutoDetectInterface: false, DefaultDNSResolver: "dns-global", RuleSets: routeRuleSets, Rules: make([]RouteRule, 0, len(rules)+len(input.RuleSets)), Final: finalOutbound},
	}
	if input.ConnectionObservation {
		secret := strings.TrimSpace(input.ConnectionAPISecret)
		if secret == "" {
			return Generated{}, fmt.Errorf("connection observation API secret is required when enabled")
		}
		model.Experimental = &ExperimentalConfig{ClashAPI: &ClashAPIConfig{ExternalController: "127.0.0.1:19090", Secret: secret}}
	}
	categories := make([]string, 0, len(rules))
	for _, rule := range rules {
		model.Route.Rules = append(model.Route.Rules, RouteRule{Protocol: rule.Protocol, IPCIDR: rule.CIDRs, DomainSuffix: rule.Domains, IPVersion: rule.IPVersion, ProcessName: rule.ProcessName, ProcessPath: rule.ProcessPath, Action: rule.Action, Outbound: rule.Outbound})
		categories = append(categories, string(rule.Category))
	}
	for _, ruleSet := range input.RuleSets {
		action, outbound := "route", ""
		switch ruleSet.Action {
		case "a":
			outbound = "domestic-direct"
		case "b":
			outbound = "foreign-direct"
		case "final":
			outbound = finalOutbound
		case "reject":
			action = "reject"
		}
		model.Route.Rules = append(model.Route.Rules, RouteRule{RuleSet: []string{ruleSet.Tag}, Action: action, Outbound: outbound})
		categories = append(categories, "rule-set")
	}
	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return Generated{}, fmt.Errorf("marshal MVP configuration: %w", err)
	}
	generated := Generated{Model: model, JSON: append(data, '\n'), RuleCategories: categories, Mode: input.Mode, ProxyBindInterface: input.InterfaceB.BindInterface}
	if err := ValidateGeneratedSchema(generated.JSON); err != nil {
		return Generated{}, err
	}
	if err := ValidateMVPSemantics(generated); err != nil {
		return Generated{}, err
	}
	return generated, nil
}

func dnsStrategy(ipv6Policy string) string {
	if ipv6Policy == IPv6Split {
		return "prefer_ipv4"
	}
	return "ipv4_only"
}

func PreviewMVPRules(input MVPConfig) ([]RulePreview, error) {
	generated, err := GenerateMVP(input)
	if err != nil {
		return nil, err
	}
	result := make([]RulePreview, 0, len(generated.Model.Route.Rules)+1)
	for index, rule := range generated.Model.Route.Rules {
		matches := append([]string(nil), rule.DomainSuffix...)
		matches = append(matches, rule.IPCIDR...)
		for _, value := range rule.RuleSet {
			matches = append(matches, "rule-set="+value)
		}
		for _, value := range rule.ProcessName {
			matches = append(matches, "process-name="+value)
		}
		for _, value := range rule.ProcessPath {
			matches = append(matches, "process-path="+value)
		}
		if rule.Protocol != "" {
			matches = append(matches, "protocol="+rule.Protocol)
		}
		if rule.IPVersion != 0 {
			matches = append(matches, fmt.Sprintf("ip-version=%d", rule.IPVersion))
		}
		if len(matches) == 0 {
			matches = []string{"all metadata"}
		}
		action := rule.Action
		if rule.Outbound != "" {
			action += " -> " + rule.Outbound
		}
		result = append(result, RulePreview{Position: index + 1, Category: generated.RuleCategories[index], Match: matches, Action: action})
	}
	result = append(result, RulePreview{Position: len(result) + 1, Category: "final", Match: []string{"unmatched traffic"}, Action: "route -> " + generated.Model.Route.Final})
	return result, nil
}

func dnsServer(tag string, input MVPDNSServer, detour string) DNSServer {
	server := DNSServer{Type: input.Type, Tag: tag, Server: input.Server, ServerPort: input.Port, Detour: detour}
	if input.Type == "tls" || input.Type == "https" {
		server.TLS = &TLSConfig{Enabled: true, ServerName: input.ServerName}
	}
	if input.Type == "https" {
		server.Path = "/dns-query"
	}
	return server
}

func normalizedDomainCopy(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		value = strings.ToLower(strings.Trim(strings.TrimSpace(value), "."))
		if value != "" {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	sort.Strings(result)
	return result
}

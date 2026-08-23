package policy

import (
	"fmt"
	"net/netip"
	"sort"
	"strings"
)

type Category string

const (
	CategoryMetadata       Category = "metadata"
	CategoryLocal          Category = "local"
	CategoryInfrastructure Category = "infrastructure"
	CategoryUser           Category = "user"
	CategoryDirectPrefix   Category = "direct-prefix"
	CategoryReserved       Category = "reserved"
	CategoryDomestic       Category = "domestic"
)

type DirectPrefix struct {
	Prefix   string
	Outbound string
}

type Request struct {
	DNSUpstreamA    string
	DNSUpstreamB    string
	TUNIPv6         string
	InfrastructureA []string
	InfrastructureB []string
	DirectPrefixes  []DirectPrefix
	DomesticCIDRs   []string
	DomesticDomains []string
	BlockIPv6       bool
	CustomRules     []CustomRule
}

type CustomRule struct {
	ID       string
	Type     string
	Value    string
	Action   string
	Outbound string
}

type Rule struct {
	ID           string
	Category     Category
	Protocol     string
	CIDRs        []string
	Domains      []string
	ExactDomains []string
	IPVersion    int
	ProcessName  []string
	ProcessPath  []string
	Action       string
	Outbound     string
}

var reservedCIDRs = []string{
	"0.0.0.0/8", "10.0.0.0/8", "100.64.0.0/10", "169.254.0.0/16", "172.16.0.0/12",
	"192.0.0.0/24", "192.0.2.0/24", "192.168.0.0/16",
	"198.18.0.0/15", "198.51.100.0/24", "203.0.113.0/24", "224.0.0.0/4", "240.0.0.0/4",
}

var reservedIPv6CIDRs = []string{
	"::/128", "2001:db8::/32", "fc00::/7", "fe80::/10", "ff00::/8",
}

func BuildDirectSplit(request Request) ([]Rule, error) {
	upstreamA, err := hostPrefix(request.DNSUpstreamA)
	if err != nil {
		return nil, fmt.Errorf("DNS upstream A: %w", err)
	}
	upstreamB, err := hostPrefix(request.DNSUpstreamB)
	if err != nil {
		return nil, fmt.Errorf("DNS upstream B: %w", err)
	}
	direct, err := normalizeDirectPrefixes(request.DirectPrefixes)
	if err != nil {
		return nil, err
	}
	domesticCIDRs, err := normalizePrefixes(request.DomesticCIDRs)
	if err != nil {
		return nil, fmt.Errorf("domestic CIDR: %w", err)
	}
	domesticDomains, err := normalizeDomains(request.DomesticDomains)
	if err != nil {
		return nil, fmt.Errorf("domestic domain: %w", err)
	}
	localCIDRs := []string{"127.0.0.0/8", "::1/128"}
	if request.TUNIPv6 != "" {
		if tunHost, err := hostPrefix(request.TUNIPv6); err == nil {
			localCIDRs = append(localCIDRs, tunHost)
		}
	}
	rules := []Rule{
		{Category: CategoryMetadata, Action: "sniff"},
		{Category: CategoryLocal, CIDRs: localCIDRs, Action: "route", Outbound: "local-direct"},
		{Category: CategoryInfrastructure, Protocol: "dns", Action: "hijack-dns"},
		{Category: CategoryInfrastructure, CIDRs: []string{upstreamA}, Action: "route", Outbound: "domestic-direct"},
		{Category: CategoryInfrastructure, CIDRs: []string{upstreamB}, Action: "route", Outbound: "foreign-direct"},
	}
	infrastructureA, err := normalizePrefixes(request.InfrastructureA)
	if err != nil {
		return nil, fmt.Errorf("interface A infrastructure: %w", err)
	}
	if len(infrastructureA) > 0 {
		rules = append(rules, Rule{Category: CategoryInfrastructure, CIDRs: infrastructureA, Action: "route", Outbound: "domestic-direct"})
	}
	infrastructureB, err := normalizePrefixes(request.InfrastructureB)
	if err != nil {
		return nil, fmt.Errorf("interface B infrastructure: %w", err)
	}
	if len(infrastructureB) > 0 {
		rules = append(rules, Rule{Category: CategoryInfrastructure, CIDRs: infrastructureB, Action: "route", Outbound: "foreign-direct"})
	}
	seenCustom := make(map[string]string)
	for index, custom := range request.CustomRules {
		var rule Rule
		switch custom.Type {
		case "domain", "domain-suffix":
			values, normalizeErr := normalizeDomains([]string{custom.Value})
			if normalizeErr != nil {
				return nil, fmt.Errorf("custom rule %d: %w", index+1, normalizeErr)
			}
			custom.Value = values[0]
			if custom.Type == "domain" {
				rule = Rule{Category: CategoryUser, ExactDomains: []string{custom.Value}}
			} else {
				rule = Rule{Category: CategoryUser, Domains: []string{custom.Value}}
			}
		case "ip":
			values, normalizeErr := normalizePrefixes([]string{custom.Value})
			if normalizeErr != nil {
				return nil, fmt.Errorf("custom rule %d: %w", index+1, normalizeErr)
			}
			custom.Value = values[0]
			rule = Rule{Category: CategoryUser, CIDRs: []string{custom.Value}}
		case "process-name":
			value := strings.ToLower(strings.TrimSpace(custom.Value))
			if value == "" || strings.ContainsAny(value, `/\\`) || !strings.HasSuffix(value, ".exe") {
				return nil, fmt.Errorf("custom rule %d has invalid process name", index+1)
			}
			custom.Value = value
			rule = Rule{Category: CategoryUser, ProcessName: []string{value}}
		case "process-path":
			value := strings.TrimSpace(custom.Value)
			if !isAbsoluteWindowsPath(value) || !strings.HasSuffix(strings.ToLower(value), ".exe") {
				return nil, fmt.Errorf("custom rule %d has invalid absolute process path", index+1)
			}
			custom.Value = strings.ToLower(value)
			rule = Rule{Category: CategoryUser, ProcessPath: []string{value}}
		default:
			return nil, fmt.Errorf("custom rule %d has unsupported type %q", index+1, custom.Type)
		}
		key := custom.Type + ":" + custom.Value
		decision := custom.Action + ":" + custom.Outbound
		if previous, ok := seenCustom[key]; ok && previous != decision {
			return nil, fmt.Errorf("custom rule %q has conflicting actions", custom.Value)
		}
		if _, ok := seenCustom[key]; ok {
			continue
		}
		seenCustom[key] = decision
		if custom.Action == "reject" {
			rule.Action = "reject"
		} else if custom.Action == "route" && custom.Outbound != "" {
			rule.Action, rule.Outbound = "route", custom.Outbound
		} else {
			return nil, fmt.Errorf("custom rule %d has invalid action", index+1)
		}
		rule.ID = custom.ID
		rules = append(rules, rule)
	}
	for _, prefix := range direct {
		rules = append(rules, Rule{Category: CategoryDirectPrefix, CIDRs: []string{prefix.Prefix}, Action: "route", Outbound: prefix.Outbound})
	}
	reserved := append([]string(nil), reservedCIDRs...)
	if !request.BlockIPv6 {
		reserved = append(reserved, reservedIPv6CIDRs...)
	}
	rules = append(rules, Rule{Category: CategoryReserved, CIDRs: reserved, Action: "reject"})
	if request.BlockIPv6 {
		rules = append(rules, Rule{Category: CategoryReserved, IPVersion: 6, Action: "reject"})
	}
	if len(domesticDomains) > 0 {
		rules = append(rules, Rule{Category: CategoryDomestic, Domains: domesticDomains, Action: "route", Outbound: "domestic-direct"})
	}
	if len(domesticCIDRs) > 0 {
		rules = append(rules, Rule{Category: CategoryDomestic, CIDRs: domesticCIDRs, Action: "route", Outbound: "domestic-direct"})
	}
	return rules, nil
}

func isAbsoluteWindowsPath(value string) bool {
	return len(value) >= 3 && ((value[1] == ':' && (value[2] == '\\' || value[2] == '/')) || strings.HasPrefix(value, `\\`))
}

func normalizeDirectPrefixes(values []DirectPrefix) ([]DirectPrefix, error) {
	byPrefix := make(map[string]string)
	parsed := make(map[string]netip.Prefix)
	for _, value := range values {
		prefix, err := netip.ParsePrefix(value.Prefix)
		if err != nil {
			return nil, fmt.Errorf("direct prefix %q: %w", value.Prefix, err)
		}
		if strings.TrimSpace(value.Outbound) == "" {
			return nil, fmt.Errorf("direct prefix %q has no outbound", value.Prefix)
		}
		key := prefix.Masked().String()
		if existing, ok := byPrefix[key]; ok && existing != value.Outbound {
			return nil, fmt.Errorf("direct prefix %s has ambiguous outbounds %q and %q", key, existing, value.Outbound)
		}
		byPrefix[key] = value.Outbound
		parsed[key] = prefix.Masked()
	}
	keys := make([]string, 0, len(byPrefix))
	for key := range byPrefix {
		keys = append(keys, key)
	}
	for i := 0; i < len(keys); i++ {
		for j := i + 1; j < len(keys); j++ {
			first, second := parsed[keys[i]], parsed[keys[j]]
			if first.Addr().BitLen() == second.Addr().BitLen() && (first.Contains(second.Addr()) || second.Contains(first.Addr())) && byPrefix[keys[i]] != byPrefix[keys[j]] {
				return nil, fmt.Errorf("overlapping direct prefixes %s and %s use different outbounds", first, second)
			}
		}
	}
	result := make([]DirectPrefix, 0, len(byPrefix))
	for prefix, outbound := range byPrefix {
		result = append(result, DirectPrefix{Prefix: prefix, Outbound: outbound})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Prefix < result[j].Prefix })
	return result, nil
}

func normalizePrefixes(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		var prefix netip.Prefix
		if addr, err := netip.ParseAddr(trimmed); err == nil {
			if addr.IsUnspecified() || addr.IsMulticast() {
				return nil, fmt.Errorf("invalid prefix %q", value)
			}
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		} else if p, err := netip.ParsePrefix(trimmed); err == nil {
			if p.Addr().IsUnspecified() || p.Addr().IsMulticast() {
				return nil, fmt.Errorf("invalid prefix %q", value)
			}
			prefix = p.Masked()
		} else {
			return nil, fmt.Errorf("invalid prefix %q", value)
		}
		key := prefix.String()
		if _, ok := seen[key]; !ok {
			seen[key] = struct{}{}
			result = append(result, key)
		}
	}
	sort.Strings(result)
	return result, nil
}

func normalizeDomains(values []string) ([]string, error) {
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.Trim(strings.TrimSpace(value), "."))
		if value == "" || strings.ContainsAny(value, " /\\") {
			return nil, fmt.Errorf("invalid domain suffix %q", value)
		}
		if _, ok := seen[value]; !ok {
			seen[value] = struct{}{}
			result = append(result, value)
		}
	}
	sort.Strings(result)
	return result, nil
}

func hostPrefix(value string) (string, error) {
	ip, err := netip.ParseAddr(value)
	if err != nil {
		return "", fmt.Errorf("must be a fixed IP address")
	}
	return netip.PrefixFrom(ip, ip.BitLen()).String(), nil
}

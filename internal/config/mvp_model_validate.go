package config

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"
)

func ValidateMVPModel(input MVPConfig) error {
	if input.SchemaVersion != SchemaVersion1 {
		return fmt.Errorf("unsupported schema_version %d", input.SchemaVersion)
	}
	if input.DefaultOutbound != "" && input.DefaultOutbound != "a" && input.DefaultOutbound != "b" && input.DefaultOutbound != "c" {
		return fmt.Errorf("unsupported default_outbound %q", input.DefaultOutbound)
	}
	requiresProxy := input.DefaultOutbound == "c"
	for _, ruleSet := range input.RuleSets {
		if ruleSet.Action == "c" {
			requiresProxy = true
		}
	}
	for _, custom := range input.CustomRules {
		if custom.Action == "c" {
			requiresProxy = true
		}
	}
	hasProxy := input.Proxy != nil || len(input.ProxyChain) > 0
	if requiresProxy && !hasProxy {
		return fmt.Errorf("proxy configuration is required when default_outbound or a rule targets proxy (c)")
	}
	if len(input.ProxyChain) > 0 {
		for i, p := range input.ProxyChain {
			if err := validateProxyItem(p, i == 0); err != nil {
				return fmt.Errorf("proxy chain hop %d: %w", i+1, err)
			}
		}
	} else if input.Proxy != nil {
		if err := validateProxyItem(*input.Proxy, true); err != nil {
			return err
		}
	}
	isSingle := input.Mode == ModeSingle || input.InterfaceB.GUID == ""
	if isSingle {
		if strings.TrimSpace(input.InterfaceA.GUID) == "" {
			return fmt.Errorf("interface A GUID is required")
		}
		if strings.TrimSpace(input.InterfaceA.BindInterface) == "" {
			return fmt.Errorf("interface A bind_interface is required")
		}
	} else {
		if strings.TrimSpace(input.InterfaceA.GUID) == "" || strings.TrimSpace(input.InterfaceB.GUID) == "" {
			return fmt.Errorf("two interface GUIDs are required")
		}
		if strings.EqualFold(strings.Trim(input.InterfaceA.GUID, "{}"), strings.Trim(input.InterfaceB.GUID, "{}")) {
			return fmt.Errorf("interface A and B must be different")
		}
		if strings.TrimSpace(input.InterfaceA.BindInterface) == "" || strings.TrimSpace(input.InterfaceB.BindInterface) == "" || strings.EqualFold(input.InterfaceA.BindInterface, input.InterfaceB.BindInterface) {
			return fmt.Errorf("two distinct bind_interface values are required")
		}
	}
	prefix, err := netip.ParsePrefix(input.TUN.Prefix)
	if err != nil || !prefix.Addr().Is4() || !prefix.Addr().IsPrivate() || prefix.Bits() != 30 || prefix != prefix.Masked() {
		return fmt.Errorf("tun.prefix must be a canonical private IPv4 /30")
	}
	ipv6TUN := input.TUN.IPv6Prefix
	if ipv6TUN == "" {
		ipv6TUN = DefaultIPv6TUNPrefix
	}
	v6Prefix, err := netip.ParsePrefix(ipv6TUN)
	if err != nil || !v6Prefix.Addr().Is6() || !v6Prefix.Addr().IsPrivate() || v6Prefix.Bits() != 126 || v6Prefix != v6Prefix.Masked() {
		return fmt.Errorf("tun.ipv6_prefix must be a canonical private IPv6 /126")
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
	if input.DNS.Proxy != nil && input.DNS.Proxy.Server != "" {
		if err := validateDNSServer("proxy", *input.DNS.Proxy); err != nil {
			return err
		}
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
	tunIPv4 := prefix
	for _, direct := range input.DirectPrefixes {
		if strings.TrimSpace(direct.BindInterface) == "" {
			return fmt.Errorf("direct prefix %q requires bind_interface", direct.Prefix)
		}
		dp, err := netip.ParsePrefix(direct.Prefix)
		if err != nil || (input.IPv6 == IPv6Block && !dp.Addr().Is4()) {
			return fmt.Errorf("direct prefix %q is not allowed by IPv6 policy %q", direct.Prefix, input.IPv6)
		}
		if dp.Addr().Is4() && (dp.Overlaps(tunIPv4) || tunIPv4.Overlaps(dp)) {
			return fmt.Errorf("direct prefix %q conflicts with TUN IPv4 prefix %s", direct.Prefix, input.TUN.Prefix)
		}
		if dp.Addr().Is6() && (dp.Overlaps(v6Prefix) || v6Prefix.Overlaps(dp)) {
			return fmt.Errorf("direct prefix %q conflicts with TUN IPv6 prefix %s", direct.Prefix, ipv6TUN)
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
		case "a", "b", "c", "final", "reject":
		default:
			return fmt.Errorf("rule-set %q has unsupported action %q", ruleSet.Tag, ruleSet.Action)
		}
	}
	for index, custom := range input.CustomRules {
		if strings.TrimSpace(custom.Name) == "" || len([]rune(custom.Name)) > 80 {
			return fmt.Errorf("custom rule %d requires a name of at most 80 characters", index+1)
		}
		if custom.Type != "domain" && custom.Type != "domain-suffix" && custom.Type != "ip" && custom.Type != "process-name" && custom.Type != "process-path" {
			return fmt.Errorf("custom rule %q has unsupported type %q", custom.Name, custom.Type)
		}
		switch custom.Action {
		case "a", "b", "c", "final", "reject":
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

func validateProxyItem(p MVPProxy, isEntryNode bool) error {
	switch p.Type {
	case "http", "shadowsocks", "vmess", "vless", "trojan":
	default:
		return fmt.Errorf("unsupported proxy type %q", p.Type)
	}
	if p.Egress != "" && p.Egress != "a" && p.Egress != "b" {
		return fmt.Errorf("unsupported proxy egress %q", p.Egress)
	}
	if isEntryNode {
		address, err := netip.ParseAddr(p.Server)
		if err != nil || (!address.Is4() && !address.Is6()) || address.IsUnspecified() || address.IsLoopback() || address.IsLinkLocalUnicast() || address.IsMulticast() || address.IsPrivate() {
			return fmt.Errorf("proxy server must be a usable fixed IP address")
		}
	} else {
		if strings.TrimSpace(p.Server) == "" {
			return fmt.Errorf("proxy server is required")
		}
	}
	if p.Port == 0 {
		return fmt.Errorf("proxy port is required")
	}
	if p.Type == "http" && p.Password != "" && strings.TrimSpace(p.Username) == "" {
		return fmt.Errorf("proxy username is required when password is set")
	}
	if p.Type == "shadowsocks" && (strings.TrimSpace(p.Method) == "" || p.Password == "") {
		return fmt.Errorf("Shadowsocks method and password are required")
	}
	if p.Type == "vmess" && strings.TrimSpace(p.UUID) == "" {
		return fmt.Errorf("VMess UUID is required")
	}
	if p.Type == "vless" && strings.TrimSpace(p.UUID) == "" {
		return fmt.Errorf("VLESS UUID is required")
	}
	if p.Type == "trojan" && strings.TrimSpace(p.Password) == "" {
		return fmt.Errorf("Trojan password is required")
	}
	if p.Transport != nil && p.Transport.Type != "tcp" && p.Transport.Type != "ws" {
		return fmt.Errorf("unsupported proxy transport %q", p.Transport.Type)
	}
	return nil
}

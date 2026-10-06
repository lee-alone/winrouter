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
	isSingle := input.Mode == ModeSingle || input.InterfaceB.GUID == ""
	interfaceBBind := input.InterfaceB.BindInterface
	if isSingle {
		interfaceBBind = input.InterfaceA.BindInterface
	}
	outbounds := []Outbound{
		{Type: "direct", Tag: "local-direct"},
		{Type: "direct", Tag: "domestic-direct", BindInterface: input.InterfaceA.BindInterface},
		{Type: "direct", Tag: "foreign-direct", BindInterface: interfaceBBind},
	}
	effectiveDefault := input.DefaultOutbound
	if (effectiveDefault == "" || effectiveDefault == "b") && isSingle {
		effectiveDefault = "a"
	}
	finalOutbound := "foreign-direct"
	switch effectiveDefault {
	case "a":
		finalOutbound = "domestic-direct"
	case "c":
		finalOutbound = "proxy"
	}
	var proxyChain []MVPProxy
	if len(input.ProxyChain) > 0 {
		proxyChain = input.ProxyChain
	} else if input.Proxy != nil {
		proxyChain = []MVPProxy{*input.Proxy}
	}

	if len(proxyChain) > 0 {
		entryBind := interfaceBBind
		if isSingle || proxyChain[0].Egress == "a" {
			entryBind = input.InterfaceA.BindInterface
		}
		if len(proxyChain) == 1 {
			outbounds = append(outbounds, buildOutboundFromProxy(proxyChain[0], "proxy", entryBind, ""))
		} else {
			for i, p := range proxyChain {
				if i == 0 {
					outbounds = append(outbounds, buildOutboundFromProxy(p, "proxy-hop-0", entryBind, ""))
				} else if i == len(proxyChain)-1 {
					prevTag := fmt.Sprintf("proxy-hop-%d", i-1)
					outbounds = append(outbounds, buildOutboundFromProxy(p, "proxy", "", prevTag))
				} else {
					tag := fmt.Sprintf("proxy-hop-%d", i)
					prevTag := fmt.Sprintf("proxy-hop-%d", i-1)
					outbounds = append(outbounds, buildOutboundFromProxy(p, tag, "", prevTag))
				}
			}
		}
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
	infrastructureA := []string(nil)
	infrastructureB := []string(nil)
	if len(proxyChain) > 0 {
		if proxyAddress, err := netip.ParseAddr(proxyChain[0].Server); err == nil {
			proxyPrefix := netip.PrefixFrom(proxyAddress, proxyAddress.BitLen()).String()
			if isSingle || proxyChain[0].Egress == "a" {
				infrastructureA = []string{proxyPrefix}
			} else {
				infrastructureB = []string{proxyPrefix}
			}
		}
	}
	customRules := make([]policy.CustomRule, 0, len(input.CustomRules))
	for index, custom := range input.CustomRules {
		customID := custom.ID
		if customID == "" {
			customID = fmt.Sprintf("custom:%d", index)
		}
		mapped := policy.CustomRule{ID: customID, Type: custom.Type, Value: custom.Value}
		switch custom.Action {
		case "reject":
			mapped.Action = "reject"
		case "a":
			mapped.Action, mapped.Outbound = "route", "domestic-direct"
		case "b":
			if isSingle {
				mapped.Action, mapped.Outbound = "route", "domestic-direct"
			} else {
				mapped.Action, mapped.Outbound = "route", "foreign-direct"
			}
		case "c":
			mapped.Action, mapped.Outbound = "route", "proxy"
		case "final":
			mapped.Action, mapped.Outbound = "route", finalOutbound
		}
		customRules = append(customRules, mapped)
	}
	ipv6TUN := input.TUN.IPv6Prefix
	if ipv6TUN == "" {
		ipv6TUN = DefaultIPv6TUNPrefix
	}
	ipv6Prefix := netip.MustParsePrefix(ipv6TUN)
	ipv6InterfaceAddress := netip.PrefixFrom(ipv6Prefix.Addr().Next(), ipv6Prefix.Bits()).String()
	rules, err := policy.BuildDirectSplit(policy.Request{
		DNSUpstreamA: input.DNS.Domestic.Server, DNSUpstreamB: input.DNS.Global.Server,
		TUNIPv6:         ipv6Prefix.Addr().Next().String(),
		InfrastructureA: infrastructureA,
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
	// Keep DNS selection aligned with user domain routing rules. Otherwise a
	// domain can be resolved through B and then connected through A (or vice
	// versa), which is particularly harmful for CDN-backed services.
	for _, rule := range input.CustomRules {
		if rule.Type != "domain" && rule.Type != "domain-suffix" {
			continue
		}
		var action, server string
		switch rule.Action {
		case "reject":
			action = "reject"
		case "a":
			action, server = "route", "dns-domestic"
		case "b":
			if isSingle {
				action, server = "route", "dns-domestic"
			} else {
				action, server = "route", "dns-global"
			}
		case "c":
			action, server = "route", "dns-proxy"
		case "final":
			action, server = "route", finalDNSResolver(effectiveDefault)
		default:
			continue
		}
		normalized := normalizedDomainCopy([]string{rule.Value})
		if len(normalized) == 0 {
			continue
		}
		dnsRule := DNSRule{Action: action, Server: server}
		if rule.Type == "domain" {
			dnsRule.Domain = normalized
		} else {
			dnsRule.DomainSuffix = normalized
		}
		dnsRules = append(dnsRules, dnsRule)
	}
	if domains := normalizedDomainCopy(input.Domestic.DomainSuffixes); len(domains) > 0 {
		dnsRules = append(dnsRules, DNSRule{DomainSuffix: domains, Action: "route", Server: "dns-domestic"})
	}
	for _, ruleSet := range input.RuleSets {
		if ruleSet.Kind != "domain" {
			continue
		}
		action, server := "", ""
		switch ruleSet.Action {
		case "a":
			action, server = "route", "dns-domestic"
		case "b":
			if isSingle {
				action, server = "route", "dns-domestic"
			} else {
				action, server = "route", "dns-global"
			}
		case "c":
			action, server = "route", "dns-proxy"
		case "final":
			action, server = "route", finalDNSResolver(effectiveDefault)
		case "reject":
			action = "reject"
		}
		if action != "" {
			dnsRules = append(dnsRules, DNSRule{RuleSet: []string{ruleSet.Tag}, Action: action, Server: server})
		}
	}
	finalDNS := finalDNSResolver(effectiveDefault)
	routeRuleSets := make([]RuleSet, 0, len(input.RuleSets))
	for _, ruleSet := range input.RuleSets {
		routeRuleSets = append(routeRuleSets, RuleSet{Type: "local", Tag: ruleSet.Tag, Format: "binary", Path: ruleSet.Path})
	}
	dnsServers := []DNSServer{
		dnsServer("dns-domestic", input.DNS.Domestic, "domestic-direct"),
		dnsServer("dns-global", input.DNS.Global, "foreign-direct"),
	}
	if len(proxyChain) > 0 {
		proxyDNS := input.DNS.Global
		if input.DNS.Proxy != nil && input.DNS.Proxy.Server != "" {
			proxyDNS = *input.DNS.Proxy
		}
		dnsServers = append(dnsServers, dnsServer("dns-proxy", proxyDNS, "proxy"))
	}
	model := MinimalTUN{
		Log: LogConfig{Level: "info", Timestamp: true},
		DNS: DNSConfig{
			Servers: dnsServers,
			Rules:   dnsRules,
			Final:   finalDNS, Strategy: dnsStrategy(input.IPv6), IndependentCache: true,
		},
		Inbounds:  []TUNInbound{{Type: "tun", Tag: "tun-in", InterfaceName: "WinRouter-TUN", Address: []string{netip.PrefixFrom(interfacePrefix.Addr().Next(), 30).String(), ipv6InterfaceAddress}, AutoRoute: true, StrictRoute: true, Stack: input.TUN.Stack}},
		Outbounds: outbounds,
		Route:     RouteConfig{AutoDetectInterface: false, DefaultDNSResolver: finalDNS, RuleSets: routeRuleSets, Rules: make([]RouteRule, 0, len(rules)+len(input.RuleSets)), Final: finalOutbound},
	}
	if input.ConnectionObservation {
		secret := strings.TrimSpace(input.ConnectionAPISecret)
		if secret == "" {
			return Generated{}, fmt.Errorf("connection observation API secret is required when enabled")
		}
		model.Experimental = &ExperimentalConfig{ClashAPI: &ClashAPIConfig{ExternalController: "127.0.0.1:19090", Secret: secret}}
	}
	categories := make([]string, 0, len(rules))
	// Keep infrastructure, reserved, and domestic safety rules in their fixed positions;
	// only the user rule and SRS rule-set region follows the configured order.
	type orderedEntry struct {
		route    RouteRule
		category string
		key      string
	}
	entries := make(map[string][]orderedEntry)
	for _, rule := range rules {
		if rule.Category != policy.CategoryUser {
			continue
		}
		entries[rule.ID] = append(entries[rule.ID], orderedEntry{route: RouteRule{Protocol: rule.Protocol, IPCIDR: rule.CIDRs, Domain: rule.ExactDomains, DomainSuffix: rule.Domains, IPVersion: rule.IPVersion, ProcessName: rule.ProcessName, ProcessPath: rule.ProcessPath, Action: rule.Action, Outbound: rule.Outbound}, category: string(rule.Category), key: rule.ID})
	}
	for _, ruleSet := range input.RuleSets {
		action, outbound := "route", ""
		switch ruleSet.Action {
		case "a":
			outbound = "domestic-direct"
		case "b":
			if isSingle {
				outbound = "domestic-direct"
			} else {
				outbound = "foreign-direct"
			}
		case "c":
			outbound = "proxy"
		case "final":
			outbound = finalOutbound
		case "reject":
			action = "reject"
		}
		key := "srs:" + strings.TrimPrefix(ruleSet.Tag, "winrouter-")
		entries[key] = []orderedEntry{{route: RouteRule{RuleSet: []string{ruleSet.Tag}, Action: action, Outbound: outbound}, category: "rule-set", key: key}}
	}
	ordered := make([]orderedEntry, 0, len(rules)+len(input.RuleSets))
	seen := make(map[string]bool)
	for _, key := range input.RuleOrder {
		if group, ok := entries[key]; ok && !seen[key] {
			ordered = append(ordered, group...)
			seen[key] = true
		}
	}
	for _, rule := range rules {
		if rule.Category == policy.CategoryUser && !seen[rule.ID] {
			ordered = append(ordered, entries[rule.ID]...)
			seen[rule.ID] = true
		}
	}
	for _, ruleSet := range input.RuleSets {
		key := "srs:" + strings.TrimPrefix(ruleSet.Tag, "winrouter-")
		if !seen[key] {
			ordered = append(ordered, entries[key]...)
			seen[ruleSet.Tag] = true
			seen[key] = true
		}
	}
	inserted := false
	insertOrdered := func() {
		for _, entry := range ordered {
			model.Route.Rules = append(model.Route.Rules, entry.route)
			categories = append(categories, entry.category)
		}
		inserted = true
	}
	for _, rule := range rules {
		if rule.Category == policy.CategoryUser {
			if !inserted {
				insertOrdered()
			}
			continue
		}
		if !inserted && (rule.Category == policy.CategoryDirectPrefix || rule.Category == policy.CategoryReserved || rule.Category == policy.CategoryDomestic) {
			insertOrdered()
		}
		model.Route.Rules = append(model.Route.Rules, RouteRule{Protocol: rule.Protocol, IPCIDR: rule.CIDRs, Domain: rule.ExactDomains, DomainSuffix: rule.Domains, IPVersion: rule.IPVersion, ProcessName: rule.ProcessName, ProcessPath: rule.ProcessPath, Action: rule.Action, Outbound: rule.Outbound})
		categories = append(categories, string(rule.Category))
	}
	if !inserted {
		insertOrdered()
	}
	data, err := json.MarshalIndent(model, "", "  ")
	if err != nil {
		return Generated{}, fmt.Errorf("marshal MVP configuration: %w", err)
	}
	proxyBindInterface := ""
	if len(proxyChain) > 0 {
		proxyBindInterface = interfaceBBind
		if isSingle || proxyChain[0].Egress == "a" {
			proxyBindInterface = input.InterfaceA.BindInterface
		}
	}
	generated := Generated{Model: model, JSON: append(data, '\n'), RuleCategories: categories, ProxyBindInterface: proxyBindInterface}
	if err := ValidateGeneratedSchema(generated.JSON); err != nil {
		return Generated{}, err
	}
	if err := ValidateMVPSemantics(generated); err != nil {
		return Generated{}, err
	}
	return generated, nil
}

func PreviewMVPRules(input MVPConfig) ([]RulePreview, error) {
	generated, err := GenerateMVP(input)
	if err != nil {
		return nil, err
	}
	result := make([]RulePreview, 0, len(generated.Model.Route.Rules)+1)
	for index, rule := range generated.Model.Route.Rules {
		matches := append([]string(nil), rule.Domain...)
		matches = append(matches, rule.DomainSuffix...)
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

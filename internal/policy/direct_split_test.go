package policy

import "testing"

func TestBuildDirectSplitProducesDeterministicPriority(t *testing.T) {
	rules, err := BuildDirectSplit(Request{
		DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", BlockIPv6: true,
		DirectPrefixes: []DirectPrefix{{Prefix: "192.168.20.3/24", Outbound: "b"}, {Prefix: "192.168.10.2/24", Outbound: "a"}},
		DomesticCIDRs:  []string{"1.0.1.1/24"}, DomesticDomains: []string{"Example.CN."},
	})
	if err != nil {
		t.Fatal(err)
	}
	ranks := map[Category]int{CategoryMetadata: 0, CategoryLocal: 1, CategoryInfrastructure: 2, CategoryDirectPrefix: 3, CategoryReserved: 4, CategoryDomestic: 5}
	previous := -1
	for index, rule := range rules {
		rank := ranks[rule.Category]
		if rank < previous {
			t.Fatalf("rule %d category %q follows rank %d", index, rule.Category, previous)
		}
		previous = rank
	}
	if rules[5].CIDRs[0] != "192.168.10.0/24" || rules[6].CIDRs[0] != "192.168.20.0/24" {
		t.Fatalf("direct rules are not deterministic: %#v", rules)
	}
	if rules[len(rules)-2].Domains[0] != "example.cn" || rules[len(rules)-1].CIDRs[0] != "1.0.1.0/24" {
		t.Fatalf("domestic rules = %#v", rules[len(rules)-2:])
	}
}

func TestBuildDirectSplitRejectsAmbiguousAndInvalidInputs(t *testing.T) {
	tests := []Request{
		{DNSUpstreamA: "dns.example", DNSUpstreamB: "1.1.1.1"},
		{DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", DirectPrefixes: []DirectPrefix{{Prefix: "bad", Outbound: "a"}}},
		{DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", DirectPrefixes: []DirectPrefix{{Prefix: "10.0.0.1/24", Outbound: "a"}, {Prefix: "10.0.0.2/24", Outbound: "b"}}},
		{DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", DirectPrefixes: []DirectPrefix{{Prefix: "10.0.0.0/24", Outbound: "a"}, {Prefix: "10.0.0.0/25", Outbound: "b"}}},
		{DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", DomesticDomains: []string{"bad domain"}},
	}
	for index, request := range tests {
		if _, err := BuildDirectSplit(request); err == nil {
			t.Fatalf("case %d succeeded", index)
		}
	}
}

func TestBuildDirectSplitPlacesCustomRulesBeforeDirectPrefixes(t *testing.T) {
	rules, err := BuildDirectSplit(Request{
		DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", BlockIPv6: true,
		CustomRules:    []CustomRule{{Type: "domain-suffix", Value: "Example.COM.", Action: "reject"}, {Type: "ip", Value: "8.8.8.8/32", Action: "route", Outbound: "a"}},
		DirectPrefixes: []DirectPrefix{{Prefix: "192.168.1.0/24", Outbound: "a"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	customIndexes, directIndex := []int{}, -1
	for index, rule := range rules {
		if rule.Category == CategoryUser {
			customIndexes = append(customIndexes, index)
		}
		if rule.Category == CategoryDirectPrefix {
			directIndex = index
		}
	}
	if len(customIndexes) != 2 || customIndexes[1] >= directIndex {
		t.Fatalf("custom/direct order = %#v / %d", customIndexes, directIndex)
	}
	if rules[customIndexes[0]].Domains[0] != "example.com" || rules[customIndexes[0]].Action != "reject" {
		t.Fatalf("domain rule = %#v", rules[customIndexes[0]])
	}
}

func TestBuildDirectSplitRejectsConflictingCustomRules(t *testing.T) {
	_, err := BuildDirectSplit(Request{DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", CustomRules: []CustomRule{
		{Type: "domain", Value: "example.com", Action: "reject"},
		{Type: "domain", Value: "EXAMPLE.COM.", Action: "route", Outbound: "a"},
	}})
	if err == nil {
		t.Fatal("conflicting custom rules accepted")
	}
}

func TestBuildDirectSplitPreservesUserRuleOrderAndValidatesIdentity(t *testing.T) {
	rules, err := BuildDirectSplit(Request{DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", CustomRules: []CustomRule{
		{Type: "domain", Value: "example.com", Action: "reject"},
		{Type: "process-name", Value: "Browser.EXE", Action: "route", Outbound: "b"},
		{Type: "process-path", Value: `C:\Tools\browser.exe`, Action: "route", Outbound: "a"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	users := []Rule{}
	for _, rule := range rules {
		if rule.Category == CategoryUser {
			users = append(users, rule)
		}
	}
	if len(users) != 3 || users[0].ExactDomains[0] != "example.com" || users[1].ProcessName[0] != "browser.exe" || users[2].ProcessPath[0] != `C:\Tools\browser.exe` {
		t.Fatalf("user rule order = %#v", users)
	}
	for _, value := range []CustomRule{{Type: "process-name", Value: `C:\bad.exe`, Action: "reject"}, {Type: "process-path", Value: "relative.exe", Action: "reject"}} {
		if _, err := BuildDirectSplit(Request{DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", CustomRules: []CustomRule{value}}); err == nil {
			t.Fatalf("invalid process identity accepted: %#v", value)
		}
	}
}

func TestBuildDirectSplitWithIPv6UpstreamsAndCustomRules(t *testing.T) {
	rules, err := BuildDirectSplit(Request{
		DNSUpstreamA: "2400:3200::1",
		DNSUpstreamB: "2001:4860:4860::8888",
		BlockIPv6:    false,
		CustomRules: []CustomRule{
			{Type: "ip", Value: "2001:db8::1", Action: "route", Outbound: "a"},
			{Type: "ip", Value: "2001:db8:1234::/48", Action: "route", Outbound: "b"},
		},
		DirectPrefixes: []DirectPrefix{
			{Prefix: "2400:3200:1000::/64", Outbound: "a"},
		},
		DomesticCIDRs: []string{"2400:3200::/32", "1.0.1.0/24"},
	})
	if err != nil {
		t.Fatalf("BuildDirectSplit() error: %v", err)
	}

	// Verify DNS upstreams converted to /128
	var upstreamARule, upstreamBRule *Rule
	for index := range rules {
		if rules[index].Category == CategoryInfrastructure && len(rules[index].CIDRs) == 1 {
			if rules[index].CIDRs[0] == "2400:3200::1/128" {
				upstreamARule = &rules[index]
			}
			if rules[index].CIDRs[0] == "2001:4860:4860::8888/128" {
				upstreamBRule = &rules[index]
			}
		}
	}
	if upstreamARule == nil || upstreamARule.Outbound != "domestic-direct" {
		t.Fatalf("upstream A rule = %#v", upstreamARule)
	}
	if upstreamBRule == nil || upstreamBRule.Outbound != "foreign-direct" {
		t.Fatalf("upstream B rule = %#v", upstreamBRule)
	}

	// Verify reserved category includes IPv6 reserved CIDRs when BlockIPv6 is false
	var reservedRule *Rule
	for index := range rules {
		if rules[index].Category == CategoryReserved {
			reservedRule = &rules[index]
			break
		}
	}
	if reservedRule == nil {
		t.Fatal("reserved rule not found")
	}
	hasV6Reserved := false
	for _, cidr := range reservedRule.CIDRs {
		if cidr == "fc00::/7" {
			hasV6Reserved = true
			break
		}
	}
	if !hasV6Reserved {
		t.Fatalf("reserved rule does not contain IPv6 reserved CIDRs: %#v", reservedRule.CIDRs)
	}
}

func TestNormalizePrefixesMixedIPv4AndIPv6(t *testing.T) {
	input := []string{"192.168.1.1", "2001:db8::1", "10.0.0.5/24", "2001:db8:abcd::1/48", "192.168.1.1/32"}
	got, err := normalizePrefixes(input)
	if err != nil {
		t.Fatalf("normalizePrefixes() error: %v", err)
	}
	want := []string{"10.0.0.0/24", "192.168.1.1/32", "2001:db8::1/128", "2001:db8:abcd::/48"}
	if len(got) != len(want) {
		t.Fatalf("normalizePrefixes() = %#v, want %#v", got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("got[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

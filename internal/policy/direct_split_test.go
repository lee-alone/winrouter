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
		CustomRules:    []CustomRule{{Type: "domain", Value: "Example.COM.", Action: "reject"}, {Type: "ip", Value: "8.8.8.8/32", Action: "route", Outbound: "a"}},
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

func TestBuildDirectSplitPrioritizesProcessRulesAndValidatesIdentity(t *testing.T) {
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
	if len(users) != 3 || users[0].ProcessName[0] != "browser.exe" || users[1].ProcessPath[0] != `C:\Tools\browser.exe` || users[2].Domains[0] != "example.com" {
		t.Fatalf("user rule order = %#v", users)
	}
	for _, value := range []CustomRule{{Type: "process-name", Value: `C:\bad.exe`, Action: "reject"}, {Type: "process-path", Value: "relative.exe", Action: "reject"}} {
		if _, err := BuildDirectSplit(Request{DNSUpstreamA: "223.5.5.5", DNSUpstreamB: "1.1.1.1", CustomRules: []CustomRule{value}}); err == nil {
			t.Fatalf("invalid process identity accepted: %#v", value)
		}
	}
}

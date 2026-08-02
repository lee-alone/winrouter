package main

import (
	"testing"

	"winrouter/internal/config"
	"winrouter/internal/interfaces"
)

func TestProcessExperimentInputKeepsRuleAndDirectPrefixes(t *testing.T) {
	candidates := []interfaces.Adapter{
		{GUID: "{A}", FriendlyName: "Wi-Fi", Kind: interfaces.KindWiFi, Candidate: true, Status: "up", Addresses: []interfaces.Address{{IP: "192.168.10.2", PrefixLength: 24}}},
		{GUID: "{B}", FriendlyName: "Ethernet", Kind: interfaces.KindEthernet, Candidate: true, Status: "up", Addresses: []interfaces.Address{{IP: "192.168.20.2", PrefixLength: 24}}},
	}
	rule := config.MVPCustomRule{Name: "probe", Type: "process-name", Value: "winrouter-probe.exe", Action: "a"}
	input := processExperimentInput("172.19.0.0/30", config.TUNStackSystem, candidates, rule)
	if len(input.CustomRules) != 1 || input.CustomRules[0] != rule {
		t.Fatalf("custom rules = %#v", input.CustomRules)
	}
	if len(input.DirectPrefixes) != 2 {
		t.Fatalf("direct prefixes = %#v", input.DirectPrefixes)
	}
	if _, err := config.GenerateMVP(input); err != nil {
		t.Fatalf("GenerateMVP() error: %v", err)
	}
}

func TestActiveCandidatesFiltersUnavailableAdapters(t *testing.T) {
	adapters := []interfaces.Adapter{
		{GUID: "{A}", Candidate: true, Status: "up", Addresses: []interfaces.Address{{IP: "192.168.1.2", PrefixLength: 24}}},
		{GUID: "{DOWN}", Candidate: true, Status: "down", Addresses: []interfaces.Address{{IP: "192.168.2.2", PrefixLength: 24}}},
		{GUID: "{VIRTUAL}", Candidate: false, Status: "up", Addresses: []interfaces.Address{{IP: "192.168.3.2", PrefixLength: 24}}},
	}
	result := activeCandidates(adapters)
	if len(result) != 1 || result[0].GUID != "{A}" {
		t.Fatalf("active candidates = %#v", result)
	}
}

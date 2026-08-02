package config

import (
	"encoding/json"
	"testing"
)

func TestGenerateDualDirectTUN(t *testing.T) {
	data, err := GenerateDualDirectTUN("172.19.0.0/30", TUNStackSystem, "WLAN", "Ethernet", "1.1.1.1/32", "8.8.8.8/32", "162.159.200.1/32", "162.159.200.123/32")
	if err != nil {
		t.Fatalf("GenerateDualDirectTUN() error: %v", err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	if len(model.Outbounds) != 2 || model.Outbounds[0].BindInterface != "WLAN" || model.Outbounds[1].BindInterface != "Ethernet" {
		t.Fatalf("outbounds = %#v", model.Outbounds)
	}
	if model.Route.Final != "b-direct" || len(model.Route.Rules) != 4 || model.Route.Rules[1].Outbound != "a-direct" || model.Route.Rules[2].Outbound != "b-direct" || model.Route.Rules[3].Action != "hijack-dns" {
		t.Fatalf("route = %#v", model.Route)
	}
	if model.DNS.Servers[0].Detour != "b-direct" {
		t.Fatalf("DNS server = %#v", model.DNS.Servers[0])
	}
}

func TestGenerateDualDirectTUNRejectsInvalidSelection(t *testing.T) {
	for _, test := range []struct{ a, b, targetA, targetB string }{
		{"", "Ethernet", "1.1.1.1/32", "8.8.8.8/32"},
		{"WLAN", "WLAN", "1.1.1.1/32", "8.8.8.8/32"},
		{"WLAN", "Ethernet", "1.1.1.0/24", "8.8.8.8/32"},
	} {
		if _, err := GenerateDualDirectTUN("172.19.0.0/30", TUNStackSystem, test.a, test.b, test.targetA, test.targetB, "162.159.200.1/32", "162.159.200.123/32"); err == nil {
			t.Fatalf("GenerateDualDirectTUN(%q, %q, %q, %q) succeeded", test.a, test.b, test.targetA, test.targetB)
		}
	}
}

func TestGenerateDualLANDirectTUN(t *testing.T) {
	data, err := GenerateDualLANDirectTUN("172.19.0.0/30", TUNStackSystem, "WLAN", "Ethernet", "10.12.85.0/24", "192.168.1.0/24")
	if err != nil {
		t.Fatal(err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	if model.Route.Rules[1].IPCIDR[0] != "10.12.85.0/24" || model.Route.Rules[1].Outbound != "a-direct" || model.Route.Rules[2].IPCIDR[0] != "192.168.1.0/24" || model.Route.Rules[2].Outbound != "b-direct" {
		t.Fatalf("route rules = %#v", model.Route.Rules)
	}
}

func TestGenerateDualDNSTUN(t *testing.T) {
	data, err := GenerateDualDNSTUN("172.19.0.0/30", TUNStackSystem, "WLAN", "Ethernet")
	if err != nil {
		t.Fatal(err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	if len(model.DNS.Servers) != 2 || model.DNS.Servers[0].Detour != "a-direct" || model.DNS.Servers[1].Detour != "b-direct" {
		t.Fatalf("DNS servers = %#v", model.DNS.Servers)
	}
	if !model.DNS.IndependentCache || len(model.DNS.Rules) != 2 || model.DNS.Rules[0].Server != "dns-a" || model.DNS.Rules[1].Server != "dns-b" {
		t.Fatalf("DNS policy = %#v", model.DNS)
	}
	if model.Route.DefaultDNSResolver != "dns-b" {
		t.Fatalf("default domain resolver = %q", model.Route.DefaultDNSResolver)
	}
}

func TestGenerateIPv6BlockedTUN(t *testing.T) {
	data, err := GenerateIPv6BlockedTUN("172.19.0.0/30", TUNStackSystem, "WLAN", "Ethernet")
	if err != nil {
		t.Fatal(err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	if len(model.Inbounds[0].Address) != 2 || model.Inbounds[0].Address[1] != "fdfe:dcba:9876::1/126" {
		t.Fatalf("TUN addresses = %#v", model.Inbounds[0].Address)
	}
	if len(model.Route.Rules) < 2 || model.Route.Rules[len(model.Route.Rules)-2].Action != "hijack-dns" || model.Route.Rules[len(model.Route.Rules)-1].IPVersion != 6 || model.Route.Rules[len(model.Route.Rules)-1].Action != "reject" {
		t.Fatalf("IPv6 rule = %#v", model.Route.Rules)
	}
}

func TestGenerateEncryptedDNSTUN(t *testing.T) {
	data, err := GenerateEncryptedDNSTUN("172.19.0.0/30", TUNStackSystem, "WLAN", "Ethernet")
	if err != nil {
		t.Fatal(err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	if model.DNS.Servers[0].Type != "tls" || model.DNS.Servers[0].ServerPort != 853 || model.DNS.Servers[0].TLS.ServerName != "cloudflare-dns.com" {
		t.Fatalf("DoT server = %#v", model.DNS.Servers[0])
	}
	if model.DNS.Servers[1].Type != "https" || model.DNS.Servers[1].Path != "/dns-query" || model.DNS.Servers[1].TLS.ServerName != "dns.google" {
		t.Fatalf("DoH server = %#v", model.DNS.Servers[1])
	}
}

func TestGenerateFakeIPTUN(t *testing.T) {
	data, err := GenerateFakeIPTUN("172.19.0.0/30", TUNStackSystem, "WLAN", "Ethernet")
	if err != nil {
		t.Fatal(err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	fake := model.DNS.Servers[len(model.DNS.Servers)-1]
	if fake.Type != "fakeip" || fake.Inet4Range != "198.18.0.0/15" || fake.Inet6Range != "fc00::/18" {
		t.Fatalf("fake-IP server = %#v", fake)
	}
	if len(model.DNS.Rules) != 1 || model.DNS.Rules[0].Server != "dns-fakeip" || len(model.DNS.Rules[0].QueryType) != 1 || model.DNS.Rules[0].QueryType[0] != "A" {
		t.Fatalf("fake-IP DNS rule = %#v", model.DNS.Rules)
	}
}

func TestGenerateProxyLoopTUN(t *testing.T) {
	data, err := GenerateProxyLoopTUN("172.19.0.0/30", TUNStackSystem, "WLAN", "Ethernet", "192.168.1.166", 18080, "1.1.1.1/32")
	if err != nil {
		t.Fatal(err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	proxy := model.Outbounds[len(model.Outbounds)-1]
	if proxy.Type != "http" || proxy.Tag != "proxy" || proxy.BindInterface != "Ethernet" || proxy.DomainResolver != "dns-b" {
		t.Fatalf("proxy outbound = %#v", proxy)
	}
	if model.Route.Rules[1].Outbound != "b-direct" || model.Route.Rules[1].IPCIDR[0] != "192.168.1.166/32" || model.Route.Rules[2].Outbound != "proxy" {
		t.Fatalf("proxy loop rules = %#v", model.Route.Rules)
	}
}

func TestGenerateDomainProxyLoopTUN(t *testing.T) {
	data, err := GenerateDomainProxyLoopTUN("172.19.0.0/30", TUNStackSystem, "WLAN", "Ethernet", "proxy.winrouter.test", "192.168.1.166", 18080, "1.1.1.1/32")
	if err != nil {
		t.Fatal(err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	proxy := model.Outbounds[len(model.Outbounds)-1]
	if proxy.Server != "proxy.winrouter.test" || proxy.DomainResolver != "proxy-bootstrap" {
		t.Fatalf("domain proxy outbound = %#v", proxy)
	}
	bootstrap := model.DNS.Servers[len(model.DNS.Servers)-1]
	if bootstrap.Type != "hosts" || bootstrap.Predefined["proxy.winrouter.test"][0] != "192.168.1.166" {
		t.Fatalf("proxy bootstrap DNS = %#v", bootstrap)
	}
	if model.Route.Rules[0].DomainSuffix[0] != "proxy.winrouter.test" || model.Route.Rules[0].Outbound != "b-direct" {
		t.Fatalf("proxy domain bypass = %#v", model.Route.Rules[0])
	}
}

func TestGenerateDomainProxyLoopTUNReplacesResolvedAddress(t *testing.T) {
	data, err := GenerateDomainProxyLoopTUN("172.19.0.0/30", TUNStackSystem, "WLAN", "Ethernet", "proxy.winrouter.test", "192.168.1.200", 18080, "1.1.1.1/32")
	if err != nil {
		t.Fatal(err)
	}
	var model MinimalTUN
	if err := json.Unmarshal(data, &model); err != nil {
		t.Fatal(err)
	}
	bootstrap := model.DNS.Servers[len(model.DNS.Servers)-1]
	if bootstrap.Predefined["proxy.winrouter.test"][0] != "192.168.1.200" {
		t.Fatalf("updated bootstrap DNS = %#v", bootstrap)
	}
	for _, rule := range model.Route.Rules {
		for _, prefix := range rule.IPCIDR {
			if prefix == "192.168.1.166/32" {
				t.Fatal("stale proxy address remained in regenerated rules")
			}
		}
	}
}

package recovery

import (
	"testing"

	"winrouter/internal/config"
	"winrouter/internal/interfacemanager"
	"winrouter/internal/interfaces"
	"winrouter/internal/tunprefix"
)

func TestRebuildConfigUsesCurrentBindingsAndIPv4Prefixes(t *testing.T) {
	first := interfaces.Adapter{GUID: "{NEW-A}", FriendlyName: "Wi-Fi 2", Status: "up"}
	second := interfaces.Adapter{GUID: "{B}", FriendlyName: "Ethernet", Status: "up"}
	allocation := tunprefix.Allocation{Prefix: "172.19.0.4/30"}
	snapshot := interfacemanager.Snapshot{
		InterfaceA: interfacemanager.ResolvedSelection{Status: "resolved", Match: &interfaces.Match{Adapter: first}},
		InterfaceB: interfacemanager.ResolvedSelection{Status: "resolved", Match: &interfaces.Match{Adapter: second}},
		TUN:        &allocation,
		Topology: interfaces.Topology{Prefixes: []interfaces.DirectPrefix{
			{Prefix: "192.168.10.0/24", AdapterGUID: "new-a", AdapterName: "Wi-Fi 2", Action: interfaces.PrefixBindInterface},
			{Prefix: "192.168.20.0/24", AdapterGUID: "b", AdapterName: "Ethernet", Action: interfaces.PrefixBindInterface},
			{Prefix: "fe80::/64", AdapterGUID: "b", AdapterName: "Ethernet", Action: interfaces.PrefixBindInterface},
		}},
	}
	base := validConfig()
	rebuilt, err := RebuildConfig(base, snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if rebuilt.InterfaceA.GUID != first.GUID || rebuilt.InterfaceA.BindInterface != first.FriendlyName || rebuilt.TUN.Prefix != allocation.Prefix {
		t.Fatalf("rebuilt identity = %#v TUN=%q", rebuilt.InterfaceA, rebuilt.TUN.Prefix)
	}
	if len(rebuilt.DirectPrefixes) != 2 {
		t.Fatalf("direct prefixes = %#v", rebuilt.DirectPrefixes)
	}
}

func TestRebuildConfigRejectsUnavailableSnapshot(t *testing.T) {
	_, err := RebuildConfig(validConfig(), interfacemanager.Snapshot{})
	if err == nil {
		t.Fatal("expected unavailable snapshot error")
	}
}

func validConfig() config.MVPConfig {
	return config.MVPConfig{
		SchemaVersion: config.SchemaVersion1,
		TUN:           config.MVPTUN{Prefix: "172.19.0.0/30", Stack: "system"},
		InterfaceA:    config.MVPInterface{GUID: "{A}", BindInterface: "Wi-Fi"},
		InterfaceB:    config.MVPInterface{GUID: "{B}", BindInterface: "Ethernet"},
		Domestic:      config.MVPDomestic{CIDRs: []string{"1.0.1.0/24"}, DomainSuffixes: []string{"cn"}},
		DNS: config.MVPDNS{
			Domestic: config.MVPDNSServer{Type: "udp", Server: "223.5.5.5", Port: 53},
			Global:   config.MVPDNSServer{Type: "tls", Server: "1.1.1.1", Port: 853, ServerName: "cloudflare-dns.com"},
		},
		IPv6: config.IPv6Block,
	}
}

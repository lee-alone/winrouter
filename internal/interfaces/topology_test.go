package interfaces

import "testing"

func TestBuildTopologyMapsPhysicalAndVirtualPrefixes(t *testing.T) {
	physical := testAdapter("{PHYSICAL}", "00:11:22:33:44:55", "Ethernet", "192.168.10.23", 24)
	physical.Status = "up"
	physical.IPv4Metric = 25
	virtual := Adapter{
		GUID: "{VIRTUAL}", FriendlyName: "vEthernet", Kind: KindHyperV, Status: "up", IPv4Metric: 35,
		Addresses: []Address{{IP: "172.24.3.1", PrefixLength: 20}},
	}
	loopback := Adapter{
		GUID: "{LOOPBACK}", FriendlyName: "Loopback", Kind: KindLoopback, Status: "up",
		Addresses: []Address{{IP: "127.0.0.1", PrefixLength: 8}},
	}

	topology := BuildTopology([]Adapter{virtual, loopback, physical})
	if len(topology.Prefixes) != 3 || len(topology.Overlaps) != 0 {
		t.Fatalf("BuildTopology() = %#v", topology)
	}
	want := map[string]PrefixAction{
		"127.0.0.0/8": PrefixLocal, "172.24.0.0/20": PrefixBypassTUN, "192.168.10.0/24": PrefixBindInterface,
	}
	for _, prefix := range topology.Prefixes {
		if prefix.Action != want[prefix.Prefix] {
			t.Errorf("prefix %s action = %q, want %q", prefix.Prefix, prefix.Action, want[prefix.Prefix])
		}
	}
}

func TestBuildTopologyDetectsEqualAndContainedPrefixes(t *testing.T) {
	first := testAdapter("{A}", "00:00:00:00:00:01", "A", "192.168.10.2", 24)
	second := testAdapter("{B}", "00:00:00:00:00:02", "B", "192.168.10.3", 24)
	third := testAdapter("{C}", "00:00:00:00:00:03", "C", "192.168.10.129", 25)

	topology := BuildTopology([]Adapter{first, second, third})
	if len(topology.Overlaps) != 3 {
		t.Fatalf("overlaps = %#v, want 3", topology.Overlaps)
	}
	equal, contained := 0, 0
	for _, overlap := range topology.Overlaps {
		if !overlap.RequiresSelection {
			t.Errorf("private overlap must require selection: %#v", overlap)
		}
		switch overlap.Kind {
		case OverlapEqual:
			equal++
		case OverlapContains:
			contained++
		}
	}
	if equal != 1 || contained != 2 {
		t.Fatalf("equal=%d contained=%d", equal, contained)
	}
}

func TestBuildTopologyTreatsIPv6LinkLocalOverlapAsScoped(t *testing.T) {
	first := testAdapter("{A}", "", "A", "fe80::1", 64)
	second := testAdapter("{B}", "", "B", "fe80::2", 64)

	topology := BuildTopology([]Adapter{first, second})
	if len(topology.Overlaps) != 1 || topology.Overlaps[0].RequiresSelection {
		t.Fatalf("link-local overlaps = %#v, want scoped diagnostic", topology.Overlaps)
	}
}

func TestBuildTopologyTreatsIPv4LinkLocalOverlapAsScoped(t *testing.T) {
	first := testAdapter("{A}", "", "A", "169.254.1.2", 16)
	second := testAdapter("{B}", "", "B", "169.254.2.3", 16)

	topology := BuildTopology([]Adapter{first, second})
	if len(topology.Overlaps) != 1 || topology.Overlaps[0].RequiresSelection {
		t.Fatalf("link-local overlaps = %#v, want scoped diagnostic", topology.Overlaps)
	}
}

func TestBuildTopologySkipsInvalidAndDuplicateAddresses(t *testing.T) {
	adapter := testAdapter("{A}", "", "A", "10.0.0.2", 24)
	adapter.Addresses = append(adapter.Addresses,
		Address{IP: "10.0.0.3", PrefixLength: 24},
		Address{IP: "invalid", PrefixLength: 24},
		Address{IP: "10.0.0.4", PrefixLength: 33},
		Address{IP: "224.0.0.1", PrefixLength: 24},
	)

	topology := BuildTopology([]Adapter{adapter})
	if len(topology.Prefixes) != 1 || topology.Prefixes[0].Prefix != "10.0.0.0/24" {
		t.Fatalf("prefixes = %#v", topology.Prefixes)
	}
}

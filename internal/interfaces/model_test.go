package interfaces

import "testing"

func TestClassify(t *testing.T) {
	tests := []struct {
		name        string
		ifType      uint32
		friendly    string
		description string
		want        Kind
	}{
		{name: "ethernet", ifType: 6, want: KindEthernet},
		{name: "wifi", ifType: 71, want: KindWiFi},
		{name: "loopback", ifType: 24, want: KindLoopback},
		{name: "native tunnel", ifType: 131, want: KindTunnel},
		{name: "hyper-v before ethernet", ifType: 6, friendly: "vEthernet (Default Switch)", want: KindHyperV},
		{name: "wsl", ifType: 6, description: "Hyper-V Virtual Ethernet Adapter # WSL", want: KindWSL},
		{name: "docker", ifType: 6, friendly: "DockerNAT", want: KindDocker},
		{name: "vpn", ifType: 6, description: "WireGuard Tunnel", want: KindVPN},
		{name: "tap", ifType: 1, friendly: "TAP-Windows Adapter", want: KindTunnel},
		{name: "wifi direct", ifType: 71, description: "Microsoft Wi-Fi Direct Virtual Adapter", want: KindVirtual},
		{name: "ndis filter", ifType: 6, friendly: "Ethernet-QoS Packet Scheduler-0000", want: KindVirtual},
		{name: "wan miniport", ifType: 6, description: "WAN Miniport (IP)", want: KindVirtual},
		{name: "kernel debugger", ifType: 6, description: "Microsoft Kernel Debug Network Adapter", want: KindVirtual},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := classify(test.ifType, test.friendly, test.description); got != test.want {
				t.Fatalf("classify() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestCandidatesAreLimitedToPhysicalMediaTypes(t *testing.T) {
	if !isCandidate(KindEthernet) || !isCandidate(KindWiFi) {
		t.Fatal("ethernet and Wi-Fi must be candidates")
	}
	for _, kind := range []Kind{KindLoopback, KindTunnel, KindHyperV, KindWSL, KindVPN, KindDocker, KindVirtual, KindOther} {
		if isCandidate(kind) {
			t.Fatalf("%q must not be a candidate", kind)
		}
	}
}

package interfaces

import "strings"

// Kind describes how an interface participates in routing decisions.
type Kind string

const (
	KindEthernet Kind = "ethernet"
	KindWiFi     Kind = "wifi"
	KindLoopback Kind = "loopback"
	KindTunnel   Kind = "tunnel"
	KindHyperV   Kind = "hyper-v"
	KindWSL      Kind = "wsl"
	KindVPN      Kind = "vpn"
	KindDocker   Kind = "docker"
	KindVirtual  Kind = "virtual"
	KindOther    Kind = "other"
)

type Address struct {
	IP           string `json:"ip"`
	PrefixLength uint8  `json:"prefix_length"`
}

type Adapter struct {
	GUID         string    `json:"guid"`
	LUID         uint64    `json:"luid"`
	Index        uint32    `json:"index"`
	IPv6Index    uint32    `json:"ipv6_index"`
	MAC          string    `json:"mac,omitempty"`
	FriendlyName string    `json:"friendly_name"`
	Description  string    `json:"description"`
	Status       string    `json:"status"`
	Kind         Kind      `json:"kind"`
	Candidate    bool      `json:"candidate"`
	MTU          uint32    `json:"mtu"`
	IPv4Metric   uint32    `json:"ipv4_metric"`
	IPv6Metric   uint32    `json:"ipv6_metric"`
	Addresses    []Address `json:"addresses"`
	Gateways     []string  `json:"gateways"`
	DNSServers   []string  `json:"dns_servers"`
}

func classify(ifType uint32, friendlyName, description string) Kind {
	text := strings.ToLower(friendlyName + " " + description)
	switch {
	case ifType == 24:
		return KindLoopback
	case strings.Contains(text, "wsl"):
		return KindWSL
	case strings.Contains(text, "docker"):
		return KindDocker
	case strings.Contains(text, "vpn") || strings.Contains(text, "wireguard") || strings.Contains(text, "openvpn"):
		return KindVPN
	case ifType == 131 || strings.Contains(text, "tun") || strings.Contains(text, "tap"):
		return KindTunnel
	case strings.Contains(text, "hyper-v") || strings.Contains(text, "vethernet"):
		return KindHyperV
	case containsAny(text,
		"virtual", "wan miniport", "filter driver", "packet scheduler",
		"lightweight filter", "kernel debug"):
		return KindVirtual
	case ifType == 6:
		return KindEthernet
	case ifType == 71:
		return KindWiFi
	default:
		return KindOther
	}
}

func containsAny(value string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(value, term) {
			return true
		}
	}
	return false
}

func isCandidate(kind Kind) bool {
	return kind == KindEthernet || kind == KindWiFi
}

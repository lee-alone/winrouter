package interfacemanager

import (
	"net/netip"
	"sort"

	"winrouter/internal/interfaces"
)

func BuildCandidates(adapters []interfaces.Adapter) []Candidate {
	result := make([]Candidate, 0)
	for _, adapter := range adapters {
		if !adapter.Candidate || (adapter.Kind != interfaces.KindEthernet && adapter.Kind != interfaces.KindWiFi) {
			continue
		}
		candidate := Candidate{Adapter: adapter, Reasons: make([]string, 0, 4)}
		if adapter.Status == "up" {
			candidate.RecommendationScore += 100
			candidate.Reasons = append(candidate.Reasons, "up")
		}
		if hasUsableIPv4(adapter) {
			candidate.RecommendationScore += 40
			candidate.Reasons = append(candidate.Reasons, "ipv4")
		}
		if hasUsableIPv4Gateway(adapter) {
			candidate.RecommendationScore += 20
			candidate.Reasons = append(candidate.Reasons, "ipv4-gateway")
		}
		if len(adapter.DNSServers) > 0 {
			candidate.RecommendationScore += 5
			candidate.Reasons = append(candidate.Reasons, "dns")
		}
		candidate.Eligible = adapter.Status == "up" && hasUsableIPv4(adapter) && hasUsableIPv4Gateway(adapter)
		result = append(result, candidate)
	}
	sort.SliceStable(result, func(i, j int) bool {
		if result[i].Eligible != result[j].Eligible {
			return result[i].Eligible
		}
		if result[i].RecommendationScore != result[j].RecommendationScore {
			return result[i].RecommendationScore > result[j].RecommendationScore
		}
		if result[i].Adapter.IPv4Metric != result[j].Adapter.IPv4Metric {
			return result[i].Adapter.IPv4Metric < result[j].Adapter.IPv4Metric
		}
		return result[i].Adapter.FriendlyName < result[j].Adapter.FriendlyName
	})
	return result
}

func hasUsableIPv4Gateway(adapter interfaces.Adapter) bool {
	for _, gateway := range adapter.Gateways {
		ip, err := netip.ParseAddr(gateway)
		if err == nil && ip.Is4() && !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

func hasUsableIPv4(adapter interfaces.Adapter) bool {
	for _, address := range adapter.Addresses {
		ip, err := netip.ParseAddr(address.IP)
		if err == nil && ip.Is4() && !ip.IsUnspecified() && !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
			return true
		}
	}
	return false
}

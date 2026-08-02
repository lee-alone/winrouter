package interfaces

import (
	"net/netip"
	"sort"
)

type PrefixAction string

const (
	PrefixBindInterface PrefixAction = "bind-interface"
	PrefixBypassTUN     PrefixAction = "bypass-tun"
	PrefixLocal         PrefixAction = "local"
)

type DirectPrefix struct {
	Prefix       string       `json:"prefix"`
	AdapterGUID  string       `json:"adapter_guid"`
	AdapterName  string       `json:"adapter_name"`
	AdapterKind  Kind         `json:"adapter_kind"`
	Action       PrefixAction `json:"action"`
	RouteMetric  uint32       `json:"route_metric"`
	AdapterState string       `json:"adapter_state"`
}

type OverlapKind string

const (
	OverlapEqual    OverlapKind = "equal"
	OverlapContains OverlapKind = "contains"
)

type PrefixOverlap struct {
	First             DirectPrefix `json:"first"`
	Second            DirectPrefix `json:"second"`
	Kind              OverlapKind  `json:"kind"`
	RequiresSelection bool         `json:"requires_selection"`
}

type Topology struct {
	Prefixes []DirectPrefix  `json:"prefixes"`
	Overlaps []PrefixOverlap `json:"overlaps"`
}

// BuildTopology derives direct routes from current unicast addresses. It does
// not mutate Windows routes and returns deterministic output for diagnostics.
func BuildTopology(adapters []Adapter) Topology {
	prefixes := make([]DirectPrefix, 0)
	for _, adapter := range adapters {
		seen := make(map[string]struct{})
		for _, address := range adapter.Addresses {
			ip, err := netip.ParseAddr(address.IP)
			if err != nil || ip.IsUnspecified() || ip.IsMulticast() {
				continue
			}
			bits := 128
			if ip.Is4() {
				bits = 32
			}
			if int(address.PrefixLength) > bits {
				continue
			}
			prefix := netip.PrefixFrom(ip, int(address.PrefixLength)).Masked().String()
			if _, exists := seen[prefix]; exists {
				continue
			}
			seen[prefix] = struct{}{}
			prefixes = append(prefixes, DirectPrefix{
				Prefix:       prefix,
				AdapterGUID:  normalizeGUID(adapter.GUID),
				AdapterName:  adapter.FriendlyName,
				AdapterKind:  adapter.Kind,
				Action:       prefixAction(adapter.Kind),
				RouteMetric:  metricFor(ip, adapter),
				AdapterState: adapter.Status,
			})
		}
	}
	sort.Slice(prefixes, func(i, j int) bool { return prefixLess(prefixes[i], prefixes[j]) })
	return Topology{Prefixes: prefixes, Overlaps: findOverlaps(prefixes)}
}

func prefixAction(kind Kind) PrefixAction {
	switch kind {
	case KindEthernet, KindWiFi:
		return PrefixBindInterface
	case KindLoopback:
		return PrefixLocal
	default:
		return PrefixBypassTUN
	}
}

func metricFor(ip netip.Addr, adapter Adapter) uint32 {
	if ip.Is4() {
		return adapter.IPv4Metric
	}
	return adapter.IPv6Metric
}

func prefixLess(first, second DirectPrefix) bool {
	a := netip.MustParsePrefix(first.Prefix)
	b := netip.MustParsePrefix(second.Prefix)
	if a.Addr().Compare(b.Addr()) != 0 {
		return a.Addr().Compare(b.Addr()) < 0
	}
	if a.Bits() != b.Bits() {
		return a.Bits() < b.Bits()
	}
	return first.AdapterGUID < second.AdapterGUID
}

func findOverlaps(prefixes []DirectPrefix) []PrefixOverlap {
	result := make([]PrefixOverlap, 0)
	for i := 0; i < len(prefixes); i++ {
		first := netip.MustParsePrefix(prefixes[i].Prefix)
		for j := i + 1; j < len(prefixes); j++ {
			if prefixes[i].AdapterGUID == prefixes[j].AdapterGUID {
				continue
			}
			second := netip.MustParsePrefix(prefixes[j].Prefix)
			if first.Addr().BitLen() != second.Addr().BitLen() || (!first.Contains(second.Addr()) && !second.Contains(first.Addr())) {
				continue
			}
			kind := OverlapContains
			if first == second {
				kind = OverlapEqual
			}
			result = append(result, PrefixOverlap{
				First:             prefixes[i],
				Second:            prefixes[j],
				Kind:              kind,
				RequiresSelection: overlapRequiresSelection(first, second, prefixes[i], prefixes[j]),
			})
		}
	}
	return result
}

func overlapRequiresSelection(first, second netip.Prefix, a, b DirectPrefix) bool {
	if a.Action == PrefixLocal || b.Action == PrefixLocal {
		return false
	}
	// Link-local addresses are interface scoped and normally overlap.
	if first.Addr().IsLinkLocalUnicast() && second.Addr().IsLinkLocalUnicast() {
		return false
	}
	return true
}

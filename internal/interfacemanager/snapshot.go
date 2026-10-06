package interfacemanager

import (
	"errors"
	"fmt"
	"net/netip"
	"strings"

	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
	"winrouter/internal/tunprefix"
)

func buildSnapshot(sequence uint64, state State, adapters []interfaces.Adapter, routeTable []routes.Route, pool []string) Snapshot {
	mode := state.Mode
	if mode == "" {
		if state.InterfaceA.GUID != "" && state.InterfaceB.GUID == "" {
			mode = ModeSingle
		} else {
			mode = ModeDual
		}
	}
	topology := interfaces.BuildTopology(adapters)
	snapshot := Snapshot{
		Sequence:      sequence,
		Mode:          mode,
		Adapters:      adapters,
		Routes:        routeTable,
		Candidates:    BuildCandidates(adapters),
		Topology:      topology,
		IPv6TUNPrefix: state.IPv6TUNPrefix,
		IPv6Policy:    state.IPv6Policy,
		Diagnostics:   make([]Diagnostic, 0),
	}
	snapshot.InterfaceA = resolveSelection("A", state.InterfaceA, adapters)
	if mode == ModeDual {
		snapshot.InterfaceB = resolveSelection("B", state.InterfaceB, adapters)
		if snapshot.InterfaceA.Match != nil && snapshot.InterfaceB.Match != nil && equalGUID(snapshot.InterfaceA.Match.Adapter.GUID, snapshot.InterfaceB.Match.Adapter.GUID) {
			snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "same-interface", Severity: "error", Message: ErrSameAdapter.Error()})
		}
	} else {
		snapshot.InterfaceB = ResolvedSelection{Role: "B", Status: "disabled"}
	}
	checkedSelections := []ResolvedSelection{snapshot.InterfaceA}
	if mode == ModeDual {
		checkedSelections = append(checkedSelections, snapshot.InterfaceB)
	}
	for _, selection := range checkedSelections {
		if selection.Status == "resolved" && selection.Match != nil && !hasIPv4DefaultRoute(routeTable, selection.Match.Adapter) {
			snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "missing-default-route-" + strings.ToLower(selection.Role), Severity: "error", Message: fmt.Sprintf("selected interface %s (%s) has no IPv4 default route", selection.Role, selection.Match.Adapter.FriendlyName)})
		}
	}
	if mode == ModeDual {
		var guidA, guidB string
		if snapshot.InterfaceA.Match != nil {
			guidA = strings.ToLower(strings.Trim(snapshot.InterfaceA.Match.Adapter.GUID, "{}"))
		}
		if snapshot.InterfaceB.Match != nil {
			guidB = strings.ToLower(strings.Trim(snapshot.InterfaceB.Match.Adapter.GUID, "{}"))
		}
		for _, overlap := range topology.Overlaps {
			if overlap.RequiresSelection && overlapBlocksPolicy(overlap, state.IPv6Policy) {
				firstGuid := strings.ToLower(strings.Trim(overlap.First.AdapterGUID, "{}"))
				secondGuid := strings.ToLower(strings.Trim(overlap.Second.AdapterGUID, "{}"))
				if guidA != "" && guidB != "" {
					if (firstGuid == guidA && secondGuid == guidB) || (firstGuid == guidB && secondGuid == guidA) {
						snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "prefix-overlap", Severity: "error", Message: fmt.Sprintf("%s on %s overlaps %s on %s", overlap.First.Prefix, overlap.First.AdapterName, overlap.Second.Prefix, overlap.Second.AdapterName)})
					}
				} else if guidA != "" || guidB != "" {
					selected := guidA
					if selected == "" {
						selected = guidB
					}
					if firstGuid == selected || secondGuid == selected {
						snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "prefix-overlap", Severity: "error", Message: fmt.Sprintf("%s on %s overlaps %s on %s", overlap.First.Prefix, overlap.First.AdapterName, overlap.Second.Prefix, overlap.Second.AdapterName)})
					}
				} else {
					snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "prefix-overlap", Severity: "error", Message: fmt.Sprintf("%s on %s overlaps %s on %s", overlap.First.Prefix, overlap.First.AdapterName, overlap.Second.Prefix, overlap.Second.AdapterName)})
				}
			}
		}
	}
	if state.IPv6Policy == IPv6PolicySplit {
		for _, selection := range checkedSelections {
			if selection.Status != "resolved" || selection.Match == nil {
				continue
			}
			if !hasUsableIPv6(selection.Match.Adapter) {
				snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "missing-ipv6-address-" + strings.ToLower(selection.Role), Severity: "error", Message: fmt.Sprintf("selected interface %s (%s) has no usable IPv6 address", selection.Role, selection.Match.Adapter.FriendlyName)})
			}
			if !hasIPv6DefaultRoute(routeTable, selection.Match.Adapter) {
				snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: "missing-ipv6-default-route-" + strings.ToLower(selection.Role), Severity: "error", Message: fmt.Sprintf("selected interface %s (%s) has no IPv6 default route", selection.Role, selection.Match.Adapter.FriendlyName)})
			}
		}
	}
	allocation, err := tunprefix.Allocate(pool, state.TUNPrefix, adapters, routeTable)
	if err == nil {
		snapshot.TUN = &allocation
	} else {
		severity := "error"
		code := "tun-prefix"
		if errors.Is(err, tunprefix.ErrPoolExhausted) {
			code = "tun-pool-exhausted"
		}
		snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{Code: code, Severity: severity, Message: err.Error()})
	}
	v6PrefixStr := state.IPv6TUNPrefix
	if v6PrefixStr == "" {
		v6PrefixStr = "fdfe:dcba:9876::/126"
	}
	if v6Candidate, parseErr := netip.ParsePrefix(v6PrefixStr); parseErr == nil {
		v6Conflicts := tunprefix.ConflictsForPrefix(v6Candidate, adapters, routeTable)
		if len(v6Conflicts) > 0 {
			snapshot.Diagnostics = append(snapshot.Diagnostics, Diagnostic{
				Code:     "tun-ipv6-conflict",
				Severity: "error",
				Message:  fmt.Sprintf("IPv6 TUN prefix %s conflicts with existing network (%d conflict(s))", v6PrefixStr, len(v6Conflicts)),
			})
		}
	}
	return snapshot
}

func overlapBlocksPolicy(overlap interfaces.PrefixOverlap, policy string) bool {
	if policy == IPv6PolicySplit {
		return true
	}
	first, firstErr := netip.ParsePrefix(overlap.First.Prefix)
	second, secondErr := netip.ParsePrefix(overlap.Second.Prefix)
	return firstErr == nil && secondErr == nil && first.Addr().Is4() && second.Addr().Is4()
}

func hasIPv4DefaultRoute(routeTable []routes.Route, adapter interfaces.Adapter) bool {
	for _, route := range routeTable {
		if route.Prefix == "0.0.0.0/0" && (route.InterfaceIndex == adapter.Index || (route.InterfaceLUID != 0 && adapter.LUID != 0 && route.InterfaceLUID == adapter.LUID)) {
			return true
		}
	}
	return false
}

func hasIPv6DefaultRoute(routeTable []routes.Route, adapter interfaces.Adapter) bool {
	for _, route := range routeTable {
		if route.Prefix == "::/0" && routeBelongsToIPv6Adapter(route, adapter) {
			return true
		}
	}
	return false
}

func routeBelongsToIPv6Adapter(route routes.Route, adapter interfaces.Adapter) bool {
	if route.InterfaceLUID != 0 && adapter.LUID != 0 {
		return route.InterfaceLUID == adapter.LUID
	}
	return (adapter.IPv6Index != 0 && route.InterfaceIndex == adapter.IPv6Index) || (adapter.Index != 0 && route.InterfaceIndex == adapter.Index)
}

func hasUsableIPv6(adapter interfaces.Adapter) bool {
	for _, address := range adapter.Addresses {
		parsed, err := netip.ParseAddr(address.IP)
		if err == nil && parsed.Is6() && !parsed.IsUnspecified() && !parsed.IsLoopback() && !parsed.IsLinkLocalUnicast() && !parsed.IsMulticast() {
			return true
		}
	}
	return false
}

func resolveSelection(role string, identity interfaces.Identity, adapters []interfaces.Adapter) ResolvedSelection {
	result := ResolvedSelection{Role: role, Saved: identity, Status: "unselected"}
	if identity.GUID == "" {
		return result
	}
	match, err := interfaces.Resolve(identity, adapters)
	if err != nil {
		result.Status = "selection-required"
		result.Error = err.Error()
		return result
	}
	result.Match = &match
	if match.Adapter.Status != "up" {
		result.Status = "unavailable"
		result.Error = "selected interface is not up"
	} else {
		result.Status = "resolved"
	}
	return result
}

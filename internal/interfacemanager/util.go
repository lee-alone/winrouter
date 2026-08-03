package interfacemanager

import (
	"crypto/sha256"
	"encoding/json"
	"sort"
	"strings"

	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
)

func equalGUID(first, second string) bool {
	normalize := func(value string) string { return strings.ToLower(strings.Trim(strings.TrimSpace(value), "{}")) }
	return normalize(first) != "" && normalize(first) == normalize(second)
}

func fingerprint(snapshot Snapshot) [sha256.Size]byte {
	value := struct {
		Adapters    []fingerprintAdapter
		Routes      []routes.Route
		InterfaceA  fingerprintSelection
		InterfaceB  fingerprintSelection
		TUNPrefix   string
		Diagnostics []Diagnostic
	}{
		Adapters:    relevantAdapters(snapshot),
		InterfaceA:  selectionFingerprint(snapshot.InterfaceA),
		InterfaceB:  selectionFingerprint(snapshot.InterfaceB),
		Diagnostics: append([]Diagnostic(nil), snapshot.Diagnostics...),
	}
	selected := selectedInterfaceIndexes(snapshot)
	for _, route := range snapshot.Routes {
		if isDefaultRoute(route) && selected[route.InterfaceIndex] {
			value.Routes = append(value.Routes, route)
		}
	}
	sort.Slice(value.Routes, func(i, j int) bool { return routeLess(value.Routes[i], value.Routes[j]) })
	sort.Slice(value.Diagnostics, func(i, j int) bool {
		if value.Diagnostics[i].Code != value.Diagnostics[j].Code {
			return value.Diagnostics[i].Code < value.Diagnostics[j].Code
		}
		if value.Diagnostics[i].Severity != value.Diagnostics[j].Severity {
			return value.Diagnostics[i].Severity < value.Diagnostics[j].Severity
		}
		return value.Diagnostics[i].Message < value.Diagnostics[j].Message
	})
	if snapshot.TUN != nil {
		value.TUNPrefix = snapshot.TUN.Prefix
	}
	data, _ := json.Marshal(value)
	return sha256.Sum256(data)
}

type fingerprintAdapter struct {
	GUID         string
	LUID         uint64
	Index        uint32
	FriendlyName string
	Status       string
	Kind         interfaces.Kind
	Candidate    bool
	IPv4Metric   uint32
	IPv6Metric   uint32
	Addresses    []interfaces.Address
	Gateways     []string
	DNSServers   []string
}

type fingerprintSelection struct {
	Role      string
	SavedGUID string
	MatchGUID string
	Status    string
	Error     string
}

func relevantAdapters(snapshot Snapshot) []fingerprintAdapter {
	selected := selectedInterfaceGUIDs(snapshot)
	result := make([]fingerprintAdapter, 0)
	for _, adapter := range snapshot.Adapters {
		if !adapter.Candidate && !selected[adapter.GUID] {
			continue
		}
		result = append(result, fingerprintAdapter{
			GUID: adapter.GUID, LUID: adapter.LUID, Index: adapter.Index, FriendlyName: adapter.FriendlyName,
			Status: adapter.Status, Kind: adapter.Kind, Candidate: adapter.Candidate,
			IPv4Metric: adapter.IPv4Metric, IPv6Metric: adapter.IPv6Metric,
			Addresses: append([]interfaces.Address(nil), adapter.Addresses...),
			Gateways:  append([]string(nil), adapter.Gateways...), DNSServers: append([]string(nil), adapter.DNSServers...),
		})
	}
	for i := range result {
		sort.Slice(result[i].Addresses, func(a, b int) bool {
			if result[i].Addresses[a].IP != result[i].Addresses[b].IP {
				return result[i].Addresses[a].IP < result[i].Addresses[b].IP
			}
			return result[i].Addresses[a].PrefixLength < result[i].Addresses[b].PrefixLength
		})
		sort.Strings(result[i].Gateways)
		sort.Strings(result[i].DNSServers)
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].GUID != result[j].GUID {
			return result[i].GUID < result[j].GUID
		}
		return result[i].Index < result[j].Index
	})
	return result
}

func selectionFingerprint(selection ResolvedSelection) fingerprintSelection {
	result := fingerprintSelection{Role: selection.Role, SavedGUID: selection.Saved.GUID, Status: selection.Status, Error: selection.Error}
	if selection.Match != nil {
		result.MatchGUID = selection.Match.Adapter.GUID
	}
	return result
}

func selectedInterfaceGUIDs(snapshot Snapshot) map[string]bool {
	result := make(map[string]bool, 2)
	for _, selection := range []ResolvedSelection{snapshot.InterfaceA, snapshot.InterfaceB} {
		if selection.Match != nil {
			result[selection.Match.Adapter.GUID] = true
		}
	}
	return result
}

func selectedInterfaceIndexes(snapshot Snapshot) map[uint32]bool {
	result := make(map[uint32]bool, 2)
	for _, selection := range []ResolvedSelection{snapshot.InterfaceA, snapshot.InterfaceB} {
		if selection.Match != nil {
			result[selection.Match.Adapter.Index] = true
		}
	}
	return result
}

func isDefaultRoute(route routes.Route) bool {
	return route.Prefix == "0.0.0.0/0" || route.Prefix == "::/0"
}

func routeLess(first, second routes.Route) bool {
	if first.Prefix != second.Prefix {
		return first.Prefix < second.Prefix
	}
	if first.InterfaceIndex != second.InterfaceIndex {
		return first.InterfaceIndex < second.InterfaceIndex
	}
	if first.InterfaceLUID != second.InterfaceLUID {
		return first.InterfaceLUID < second.InterfaceLUID
	}
	if first.Metric != second.Metric {
		return first.Metric < second.Metric
	}
	return first.Protocol < second.Protocol
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return Snapshot{}
	}
	var clone Snapshot
	if err := json.Unmarshal(data, &clone); err != nil {
		return Snapshot{}
	}
	return clone
}

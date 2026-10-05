package recovery

import (
	"fmt"
	"net/netip"
	"strings"

	"winrouter/internal/config"
	"winrouter/internal/interfacemanager"
	"winrouter/internal/interfaces"
)

func RebuildConfig(base config.MVPConfig, snapshot interfacemanager.Snapshot) (config.MVPConfig, error) {
	if !Usable(snapshot) || snapshot.InterfaceA.Match == nil {
		return config.MVPConfig{}, errorsForSnapshot(snapshot)
	}
	if snapshot.Mode != interfacemanager.ModeSingle && snapshot.InterfaceB.Match == nil {
		return config.MVPConfig{}, errorsForSnapshot(snapshot)
	}
	result := base
	result.Mode = snapshot.Mode
	result.TUN.Prefix = snapshot.TUN.Prefix
	result.InterfaceA = config.MVPInterface{GUID: snapshot.InterfaceA.Match.Adapter.GUID, BindInterface: snapshot.InterfaceA.Match.Adapter.FriendlyName}
	if snapshot.Mode == interfacemanager.ModeSingle {
		result.InterfaceB = config.MVPInterface{}
	} else {
		result.InterfaceB = config.MVPInterface{GUID: snapshot.InterfaceB.Match.Adapter.GUID, BindInterface: snapshot.InterfaceB.Match.Adapter.FriendlyName}
	}
	result.DirectPrefixes = make([]config.MVPDirectPrefix, 0)
	for _, prefix := range snapshot.Topology.Prefixes {
		parsed, err := netip.ParsePrefix(prefix.Prefix)
		if err != nil || !parsed.Addr().Is4() || prefix.Action != interfaces.PrefixBindInterface {
			continue
		}
		if sameGUID(prefix.AdapterGUID, result.InterfaceA.GUID) || (snapshot.Mode != interfacemanager.ModeSingle && sameGUID(prefix.AdapterGUID, result.InterfaceB.GUID)) {
			result.DirectPrefixes = append(result.DirectPrefixes, config.MVPDirectPrefix{Prefix: prefix.Prefix, BindInterface: prefix.AdapterName})
		}
	}
	if _, err := config.GenerateMVP(result); err != nil {
		return config.MVPConfig{}, fmt.Errorf("rebuilt configuration: %w", err)
	}
	return result, nil
}

func errorsForSnapshot(snapshot interfacemanager.Snapshot) error {
	if snapshot.Mode == interfacemanager.ModeSingle {
		if snapshot.InterfaceA.Status != "resolved" {
			return fmt.Errorf("selected interface A is unavailable: %s", snapshot.InterfaceA.Status)
		}
	} else {
		if snapshot.InterfaceA.Status != "resolved" || snapshot.InterfaceB.Status != "resolved" {
			return fmt.Errorf("selected interfaces are unavailable: A=%s B=%s", snapshot.InterfaceA.Status, snapshot.InterfaceB.Status)
		}
	}
	if snapshot.TUN == nil {
		return fmt.Errorf("TUN prefix is unavailable")
	}
	return fmt.Errorf("interface snapshot contains blocking diagnostics")
}

func sameGUID(first, second string) bool {
	normalize := func(value string) string { return strings.ToLower(strings.Trim(strings.TrimSpace(value), "{}")) }
	return normalize(first) != "" && normalize(first) == normalize(second)
}

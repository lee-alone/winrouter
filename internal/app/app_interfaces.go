package app

import (
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"

	"winrouter/internal/core"
	"winrouter/internal/interfacemanager"
	"winrouter/internal/interfaces"
	"winrouter/internal/nodes"
	"winrouter/internal/observability"
)

func (a *App) interfaceEgressDetails(egress string) (sourceIP string, bindInterface string, err error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return "", "", err
	}
	snapshot := manager.Snapshot()
	var iface interfacemanager.ResolvedSelection
	if egress == nodes.EgressA || snapshot.Mode == interfacemanager.ModeSingle || snapshot.InterfaceB.Match == nil {
		iface = snapshot.InterfaceA
	} else {
		iface = snapshot.InterfaceB
	}
	if iface.Match == nil {
		return "", "", fmt.Errorf("interface %s is not resolved", strings.ToUpper(egress))
	}
	// sing-box bind_interface expects the Windows interface name (for example
	// "Ethernet"), while the GUID is only used by WinRouter for stable
	// interface identity and selection persistence.
	bindInterface = iface.Match.Adapter.FriendlyName
	for _, address := range iface.Match.Adapter.Addresses {
		parsed := net.ParseIP(address.IP)
		if parsed != nil && parsed.To4() != nil && !parsed.IsLoopback() && !parsed.IsUnspecified() {
			sourceIP = parsed.String()
			break
		}
	}
	return sourceIP, bindInterface, nil
}

func (a *App) interfaceEgressDetailsIPv6(egress string) (sourceIP string, bindInterface string, err error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return "", "", err
	}
	snapshot := manager.Snapshot()
	var iface interfacemanager.ResolvedSelection
	if egress == nodes.EgressA || snapshot.Mode == interfacemanager.ModeSingle || snapshot.InterfaceB.Match == nil {
		iface = snapshot.InterfaceA
	} else {
		iface = snapshot.InterfaceB
	}
	if iface.Match == nil {
		return "", "", fmt.Errorf("interface %s is not resolved", strings.ToUpper(egress))
	}
	bindInterface = iface.Match.Adapter.FriendlyName
	for _, address := range iface.Match.Adapter.Addresses {
		parsed, parseErr := netip.ParseAddr(address.IP)
		if parseErr == nil && parsed.Is6() && !parsed.IsUnspecified() && !parsed.IsLoopback() && !parsed.IsLinkLocalUnicast() && !parsed.IsMulticast() {
			sourceIP = parsed.String()
			break
		}
	}
	return sourceIP, bindInterface, nil
}

func (a *App) interfaceASourceIPv4() (string, error) {
	source, _, err := a.interfaceEgressDetails(nodes.EgressA)
	if err != nil {
		return "", err
	}
	if source == "" {
		return "", errors.New("interface A has no usable IPv4 source address")
	}
	return source, nil
}

func (a *App) interfaceBSourceIPv4() (string, error) {
	source, _, err := a.interfaceEgressDetails(nodes.EgressB)
	if err != nil {
		return "", err
	}
	if source == "" {
		return "", errors.New("interface B has no usable IPv4 source address")
	}
	return source, nil
}

func (a *App) interfaceASourceIPv6() (string, error) {
	source, _, err := a.interfaceEgressDetailsIPv6(nodes.EgressA)
	if err != nil {
		return "", err
	}
	if source == "" {
		return "", errors.New("interface A has no usable IPv6 source address")
	}
	return source, nil
}

func (a *App) interfaceBSourceIPv6() (string, error) {
	source, _, err := a.interfaceEgressDetailsIPv6(nodes.EgressB)
	if err != nil {
		return "", err
	}
	if source == "" {
		return "", errors.New("interface B has no usable IPv6 source address")
	}
	return source, nil
}

func (a *App) GetInterfaceSnapshot() (interfacemanager.Snapshot, error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	snapshot := manager.Snapshot()
	if snapshot.Sequence == 0 {
		return manager.Refresh()
	}
	return snapshot, nil
}

func (a *App) SelectSingleInterface(interfaceAGUID string) (interfacemanager.Snapshot, error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	snapshot := manager.Snapshot()
	if snapshot.Sequence == 0 {
		snapshot, err = manager.Refresh()
		if err != nil {
			return interfacemanager.Snapshot{}, err
		}
	}
	interfaceA, foundA := findAdapter(snapshot.Adapters, interfaceAGUID)
	if !foundA {
		return interfacemanager.Snapshot{}, interfaces.ErrAdapterNotFound
	}
	return manager.SelectSingle(interfaceA)
}

func (a *App) SetRoutingMode(mode string) (interfacemanager.Snapshot, error) {
	status, err := a.GetCoreStatus()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	if status.State == core.StateRunning {
		return interfacemanager.Snapshot{}, errors.New("stop routing before changing the routing mode")
	}
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	snapshot, err := manager.SetMode(mode)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "interfaces", "Routing mode updated", "", map[string]any{"mode": mode})
	}
	return snapshot, err
}

func (a *App) SelectInterfaces(interfaceAGUID, interfaceBGUID string) (interfacemanager.Snapshot, error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	if manager.Mode() == interfacemanager.ModeSingle || strings.TrimSpace(interfaceBGUID) == "" || strings.EqualFold(strings.TrimSpace(interfaceBGUID), "none") {
		return a.SelectSingleInterface(interfaceAGUID)
	}
	snapshot := manager.Snapshot()
	if snapshot.Sequence == 0 {
		snapshot, err = manager.Refresh()
		if err != nil {
			return interfacemanager.Snapshot{}, err
		}
	}
	interfaceA, foundA := findAdapter(snapshot.Adapters, interfaceAGUID)
	interfaceB, foundB := findAdapter(snapshot.Adapters, interfaceBGUID)
	if !foundA || !foundB {
		return interfacemanager.Snapshot{}, interfaces.ErrAdapterNotFound
	}
	return manager.Select(interfaceA, interfaceB)
}

func (a *App) GetIPv6Policy() (string, error) {
	manager, err := a.getInterfaceManager()
	if err != nil {
		return "", err
	}
	return manager.IPv6Policy(), nil
}

func (a *App) SetIPv6Policy(policy string) (interfacemanager.Snapshot, error) {
	status, err := a.GetCoreStatus()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	if status.State == "running" {
		return interfacemanager.Snapshot{}, errors.New("stop routing before changing the IPv6 policy")
	}
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	snapshot, err := manager.SetIPv6Policy(policy)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "interfaces", "IPv6 policy updated", "", map[string]any{"policy": policy})
	}
	return snapshot, err
}

func (a *App) ResetInterfaceSelection() (interfacemanager.Snapshot, error) {
	if _, err := a.StopCore(); err != nil {
		return interfacemanager.Snapshot{}, fmt.Errorf("stop routing core: %w", err)
	}
	manager, err := a.getInterfaceManager()
	if err != nil {
		return interfacemanager.Snapshot{}, err
	}
	snapshot, err := manager.ClearSelection()
	if err == nil {
		a.observations.Log(observability.LevelInfo, "application-reset", "Saved interface selection reset", "", nil)
	}
	return snapshot, err
}

func (a *App) getInterfaceManager() (*interfacemanager.Manager, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.interfaceError != nil {
		return nil, a.interfaceError
	}
	if a.interfaceManager == nil {
		return nil, errors.New("interface manager is not started")
	}
	return a.interfaceManager, nil
}

func (a *App) setInterfaceError(err error) {
	a.mu.Lock()
	a.interfaceError = err
	a.mu.Unlock()
}

func findAdapter(adapters []interfaces.Adapter, guid string) (interfaces.Adapter, bool) {
	wanted := strings.Trim(strings.TrimSpace(guid), "{}")
	for _, adapter := range adapters {
		if strings.EqualFold(strings.Trim(adapter.GUID, "{}"), wanted) {
			return adapter, true
		}
	}
	return interfaces.Adapter{}, false
}

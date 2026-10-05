package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"time"

	"winrouter/internal/config"
	"winrouter/internal/core"
	"winrouter/internal/nodes"
	"winrouter/internal/observability"
)

func (a *App) ListProxyNodes() ([]nodes.Node, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nil, err
	}
	return store.List(), nil
}

func (a *App) AddProxyNode(input nodes.Input) (nodes.Node, error) {
	if a.isCoreRunning() {
		return nodes.Node{}, errors.New("cannot modify proxy nodes while core is running; please stop the core first")
	}
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.Node{}, err
	}
	result, err := store.Add(input)
	if err != nil {
		return nodes.Node{}, err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy node added", "", map[string]any{"node_id": result.ID, "type": result.Type})
	return result, nil
}

func (a *App) UpdateProxyNode(input nodes.Input) (nodes.Node, error) {
	if a.isCoreRunning() {
		return nodes.Node{}, errors.New("cannot modify proxy nodes while core is running; please stop the core first")
	}
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.Node{}, err
	}
	result, err := store.Update(input)
	if err != nil {
		return nodes.Node{}, err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy node updated", "", map[string]any{"node_id": result.ID, "type": result.Type})
	return result, nil
}

func (a *App) SaveProxyNode(input nodes.Input) (nodes.Node, error) {
	if input.ID == "" {
		return a.AddProxyNode(input)
	}
	return a.UpdateProxyNode(input)
}

func (a *App) SetProxyNodeEgress(id string, egress string) (nodes.Node, error) {
	if a.isCoreRunning() {
		return nodes.Node{}, errors.New("cannot modify proxy egress while core is running; please stop the core first")
	}
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.Node{}, err
	}
	egress = strings.ToLower(strings.TrimSpace(egress))
	if egress != nodes.EgressA && egress != nodes.EgressB {
		return nodes.Node{}, errors.New("proxy node egress must be 'a' or 'b'")
	}
	node, err := store.Get(id)
	if err != nil {
		return nodes.Node{}, err
	}
	isOverridden := false
	if node.SubscriptionID != "" && a.subscriptions != nil {
		if sub, err := a.subscriptions.Get(node.SubscriptionID); err == nil {
			isOverridden = (sub.Egress != egress)
		} else {
			isOverridden = true
		}
	}
	result, err := store.SetNodeEgress(id, egress, isOverridden)
	if err != nil {
		return nodes.Node{}, err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy node egress updated", "", map[string]any{"node_id": result.ID, "egress": result.Egress, "overridden": result.EgressOverridden})
	return result, nil
}

func (a *App) DeleteProxyNode(id string) error {
	if a.isCoreRunning() {
		return errors.New("cannot delete proxy node while core is running; please stop the core first")
	}
	store, err := a.getNodeStore()
	if err != nil {
		return err
	}
	if err := store.Delete(id); err != nil {
		return err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy node deleted", "", map[string]any{"node_id": id})
	return nil
}

func (a *App) SelectProxyNode(id string) (nodes.Node, error) {
	if a.isCoreRunning() {
		return nodes.Node{}, errors.New("cannot switch proxy node while core is running; please stop the core first")
	}
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.Node{}, err
	}
	result, err := store.Select(id)
	if err != nil {
		return nodes.Node{}, err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy node selected", "", map[string]any{"node_id": result.ID, "type": result.Type})
	return result, nil
}

func (a *App) GetProxySelection() (nodes.ProxySelection, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.ProxySelection{}, err
	}
	return store.GetSelection(), nil
}

func (a *App) SetProxyMode(mode string) error {
	if a.isCoreRunning() {
		return errors.New("cannot switch proxy mode while core is running; please stop the core first")
	}
	store, err := a.getNodeStore()
	if err != nil {
		return err
	}
	if err := store.SetMode(mode); err != nil {
		return err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy mode updated", "", map[string]any{"mode": mode})
	return nil
}

func (a *App) SelectProxyChain(ids []string) error {
	if a.isCoreRunning() {
		return errors.New("cannot switch proxy chain while core is running; please stop the core first")
	}
	store, err := a.getNodeStore()
	if err != nil {
		return err
	}
	if err := store.SelectChain(ids); err != nil {
		return err
	}
	a.observations.Log(observability.LevelInfo, "proxy", "Proxy chain selected", "", map[string]any{"chain_length": len(ids)})
	return nil
}

func (a *App) SetProxyNodeFavorite(id string, favorite bool) (nodes.Node, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.Node{}, err
	}
	result, err := store.SetFavorite(id, favorite)
	if err == nil {
		a.observations.Log(observability.LevelInfo, "proxy", "Proxy node favorite updated", "", map[string]any{"node_id": id, "favorite": favorite})
	}
	return result, err
}

func (a *App) getNodeStore() (*nodes.Store, error) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	if a.nodeError != nil {
		return nil, a.nodeError
	}
	if a.nodeStore == nil {
		return nil, errors.New("proxy node store is not started")
	}
	return a.nodeStore, nil
}

func requiresProxy(input config.MVPConfig) bool {
	if input.DefaultOutbound == "c" {
		return true
	}
	for _, rule := range input.CustomRules {
		if rule.Action == "c" {
			return true
		}
	}
	for _, ruleSet := range input.RuleSets {
		if ruleSet.Action == "c" {
			return true
		}
	}
	return false
}

func (a *App) ValidateSelectedProxyConfiguration(input config.MVPConfig) error {
	return a.ValidateCoreConfiguration(input)
}

func (a *App) ApplySelectedProxyConfiguration(input config.MVPConfig) (core.Status, error) {
	return a.ApplyCoreConfiguration(input)
}

func buildMVPProxyFromNode(node nodes.Node, credentials nodes.Credentials, server string) *config.MVPProxy {
	egress := node.Egress
	if egress == "" {
		egress = nodes.EgressB
	}
	if server == "" {
		server = node.Server
	}
	var proxyTLS *config.MVPProxyTLS
	if node.TLS != nil && node.TLS.Enabled {
		proxyTLS = &config.MVPProxyTLS{
			Enabled:    true,
			ServerName: node.TLS.ServerName,
			Insecure:   node.TLS.Insecure,
			ALPN:       append([]string(nil), node.TLS.ALPN...),
		}
	}
	var proxyTransport *config.MVPProxyTransport
	if node.Transport != nil && node.Transport.Type != "" {
		proxyTransport = &config.MVPProxyTransport{
			Type: node.Transport.Type,
			Path: node.Transport.Path,
			Host: node.Transport.Host,
		}
	}
	p := &config.MVPProxy{
		Type:      node.Type,
		Server:    server,
		Port:      node.Port,
		Egress:    egress,
		Password:  credentials.Password,
		UUID:      credentials.UUID,
		Flow:      credentials.Flow,
		TLS:       proxyTLS,
		Transport: proxyTransport,
	}
	if node.Type == nodes.TypeShadowsocks {
		method := credentials.Method
		if method == "" {
			method = credentials.Username
		}
		p.Method = method
	} else {
		p.Username = credentials.Username
	}
	return p
}

func (a *App) resolveProxyServer(server string, egress string, dnsConfig config.MVPDNS) (resolved string, actual string, err error) {
	if net.ParseIP(server) != nil {
		return "", server, nil
	}
	dnsServer := dnsConfig.Global
	if egress == nodes.EgressA {
		dnsServer = dnsConfig.Domestic
	}
	var res string
	var resolveErr error
	if dnsServer.Type == "udp" && dnsServer.Port == 53 {
		dnsAddr, err := netip.ParseAddr(dnsServer.Server)
		if err == nil {
			var source string
			if dnsAddr.Is6() {
				if egress == nodes.EgressA {
					source, _ = a.interfaceASourceIPv6()
				} else {
					source, _ = a.interfaceBSourceIPv6()
				}
			} else {
				if egress == nodes.EgressA {
					source, _ = a.interfaceASourceIPv4()
				} else {
					source, _ = a.interfaceBSourceIPv4()
				}
			}
			dnsCtx, dnsCancel := context.WithTimeout(a.ctx, 8*time.Second)
			res, resolveErr = nodes.ResolveEndpoint(dnsCtx, server, dnsServer.Server, source)
			dnsCancel()
		}
	}
	if res == "" {
		sysCtx, sysCancel := context.WithTimeout(a.ctx, 8*time.Second)
		ips, sysErr := net.DefaultResolver.LookupIP(sysCtx, "ip", server)
		sysCancel()
		if sysErr == nil && len(ips) > 0 {
			res = ips[0].String()
			resolveErr = nil
		}
	}
	if res == "" && resolveErr != nil {
		return "", "", fmt.Errorf("proxy DNS via interface %s: %w", strings.ToUpper(egress), resolveErr)
	} else if res == "" {
		return "", "", fmt.Errorf("failed to resolve proxy domain %q", server)
	}
	return res, res, nil
}

func (a *App) withSelectedProxy(input config.MVPConfig) (config.MVPConfig, error) {
	store, err := a.getNodeStore()
	if err != nil {
		if requiresProxy(input) {
			return config.MVPConfig{}, err
		}
		return input, nil
	}
	selection := store.GetSelection()
	if selection.Mode == nodes.ProxyModeChain && len(selection.SelectedChain) >= 2 {
		chainNodes, chainCreds, err := store.SelectedChainCredentials()
		if err != nil {
			if requiresProxy(input) {
				return config.MVPConfig{}, err
			}
			return input, nil
		}

		hop0 := chainNodes[0]
		egress0 := hop0.Egress
		if egress0 == "" {
			egress0 = nodes.EgressB
		}
		resolved0, server0, err := a.resolveProxyServer(hop0.Server, egress0, input.DNS)
		if err != nil {
			return config.MVPConfig{}, err
		}
		if resolved0 != "" {
			if _, commitErr := store.CommitResolvedIP(hop0.ID, resolved0); commitErr != nil {
				return config.MVPConfig{}, commitErr
			}
			chainNodes[0].ResolvedIP = resolved0
		} else {
			chainNodes[0].ResolvedIP = server0
		}

		source, bindInterface, ifaceErr := a.interfaceEgressDetails(egress0)
		if ifaceErr != nil {
			return config.MVPConfig{}, ifaceErr
		}
		if parsedServer := net.ParseIP(server0); parsedServer != nil && parsedServer.To4() == nil {
			if v6Source, _, v6Err := a.interfaceEgressDetailsIPv6(egress0); v6Err == nil && v6Source != "" {
				source = v6Source
			}
		}

		corePath, coreErr := locateBundledCore()
		if coreErr == nil && corePath != "" {
			probeCtx, probeCancel := context.WithTimeout(a.ctx, 8*time.Second)
			probe := nodes.ProbeChainWithOptions(probeCtx, chainNodes, chainCreds, nodes.ProbeOptions{
				CorePath:      corePath,
				BindInterface: bindInterface,
				SourceIP:      source,
			})
			probeCancel()
			if !probe.Available {
				a.observations.Log(observability.LevelWarning, "proxy", "Selected proxy chain failed health check, proceeding with warning", "", map[string]any{"error": probe.Error, "category": probe.ErrorCategory, "hop0_server": server0})
			}
		}

		input.ProxyChain = make([]config.MVPProxy, len(chainNodes))
		for i := range chainNodes {
			srv := chainNodes[i].Server
			if i == 0 {
				srv = server0
			}
			input.ProxyChain[i] = *buildMVPProxyFromNode(chainNodes[i], chainCreds[i], srv)
		}
		input.Proxy = &input.ProxyChain[len(input.ProxyChain)-1]
		return input, nil
	}

	node, credentials, err := store.SelectedCredentials()
	if err != nil {
		if requiresProxy(input) {
			return config.MVPConfig{}, errors.New("selected proxy configuration is required when default outbound or rules target proxy (c)")
		}
		return input, nil
	}
	egress := node.Egress
	if egress == "" {
		egress = nodes.EgressB
	}
	resolved, server, err := a.resolveProxyServer(node.Server, egress, input.DNS)
	if err != nil {
		return config.MVPConfig{}, err
	}

	source, bindInterface, ifaceErr := a.interfaceEgressDetails(egress)
	if ifaceErr != nil {
		return config.MVPConfig{}, ifaceErr
	}
	if parsedServer := net.ParseIP(server); parsedServer != nil && parsedServer.To4() == nil {
		if v6Source, _, v6Err := a.interfaceEgressDetailsIPv6(egress); v6Err == nil && v6Source != "" {
			source = v6Source
		}
	}
	corePath, coreErr := locateBundledCore()
	if node.Type != nodes.TypeHTTP && (coreErr != nil || corePath == "") {
		return config.MVPConfig{}, errors.New("sing-box core binary is required to verify proxy node before startup")
	}

	candidate := node
	candidate.ResolvedIP = server
	probeCtx, probeCancel := context.WithTimeout(a.ctx, 8*time.Second)
	defer probeCancel()

	probe := nodes.ProbeWithOptions(probeCtx, candidate, credentials, nodes.ProbeOptions{
		CorePath:      corePath,
		BindInterface: bindInterface,
		SourceIP:      source,
	})
	if !probe.Available {
		a.observations.Log(observability.LevelWarning, "proxy", "Selected proxy endpoint failed health check, proceeding with warning", "", map[string]any{"error": probe.Error, "category": probe.ErrorCategory, "server": server})
	}

	if resolved != "" {
		if _, commitErr := store.CommitResolvedIP(node.ID, resolved); commitErr != nil {
			return config.MVPConfig{}, commitErr
		}
	}

	input.Proxy = buildMVPProxyFromNode(node, credentials, server)
	input.ProxyChain = []config.MVPProxy{*input.Proxy}
	return input, nil
}

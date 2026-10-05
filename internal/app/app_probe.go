package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"winrouter/internal/nodes"
	"winrouter/internal/observability"
)

func (a *App) resolveNodeDomain(server string, egress string, dnsServer string) (string, error) {
	if net.ParseIP(server) != nil {
		return server, nil
	}
	dnsAddr, err := netip.ParseAddr(dnsServer)
	dnsIs6 := err == nil && dnsAddr.Is6()
	var dnsSource string
	var dnsSourceErr error
	if dnsIs6 {
		if egress == nodes.EgressA {
			dnsSource, dnsSourceErr = a.interfaceASourceIPv6()
		} else {
			dnsSource, dnsSourceErr = a.interfaceBSourceIPv6()
		}
	} else {
		if egress == nodes.EgressA {
			dnsSource, dnsSourceErr = a.interfaceASourceIPv4()
		} else {
			dnsSource, dnsSourceErr = a.interfaceBSourceIPv4()
		}
	}
	if dnsSourceErr != nil || dnsSource == "" {
		return "", fmt.Errorf("interface %s has no usable source address for DNS", strings.ToUpper(egress))
	}
	ctx, cancel := context.WithTimeout(a.ctx, 8*time.Second)
	defer cancel()
	resolved, resolveErr := nodes.ResolveEndpoint(ctx, server, dnsServer, dnsSource)
	if resolveErr != nil || resolved == "" {
		return "", fmt.Errorf("proxy DNS via interface %s: %v", strings.ToUpper(egress), resolveErr)
	}
	return resolved, nil
}

func (a *App) TestProxyChain(ids []string, dnsServer string) (nodes.TestResult, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.TestResult{}, err
	}
	if len(ids) == 0 {
		return nodes.TestResult{}, errors.New("no proxy nodes in chain to test")
	}
	chainNodes := make([]nodes.Node, 0, len(ids))
	chainCreds := make([]nodes.Credentials, 0, len(ids))
	for idx, id := range ids {
		n, c, err := store.Credentials(id)
		if err != nil {
			return nodes.TestResult{}, fmt.Errorf("node %d (%q): %w", idx+1, id, err)
		}
		chainNodes = append(chainNodes, n)
		chainCreds = append(chainCreds, c)
	}

	hop0 := chainNodes[0]
	egress := hop0.Egress
	if egress == "" {
		egress = nodes.EgressB
	}
	server := hop0.Server
	if net.ParseIP(server) == nil {
		if strings.TrimSpace(dnsServer) == "" {
			dnsServer = "223.5.5.5"
			if egress == nodes.EgressB {
				dnsServer = "1.1.1.1"
			}
		}
		res, err := a.resolveNodeDomain(server, egress, dnsServer)
		if err != nil {
			return nodes.TestResult{
				NodeID:        hop0.ID,
				TestedAt:      time.Now().UTC(),
				ErrorCategory: nodes.ErrorCategoryDNSUnavailable,
				Error:         err.Error(),
			}, nil
		}
		server = res
		chainNodes[0].ResolvedIP = res
	}

	source, bindInterface, ifaceErr := a.interfaceEgressDetails(egress)
	if ifaceErr != nil {
		return nodes.TestResult{
			NodeID:        hop0.ID,
			TestedAt:      time.Now().UTC(),
			ErrorCategory: nodes.ErrorCategoryEgressDown,
			Error:         ifaceErr.Error(),
		}, nil
	}
	if parsedServer := net.ParseIP(server); parsedServer != nil && parsedServer.To4() == nil {
		if v6Source, _, v6Err := a.interfaceEgressDetailsIPv6(egress); v6Err == nil && v6Source != "" {
			source = v6Source
		}
	}
	corePath, coreErr := locateBundledCore()
	if coreErr != nil || corePath == "" {
		return nodes.TestResult{
			NodeID:        chainNodes[len(chainNodes)-1].ID,
			TestedAt:      time.Now().UTC(),
			ErrorCategory: nodes.ErrorCategoryCoreFailed,
			Error:         "sing-box core binary is required for proxy chain testing",
		}, nil
	}

	probeCtx, probeCancel := context.WithTimeout(a.ctx, 12*time.Second)
	defer probeCancel()

	result := nodes.ProbeChainWithOptions(probeCtx, chainNodes, chainCreds, nodes.ProbeOptions{
		CorePath:      corePath,
		BindInterface: bindInterface,
		SourceIP:      source,
	})

	level := observability.LevelInfo
	if !result.Available {
		level = observability.LevelWarning
	}
	a.observations.Log(level, "proxy", "Proxy chain availability tested", "", map[string]any{
		"chain_length": len(ids),
		"available":    result.Available,
		"latency_ms":   result.LatencyMS,
		"status":       result.Status,
	})
	return result, nil
}

func (a *App) SpeedTestProxyNodes(dnsServer string) ([]nodes.TestResult, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nil, err
	}
	items := store.List()
	results := make([]nodes.TestResult, len(items))
	semaphore := make(chan struct{}, 4)
	var group sync.WaitGroup
	for index, item := range items {
		index, item := index, item
		group.Add(1)
		go func() {
			defer group.Done()
			semaphore <- struct{}{}
			defer func() { <-semaphore }()
			result, testErr := a.TestProxyNode(item.ID, dnsServer)
			if testErr != nil {
				result = nodes.TestResult{NodeID: item.ID, TestedAt: time.Now().UTC(), Error: testErr.Error()}
			}
			results[index] = result
		}()
	}
	group.Wait()
	return results, nil
}

func (a *App) TestProxyNode(id, dnsServer string) (nodes.TestResult, error) {
	store, err := a.getNodeStore()
	if err != nil {
		return nodes.TestResult{}, err
	}
	node, credentials, err := store.Credentials(id)
	if err != nil {
		return nodes.TestResult{}, err
	}
	ctx, cancel := context.WithTimeout(a.ctx, 10*time.Second)
	defer cancel()
	egress := node.Egress
	if egress == "" {
		egress = nodes.EgressB
	}
	source, bindInterface, ifaceErr := a.interfaceEgressDetails(egress)
	if ifaceErr != nil {
		result := nodes.TestResult{
			NodeID:        node.ID,
			TestedAt:      time.Now().UTC(),
			ErrorCategory: nodes.ErrorCategoryEgressDown,
			Error:         ifaceErr.Error(),
		}
		return result, nil
	}
	if net.ParseIP(node.Server) == nil {
		dnsAddr, err := netip.ParseAddr(dnsServer)
		dnsIs6 := err == nil && dnsAddr.Is6()
		var dnsSource string
		var dnsSourceErr error
		if dnsIs6 {
			if egress == nodes.EgressA {
				dnsSource, dnsSourceErr = a.interfaceASourceIPv6()
			} else {
				dnsSource, dnsSourceErr = a.interfaceBSourceIPv6()
			}
		} else {
			if egress == nodes.EgressA {
				dnsSource, dnsSourceErr = a.interfaceASourceIPv4()
			} else {
				dnsSource, dnsSourceErr = a.interfaceBSourceIPv4()
			}
		}
		if dnsSourceErr != nil || dnsSource == "" {
			result := nodes.TestResult{
				NodeID:        node.ID,
				TestedAt:      time.Now().UTC(),
				ErrorCategory: nodes.ErrorCategoryEgressDown,
				Error:         fmt.Sprintf("interface %s has no usable source address for DNS", strings.ToUpper(egress)),
			}
			return result, nil
		}
		resolved, resolveErr := nodes.ResolveEndpoint(ctx, node.Server, dnsServer, dnsSource)
		if resolveErr != nil {
			result := nodes.TestResult{
				NodeID:        node.ID,
				TestedAt:      time.Now().UTC(),
				ErrorCategory: nodes.ErrorCategoryDNSUnavailable,
				Error:         fmt.Sprintf("proxy DNS via interface %s: %v", strings.ToUpper(egress), resolveErr),
			}
			return result, nil
		}
		node.ResolvedIP = resolved
	}
	corePath, coreErr := locateBundledCore()
	if node.Type != nodes.TypeHTTP && (coreErr != nil || corePath == "") {
		result := nodes.TestResult{
			NodeID:        node.ID,
			TestedAt:      time.Now().UTC(),
			ErrorCategory: nodes.ErrorCategoryCoreFailed,
			Error:         "sing-box core binary is required for non-HTTP protocol testing but not found",
		}
		return result, nil
	}
	probeSource := source
	if parsedServer := net.ParseIP(node.ResolvedIP); parsedServer != nil && parsedServer.To4() == nil {
		if v6Source, _, v6Err := a.interfaceEgressDetailsIPv6(egress); v6Err == nil && v6Source != "" {
			probeSource = v6Source
		}
	} else if parsedServer := net.ParseIP(node.Server); parsedServer != nil && parsedServer.To4() == nil {
		if v6Source, _, v6Err := a.interfaceEgressDetailsIPv6(egress); v6Err == nil && v6Source != "" {
			probeSource = v6Source
		}
	}
	result := nodes.ProbeWithOptions(ctx, node, credentials, nodes.ProbeOptions{
		CorePath:      corePath,
		BindInterface: bindInterface,
		SourceIP:      probeSource,
	})
	if result.Available && net.ParseIP(node.Server) == nil && node.ResolvedIP != "" {
		if _, commitErr := store.CommitResolvedIP(node.ID, node.ResolvedIP); commitErr != nil {
			return nodes.TestResult{}, commitErr
		}
	}
	level := observability.LevelInfo
	if !result.Available {
		level = observability.LevelWarning
	}
	a.observations.Log(level, "proxy", "Proxy node availability tested", "", map[string]any{
		"node_id":            node.ID,
		"type":               node.Type,
		"egress":             egress,
		"available":          result.Available,
		"tcp_reachable":      result.TCPReachable,
		"protocol_available": result.ProtocolAvailable,
		"tcp_ms":             result.TCPMS,
		"protocol_ms":        result.ProtocolMS,
		"total_ms":           result.TotalMS,
		"latency_ms":         result.LatencyMS,
		"status":             result.Status,
		"error_category":     result.ErrorCategory,
	})
	return result, nil
}

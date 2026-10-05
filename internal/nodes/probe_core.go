package nodes

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type ephemeralSingBoxConfig struct {
	Log       ephemeralLogConfig      `json:"log"`
	Inbounds  []ephemeralMixedInbound `json:"inbounds"`
	Outbounds []map[string]any        `json:"outbounds"`
	Route     ephemeralRouteConfig    `json:"route"`
}

type ephemeralLogConfig struct {
	Level    string `json:"level"`
	Disabled bool   `json:"disabled,omitempty"`
}

type ephemeralMixedInbound struct {
	Type       string `json:"type"`
	Tag        string `json:"tag"`
	Listen     string `json:"listen"`
	ListenPort uint16 `json:"listen_port"`
}

type ephemeralRouteConfig struct {
	Final string `json:"final"`
}

func buildEphemeralOutbound(node Node, credentials Credentials, tag, bindInterface, detour string) map[string]any {
	server := node.ResolvedIP
	if server == "" {
		server = node.Server
	}
	proxyOutbound := map[string]any{
		"type":        node.Type,
		"tag":         tag,
		"server":      server,
		"server_port": node.Port,
	}
	if bindInterface != "" {
		proxyOutbound["bind_interface"] = bindInterface
	}
	if detour != "" {
		proxyOutbound["detour"] = detour
	}

	switch node.Type {
	case TypeShadowsocks:
		method := credentials.Method
		if method == "" {
			method = credentials.Username
		}
		proxyOutbound["method"] = method
		proxyOutbound["password"] = credentials.Password
	case TypeVMess:
		proxyOutbound["uuid"] = credentials.UUID
		proxyOutbound["security"] = "auto"
	case TypeVLESS:
		proxyOutbound["uuid"] = credentials.UUID
		if credentials.Flow != "" {
			proxyOutbound["flow"] = credentials.Flow
		}
	case TypeTrojan:
		proxyOutbound["password"] = credentials.Password
	}

	if node.TLS != nil && node.TLS.Enabled {
		tlsConfig := map[string]any{
			"enabled": true,
		}
		sni := node.TLS.ServerName
		if sni == "" && node.Server != "" && net.ParseIP(node.Server) == nil {
			sni = node.Server
		}
		if sni != "" {
			tlsConfig["server_name"] = sni
		}
		if node.TLS.Insecure {
			tlsConfig["insecure"] = true
		}
		var cleanALPN []string
		for _, a := range node.TLS.ALPN {
			trimmed := strings.TrimSpace(a)
			if trimmed != "" && !strings.EqualFold(trimmed, "default") {
				cleanALPN = append(cleanALPN, trimmed)
			}
		}
		if len(cleanALPN) > 0 {
			tlsConfig["alpn"] = cleanALPN
		}
		proxyOutbound["tls"] = tlsConfig
	} else if node.Type == TypeTrojan {
		proxyOutbound["tls"] = map[string]any{
			"enabled":     true,
			"server_name": node.Server,
		}
	}

	if node.Transport != nil && node.Transport.Type != "" {
		transportConfig := map[string]any{
			"type": node.Transport.Type,
		}
		if node.Transport.Path != "" {
			transportConfig["path"] = node.Transport.Path
		}
		if node.Transport.Host != "" {
			transportConfig["headers"] = map[string][]string{
				"Host": {node.Transport.Host},
			}
		}
		proxyOutbound["transport"] = transportConfig
	}
	return proxyOutbound
}

func probeWithEphemeralCore(ctx context.Context, node Node, credentials Credentials, server, corePath, bindInterface, targetURL string, result *TestResult) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = fmt.Sprintf("failed to allocate local test port: %v", err)
		return
	}
	localPort := uint16(listener.Addr().(*net.TCPAddr).Port)
	_ = listener.Close()

	if server != "" {
		node.ResolvedIP = server
	}
	proxyOutbound := buildEphemeralOutbound(node, credentials, "proxy", bindInterface, "")
	runEphemeralSingBox(ctx, localPort, []map[string]any{proxyOutbound, {"type": "direct", "tag": "direct"}}, corePath, targetURL, result)
}

func probeChainWithEphemeralCore(ctx context.Context, chainNodes []Node, credentials []Credentials, corePath, bindInterface, targetURL string, result *TestResult) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = fmt.Sprintf("failed to allocate local test port: %v", err)
		return
	}
	localPort := uint16(listener.Addr().(*net.TCPAddr).Port)
	_ = listener.Close()

	outbounds := make([]map[string]any, 0, len(chainNodes)+1)
	for i, node := range chainNodes {
		var tag, bind, detour string
		if i == 0 {
			tag = "proxy-hop-0"
			bind = bindInterface
		} else if i == len(chainNodes)-1 {
			tag = "proxy"
			detour = fmt.Sprintf("proxy-hop-%d", i-1)
		} else {
			tag = fmt.Sprintf("proxy-hop-%d", i)
			detour = fmt.Sprintf("proxy-hop-%d", i-1)
		}
		outbounds = append(outbounds, buildEphemeralOutbound(node, credentials[i], tag, bind, detour))
	}
	outbounds = append(outbounds, map[string]any{"type": "direct", "tag": "direct"})

	runEphemeralSingBox(ctx, localPort, outbounds, corePath, targetURL, result)
}

func runEphemeralSingBox(ctx context.Context, localPort uint16, outbounds []map[string]any, corePath, targetURL string, result *TestResult) {
	config := ephemeralSingBoxConfig{
		Log: ephemeralLogConfig{Level: "panic"},
		Inbounds: []ephemeralMixedInbound{
			{Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1", ListenPort: localPort},
		},
		Outbounds: outbounds,
		Route:     ephemeralRouteConfig{Final: "proxy"},
	}

	configData, err := json.Marshal(config)
	if err != nil {
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = fmt.Sprintf("failed to marshal ephemeral config: %v", err)
		return
	}

	tempDir, err := os.MkdirTemp("", "winrouter-probe-*")
	if err != nil {
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = fmt.Sprintf("failed to create temp dir: %v", err)
		return
	}
	defer os.RemoveAll(tempDir)

	tempConfigFile := filepath.Join(tempDir, "config.json")
	if err := os.WriteFile(tempConfigFile, configData, 0o600); err != nil {
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = fmt.Sprintf("failed to write ephemeral config: %v", err)
		return
	}

	// Start ephemeral core with context cancellation
	coreCtx, cancelCore := context.WithCancel(ctx)
	defer cancelCore()

	cmd := exec.CommandContext(coreCtx, corePath, "run", "-c", tempConfigFile)
	setNoWindowSysProcAttr(cmd)

	if err := cmd.Start(); err != nil {
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = fmt.Sprintf("failed to start ephemeral core: %v", err)
		return
	}

	// Ensure core process cleanup
	defer func() {
		cancelCore()
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		_ = cmd.Wait()
	}()

	// Wait for local mixed inbound to be ready
	readyAddr := fmt.Sprintf("127.0.0.1:%d", localPort)
	ready := false
	for i := 0; i < 30; i++ {
		select {
		case <-ctx.Done():
			result.ErrorCategory = ErrorCategoryTimeout
			result.Error = "timeout waiting for ephemeral core"
			return
		default:
		}
		conn, err := net.DialTimeout("tcp", readyAddr, 50*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			ready = true
			break
		}
		time.Sleep(30 * time.Millisecond)
	}

	if !ready {
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = "ephemeral core port not ready"
		return
	}

	// Make HTTP request through ephemeral proxy
	proxyURL, _ := url.Parse(fmt.Sprintf("http://127.0.0.1:%d", localPort))
	httpClient := &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyURL(proxyURL),
		},
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		result.ErrorCategory = ErrorCategoryTargetFailed
		result.Error = err.Error()
		return
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		classifyClientError(err, ctx, result)
		return
	}
	defer resp.Body.Close()

	result.Status = resp.StatusCode
	switch resp.StatusCode {
	case http.StatusNoContent, http.StatusOK:
		result.ProtocolAvailable = true
		result.Available = true
	case http.StatusProxyAuthRequired:
		result.ErrorCategory = ErrorCategoryAuthFailed
		result.Error = "proxy authentication failed"
	default:
		result.ErrorCategory = ErrorCategoryProtocolFailed
		result.Error = fmt.Sprintf("unexpected response status: %d", resp.StatusCode)
	}
}

func classifyClientError(err error, ctx context.Context, result *TestResult) {
	if ctx.Err() == context.DeadlineExceeded || strings.Contains(err.Error(), "deadline") || strings.Contains(err.Error(), "timeout") {
		result.ErrorCategory = ErrorCategoryTimeout
		result.Error = "request timed out"
		return
	}
	errStr := strings.ToLower(err.Error())
	if strings.Contains(errStr, "tls") || strings.Contains(errStr, "certificate") || strings.Contains(errStr, "handshake") {
		result.ErrorCategory = ErrorCategoryTLSFailed
		result.Error = "TLS handshake failed"
		return
	}
	if strings.Contains(errStr, "auth") || strings.Contains(errStr, "407") {
		result.ErrorCategory = ErrorCategoryAuthFailed
		result.Error = "authentication failed"
		return
	}
	result.ErrorCategory = ErrorCategoryProtocolFailed
	result.Error = fmt.Sprintf("protocol connection failed: %v", err)
}

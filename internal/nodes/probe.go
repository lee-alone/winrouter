package nodes

import (
	"bufio"
	"context"
	"encoding/base64"
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

const (
	ErrorCategoryDNSUnavailable = "dns_failed"
	ErrorCategoryEgressDown     = "egress_unavailable"
	ErrorCategoryTCPFailed      = "tcp_failed"
	ErrorCategoryTLSFailed      = "tls_failed"
	ErrorCategoryAuthFailed     = "authentication_failed"
	ErrorCategoryProtocolFailed = "protocol_failed"
	ErrorCategoryTargetFailed   = "target_failed"
	ErrorCategoryTimeout        = "timeout"
	ErrorCategoryCoreFailed     = "core_failed"

	DefaultTestTarget = "http://www.google.com/generate_204"
)

type ProbeOptions struct {
	CorePath      string
	TestTarget    string
	BindInterface string
	SourceIP      string
}

func Probe(ctx context.Context, node Node, credentials Credentials) TestResult {
	return ProbeWithOptions(ctx, node, credentials, ProbeOptions{})
}

func ProbeWithCore(ctx context.Context, node Node, credentials Credentials, corePath string) TestResult {
	return ProbeWithOptions(ctx, node, credentials, ProbeOptions{CorePath: corePath})
}

func ProbeWithOptions(ctx context.Context, node Node, credentials Credentials, opts ProbeOptions) TestResult {
	started := time.Now().UTC()
	result := TestResult{
		NodeID:   node.ID,
		TestedAt: started,
	}

	targetURL := opts.TestTarget
	if targetURL == "" {
		targetURL = DefaultTestTarget
	}

	server := node.ResolvedIP
	if server == "" {
		server = node.Server
	}

	// Stage 1: TCP Reachability Probe with optional SourceIP binding
	tcpStart := time.Now()
	dialer := dialerForEndpoint(server, opts.SourceIP)
	tcpConn, tcpErr := dialer.DialContext(ctx, "tcp", net.JoinHostPort(server, fmt.Sprint(node.Port)))
	result.TCPMS = time.Since(tcpStart).Milliseconds()

	if tcpErr != nil {
		result.TCPReachable = false
		result.ErrorCategory = ErrorCategoryTCPFailed
		if ctx.Err() == context.DeadlineExceeded {
			result.ErrorCategory = ErrorCategoryTimeout
		}
		result.Error = fmt.Sprintf("TCP connect failed: %v", tcpErr)
		result.TotalMS = result.TCPMS
		result.LatencyMS = result.TCPMS
		return result
	}
	_ = tcpConn.Close()
	result.TCPReachable = true

	// Stage 2: Protocol-level Probe
	protoStart := time.Now()
	if node.Type == TypeHTTP {
		probeHTTPProtocol(ctx, node, credentials, server, opts.SourceIP, &result)
	} else if opts.CorePath != "" {
		probeWithEphemeralCore(ctx, node, credentials, server, opts.CorePath, opts.BindInterface, targetURL, &result)
	} else {
		// Non-HTTP node without sing-box binary CANNOT be tested; fail with core_failed strictly!
		result.ProtocolAvailable = false
		result.Available = false
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = "sing-box core binary is required for protocol probe but not found"
	}

	result.ProtocolMS = time.Since(protoStart).Milliseconds()
	result.TotalMS = time.Since(started).Milliseconds()
	result.LatencyMS = result.ProtocolMS
	if result.LatencyMS <= 0 {
		result.LatencyMS = result.TotalMS
	}
	return result
}

func probeHTTPProtocol(ctx context.Context, node Node, credentials Credentials, server, sourceIP string, result *TestResult) {
	dialer := dialerForEndpoint(server, sourceIP)
	conn, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(server, fmt.Sprint(node.Port)))
	if err != nil {
		result.ErrorCategory = ErrorCategoryTCPFailed
		result.Error = "connection failed"
		return
	}
	defer conn.Close()

	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	request := "CONNECT 1.1.1.1:443 HTTP/1.1\r\nHost: 1.1.1.1:443\r\nProxy-Connection: close\r\n"
	if credentials.Username != "" || credentials.Password != "" {
		token := base64.StdEncoding.EncodeToString([]byte(credentials.Username + ":" + credentials.Password))
		request += "Proxy-Authorization: Basic " + token + "\r\n"
	}
	if _, err := fmt.Fprint(conn, request+"\r\n"); err != nil {
		result.ErrorCategory = ErrorCategoryProtocolFailed
		result.Error = "request failed"
		return
	}

	response, err := http.ReadResponse(bufio.NewReader(conn), &http.Request{Method: http.MethodConnect})
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			result.ErrorCategory = ErrorCategoryTimeout
			result.Error = "request timeout"
		} else {
			result.ErrorCategory = ErrorCategoryProtocolFailed
			result.Error = "invalid proxy response"
		}
		return
	}
	defer response.Body.Close()

	result.Status = response.StatusCode
	if response.StatusCode >= 200 && response.StatusCode < 300 {
		result.ProtocolAvailable = true
		result.Available = true
	} else if response.StatusCode == http.StatusProxyAuthRequired {
		result.ErrorCategory = ErrorCategoryAuthFailed
		result.Error = "proxy authentication required"
	} else {
		result.ErrorCategory = ErrorCategoryProtocolFailed
		result.Error = strings.TrimSpace(response.Status)
	}
}

// A LocalAddr must use the same address family as the remote endpoint. The
// egress selector currently provides an IPv4 source address, which must not
// be applied to an IPv6 node or Windows may wait until the probe deadline.
func dialerForEndpoint(server, sourceIP string) net.Dialer {
	dialer := net.Dialer{}
	remoteIP := net.ParseIP(server)
	source := net.ParseIP(sourceIP)
	if source == nil || remoteIP == nil {
		return dialer
	}
	if (remoteIP.To4() == nil) != (source.To4() == nil) {
		return dialer
	}
	dialer.LocalAddr = &net.TCPAddr{IP: source}
	return dialer
}

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

func probeWithEphemeralCore(ctx context.Context, node Node, credentials Credentials, server, corePath, bindInterface, targetURL string, result *TestResult) {
	// Pick an ephemeral local port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = fmt.Sprintf("failed to allocate local test port: %v", err)
		return
	}
	localPort := uint16(listener.Addr().(*net.TCPAddr).Port)
	_ = listener.Close()

	// Build ephemeral sing-box outbound config with optional bind_interface
	proxyOutbound := map[string]any{
		"type":        node.Type,
		"tag":         "proxy",
		"server":      server,
		"server_port": node.Port,
	}
	if bindInterface != "" {
		proxyOutbound["bind_interface"] = bindInterface
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

	config := ephemeralSingBoxConfig{
		Log: ephemeralLogConfig{Level: "panic"},
		Inbounds: []ephemeralMixedInbound{
			{Type: "mixed", Tag: "mixed-in", Listen: "127.0.0.1", ListenPort: localPort},
		},
		Outbounds: []map[string]any{
			proxyOutbound,
			{"type": "direct", "tag": "direct"},
		},
		Route: ephemeralRouteConfig{Final: "proxy"},
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
	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusOK {
		result.ProtocolAvailable = true
		result.Available = true
	} else if resp.StatusCode == http.StatusProxyAuthRequired {
		result.ErrorCategory = ErrorCategoryAuthFailed
		result.Error = "proxy authentication failed"
	} else {
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

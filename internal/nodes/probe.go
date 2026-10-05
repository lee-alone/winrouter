package nodes

import (
	"context"
	"fmt"
	"net"
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

func ProbeChainWithOptions(ctx context.Context, chainNodes []Node, credentials []Credentials, opts ProbeOptions) TestResult {
	started := time.Now().UTC()
	if len(chainNodes) == 0 {
		return TestResult{
			TestedAt:      started,
			ErrorCategory: ErrorCategoryCoreFailed,
			Error:         "no proxy nodes in chain to probe",
		}
	}
	if len(chainNodes) == 1 {
		return ProbeWithOptions(ctx, chainNodes[0], credentials[0], opts)
	}

	result := TestResult{
		NodeID:   chainNodes[len(chainNodes)-1].ID,
		TestedAt: started,
	}

	targetURL := opts.TestTarget
	if targetURL == "" {
		targetURL = DefaultTestTarget
	}

	hop0 := chainNodes[0]
	server0 := hop0.ResolvedIP
	if server0 == "" {
		server0 = hop0.Server
	}
	tcpStart := time.Now()
	dialer := dialerForEndpoint(server0, opts.SourceIP)
	tcpConn, tcpErr := dialer.DialContext(ctx, "tcp", net.JoinHostPort(server0, fmt.Sprint(hop0.Port)))
	result.TCPMS = time.Since(tcpStart).Milliseconds()
	if tcpErr != nil {
		result.TCPReachable = false
		result.ErrorCategory = ErrorCategoryTCPFailed
		if ctx.Err() == context.DeadlineExceeded {
			result.ErrorCategory = ErrorCategoryTimeout
		}
		result.Error = fmt.Sprintf("hop 0 TCP connect failed: %v", tcpErr)
		result.TotalMS = result.TCPMS
		result.LatencyMS = result.TCPMS
		return result
	}
	_ = tcpConn.Close()
	result.TCPReachable = true

	if opts.CorePath == "" {
		result.ProtocolAvailable = false
		result.Available = false
		result.ErrorCategory = ErrorCategoryCoreFailed
		result.Error = "sing-box core binary is required for chain probe but not found"
		result.TotalMS = result.TCPMS
		result.LatencyMS = result.TCPMS
		return result
	}

	protoStart := time.Now()
	probeChainWithEphemeralCore(ctx, chainNodes, credentials, opts.CorePath, opts.BindInterface, targetURL, &result)
	result.ProtocolMS = time.Since(protoStart).Milliseconds()
	result.TotalMS = time.Since(started).Milliseconds()
	result.LatencyMS = result.ProtocolMS
	if result.LatencyMS <= 0 {
		result.LatencyMS = result.TotalMS
	}
	return result
}

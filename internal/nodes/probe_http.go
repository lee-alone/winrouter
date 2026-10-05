package nodes

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
)

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

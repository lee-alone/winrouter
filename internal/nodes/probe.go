package nodes

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

func ProbeHTTP(ctx context.Context, node Node, credentials Credentials) TestResult {
	started := time.Now()
	result := TestResult{NodeID: node.ID, TestedAt: started}
	dialer := net.Dialer{}
	server := node.ResolvedIP
	if server == "" {
		server = node.Server
	}
	connection, err := dialer.DialContext(ctx, "tcp", net.JoinHostPort(server, fmt.Sprint(node.Port)))
	if err != nil {
		result.Error = "connection failed"
		return result
	}
	defer connection.Close()
	if deadline, ok := ctx.Deadline(); ok {
		_ = connection.SetDeadline(deadline)
	}
	request := "CONNECT 1.1.1.1:443 HTTP/1.1\r\nHost: 1.1.1.1:443\r\nProxy-Connection: close\r\n"
	if credentials.Username != "" {
		token := base64.StdEncoding.EncodeToString([]byte(credentials.Username + ":" + credentials.Password))
		request += "Proxy-Authorization: Basic " + token + "\r\n"
	}
	if _, err := fmt.Fprint(connection, request+"\r\n"); err != nil {
		result.Error = "request failed"
		return result
	}
	response, err := http.ReadResponse(bufio.NewReader(connection), &http.Request{Method: http.MethodConnect})
	if err != nil {
		result.Error = "invalid proxy response"
		return result
	}
	defer response.Body.Close()
	result.LatencyMS = time.Since(started).Milliseconds()
	result.Status = response.StatusCode
	result.Available = response.StatusCode >= 200 && response.StatusCode < 300
	if !result.Available {
		result.Error = strings.TrimSpace(response.Status)
	}
	return result
}

func Probe(ctx context.Context, node Node, credentials Credentials) TestResult {
	if node.Type == TypeHTTP {
		return ProbeHTTP(ctx, node, credentials)
	}
	started := time.Now()
	result := TestResult{NodeID: node.ID, TestedAt: started}
	server := node.ResolvedIP
	if server == "" {
		server = node.Server
	}
	connection, err := (&net.Dialer{}).DialContext(ctx, "tcp", net.JoinHostPort(server, fmt.Sprint(node.Port)))
	if err != nil {
		result.Error = "connection failed"
		return result
	}
	connection.Close()
	result.Available = true
	result.LatencyMS = time.Since(started).Milliseconds()
	return result
}

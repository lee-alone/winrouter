package dnssettings

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

const probeName = "example.com"

// TestResult contains only the outcome needed by the settings UI.
type TestResult struct {
	Success    bool   `json:"success"`
	DurationMS int64  `json:"duration_ms"`
	Error      string `json:"error,omitempty"`
}

// Test verifies that an upstream can answer a normal A query. It uses the
// supplied values directly, so unsaved settings can be tested safely.
func Test(ctx context.Context, server Server) TestResult {
	started := time.Now()
	result := TestResult{}
	if err := validateServer("DNS", server); err != nil {
		result.Error = err.Error()
		result.DurationMS = time.Since(started).Milliseconds()
		return result
	}
	ctx, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()
	packet, err := queryPacket(probeName)
	if err == nil {
		switch server.Type {
		case "udp":
			err = testUDP(ctx, server, packet)
		case "tls":
			err = testTLS(ctx, server, packet)
		case "https":
			err = testHTTPS(ctx, server, packet)
		}
	}
	result.DurationMS = time.Since(started).Milliseconds()
	if err != nil {
		result.Error = err.Error()
		return result
	}
	result.Success = true
	return result
}

func testUDP(ctx context.Context, server Server, packet []byte) error {
	connection, err := (&net.Dialer{}).DialContext(ctx, "udp", endpoint(server))
	if err != nil {
		return fmt.Errorf("connect: %w", err)
	}
	defer connection.Close()
	deadline, _ := ctx.Deadline()
	_ = connection.SetDeadline(deadline)
	if _, err = connection.Write(packet); err != nil {
		return fmt.Errorf("send query: %w", err)
	}
	response := make([]byte, 4096)
	count, err := connection.Read(response)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	return validateResponse(packet, response[:count])
}

func testTLS(ctx context.Context, server Server, packet []byte) error {
	dialer := tls.Dialer{Config: &tls.Config{ServerName: server.ServerName, MinVersion: tls.VersionTLS12}}
	connection, err := dialer.DialContext(ctx, "tcp", endpoint(server))
	if err != nil {
		return fmt.Errorf("TLS connect: %w", err)
	}
	defer connection.Close()
	deadline, _ := ctx.Deadline()
	_ = connection.SetDeadline(deadline)
	framed := make([]byte, 2+len(packet))
	binary.BigEndian.PutUint16(framed, uint16(len(packet)))
	copy(framed[2:], packet)
	if _, err = connection.Write(framed); err != nil {
		return fmt.Errorf("send query: %w", err)
	}
	var length uint16
	if err = binary.Read(connection, binary.BigEndian, &length); err != nil {
		return fmt.Errorf("read response length: %w", err)
	}
	if length < 12 || length > 4096 {
		return errors.New("invalid DNS response length")
	}
	response := make([]byte, length)
	if _, err = io.ReadFull(connection, response); err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	return validateResponse(packet, response)
}

func testHTTPS(ctx context.Context, server Server, packet []byte) error {
	transport := &http.Transport{TLSClientConfig: &tls.Config{ServerName: server.ServerName, MinVersion: tls.VersionTLS12}, Proxy: nil}
	client := &http.Client{Transport: transport}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://"+endpoint(server)+"/dns-query", bytes.NewReader(packet))
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/dns-message")
	request.Header.Set("Content-Type", "application/dns-message")
	request.Host = server.ServerName
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("HTTPS query: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTPS server returned %s", response.Status)
	}
	body, err := io.ReadAll(io.LimitReader(response.Body, 4097))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if len(body) > 4096 {
		return errors.New("DNS response is too large")
	}
	return validateResponse(packet, body)
}

func endpoint(server Server) string {
	return net.JoinHostPort(server.Server, fmt.Sprintf("%d", server.Port))
}

func queryPacket(name string) ([]byte, error) {
	packet := make([]byte, 12)
	binary.BigEndian.PutUint16(packet, uint16(time.Now().UnixNano()))
	binary.BigEndian.PutUint16(packet[2:], 0x0100)
	binary.BigEndian.PutUint16(packet[4:], 1)
	for _, label := range strings.Split(name, ".") {
		if len(label) == 0 || len(label) > 63 {
			return nil, errors.New("invalid DNS probe name")
		}
		packet = append(packet, byte(len(label)))
		packet = append(packet, label...)
	}
	return append(packet, 0, 0, 1, 0, 1), nil
}

func validateResponse(query, response []byte) error {
	if len(response) < 12 || response[0] != query[0] || response[1] != query[1] || response[2]&0x80 == 0 {
		return errors.New("invalid DNS response")
	}
	if code := response[3] & 0x0f; code != 0 {
		return fmt.Errorf("DNS server returned response code %d", code)
	}
	return nil
}

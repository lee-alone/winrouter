package observability

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"strings"
	"time"

	"winrouter/internal/interfaces"
)

func RunProbe(ctx context.Context, request ProbeRequest, adapters []interfaces.Adapter) ProbeResult {
	started := time.Now()
	result := ProbeResult{Time: started.UTC(), Protocol: strings.ToLower(request.Protocol), Target: request.Target, ExpectedInterface: request.ExpectedInterface}
	timeout := time.Duration(request.TimeoutMS) * time.Millisecond
	if timeout <= 0 || timeout > 30*time.Second {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	boundSource, err := sourceForInterface(request.ExpectedInterface, adapters)
	if err != nil {
		result.DurationMS = time.Since(started).Milliseconds()
		result.Error = err.Error()
		return result
	}
	var source string
	switch result.Protocol {
	case "tcp":
		source, err = probeTCP(ctx, request.Target, boundSource)
	case "udp":
		source, err = probeUDP(ctx, request.Target, boundSource)
	case "dns":
		source, err = probeDNS(ctx, request.Target, request.DNSName, boundSource)
	default:
		err = fmt.Errorf("unsupported probe protocol %q", request.Protocol)
	}
	result.DurationMS = time.Since(started).Milliseconds()
	result.SourceAddress = source
	result.ActualInterface = interfaceForSource(source, adapters)
	if err != nil {
		result.Error = err.Error()
		return result
	}
	if result.ExpectedInterface != "" && !strings.EqualFold(strings.Trim(result.ExpectedInterface, "{}"), strings.Trim(result.ActualInterface, "{}")) {
		result.Error = fmt.Sprintf("expected interface %s, actual %s", result.ExpectedInterface, emptyAsUnknown(result.ActualInterface))
		return result
	}
	result.Success = true
	return result
}

func probeTCP(ctx context.Context, target, source string) (string, error) {
	dialer := &net.Dialer{}
	if source != "" {
		dialer.LocalAddr = &net.TCPAddr{IP: net.ParseIP(source)}
	}
	connection, err := dialer.DialContext(ctx, "tcp", target)
	if err != nil {
		return "", err
	}
	defer connection.Close()
	return hostOnly(connection.LocalAddr().String()), nil
}

func probeUDP(ctx context.Context, target, source string) (string, error) {
	dialer := &net.Dialer{}
	if source != "" {
		dialer.LocalAddr = &net.UDPAddr{IP: net.ParseIP(source)}
	}
	connection, err := dialer.DialContext(ctx, "udp", target)
	if err != nil {
		return "", err
	}
	defer connection.Close()
	deadline, _ := ctx.Deadline()
	_ = connection.SetDeadline(deadline)
	request := make([]byte, 48)
	request[0] = 0x23
	if _, err = connection.Write(request); err != nil {
		return "", err
	}
	response := make([]byte, 512)
	count, err := connection.Read(response)
	if err != nil {
		return "", err
	}
	if count < 48 {
		return "", errors.New("invalid UDP NTP response")
	}
	return hostOnly(connection.LocalAddr().String()), nil
}

func probeDNS(ctx context.Context, target, name, source string) (string, error) {
	if name == "" {
		name = "example.com"
	}
	packet, err := dnsQuery(name)
	if err != nil {
		return "", err
	}
	dialer := &net.Dialer{}
	if source != "" {
		dialer.LocalAddr = &net.UDPAddr{IP: net.ParseIP(source)}
	}
	connection, err := dialer.DialContext(ctx, "udp", target)
	if err != nil {
		return "", err
	}
	defer connection.Close()
	deadline, _ := ctx.Deadline()
	_ = connection.SetDeadline(deadline)
	if _, err = connection.Write(packet); err != nil {
		return "", err
	}
	response := make([]byte, 4096)
	count, err := connection.Read(response)
	if err != nil {
		return "", err
	}
	if count < 12 || response[0] != packet[0] || response[1] != packet[1] {
		return "", errors.New("invalid DNS response")
	}
	return hostOnly(connection.LocalAddr().String()), nil
}

func dnsQuery(name string) ([]byte, error) {
	labels := strings.Split(strings.Trim(name, "."), ".")
	packet := make([]byte, 12)
	binary.BigEndian.PutUint16(packet, uint16(time.Now().UnixNano()))
	binary.BigEndian.PutUint16(packet[2:], 0x0100)
	binary.BigEndian.PutUint16(packet[4:], 1)
	for _, label := range labels {
		if len(label) < 1 || len(label) > 63 {
			return nil, errors.New("invalid DNS name")
		}
		packet = append(packet, byte(len(label)))
		packet = append(packet, label...)
	}
	packet = append(packet, 0, 0, 1, 0, 1)
	return packet, nil
}

func hostOnly(value string) string {
	host, _, err := net.SplitHostPort(value)
	if err == nil {
		return host
	}
	return strings.Trim(value, "[]")
}
func interfaceForSource(source string, adapters []interfaces.Adapter) string {
	for _, adapter := range adapters {
		for _, address := range adapter.Addresses {
			if address.IP == source {
				return adapter.GUID
			}
		}
	}
	return ""
}

func sourceForInterface(expected string, adapters []interfaces.Adapter) (string, error) {
	if expected == "" {
		return "", nil
	}
	for _, adapter := range adapters {
		if !sameInterface(adapter.GUID, expected) {
			continue
		}
		var fallback string
		for _, address := range adapter.Addresses {
			ip := net.ParseIP(address.IP)
			if ip == nil || ip.To4() == nil || ip.IsUnspecified() {
				continue
			}
			if fallback == "" {
				fallback = address.IP
			}
			if !ip.IsLoopback() && !ip.IsLinkLocalUnicast() {
				return address.IP, nil
			}
		}
		if fallback != "" {
			return fallback, nil
		}
		return "", fmt.Errorf("expected interface %s has no usable IPv4 address", expected)
	}
	return "", fmt.Errorf("expected interface %s is not available", expected)
}

func sameInterface(left, right string) bool {
	return strings.EqualFold(strings.Trim(left, "{}"), strings.Trim(right, "{}"))
}
func emptyAsUnknown(value string) string {
	if value == "" {
		return "unknown"
	}
	return value
}

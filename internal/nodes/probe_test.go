package nodes

import (
	"bufio"
	"context"
	"net"
	"net/netip"
	"strings"
	"testing"
	"time"
)

func TestProbeHTTPConnectWithBasicAuthentication(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	requestLine := make(chan string, 1)
	authorization := make(chan string, 1)
	go func() {
		for {
			connection, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func(conn net.Conn) {
				defer conn.Close()
				reader := bufio.NewReader(conn)
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "CONNECT") {
					select {
					case requestLine <- trimmed:
					default:
					}
				}
				for {
					line, err = reader.ReadString('\n')
					if err != nil {
						break
					}
					line = strings.TrimSpace(line)
					if line == "" {
						break
					}
					if strings.HasPrefix(strings.ToLower(line), "proxy-authorization:") {
						select {
						case authorization <- line:
						default:
						}
					}
				}
				_, _ = conn.Write([]byte("HTTP/1.1 200 Connection established\r\nContent-Length: 0\r\n\r\n"))
			}(connection)
		}
	}()
	address := listener.Addr().(*net.TCPAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := Probe(ctx, Node{ID: "node-1", Type: TypeHTTP, Server: address.IP.String(), Port: uint16(address.Port)}, Credentials{Username: "alice", Password: "secret"})
	if !result.Available || !result.TCPReachable || !result.ProtocolAvailable || result.Status != 200 || result.NodeID != "node-1" {
		t.Fatalf("result = %#v", result)
	}
	select {
	case line := <-requestLine:
		if line != "CONNECT 1.1.1.1:443 HTTP/1.1" {
			t.Fatalf("request line = %q", line)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for request line")
	}
	select {
	case line := <-authorization:
		if line != "Proxy-Authorization: Basic YWxpY2U6c2VjcmV0" {
			t.Fatalf("authorization = %q", line)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for authorization")
	}
}

func TestProbeTCPUnreachableCategorization(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	result := Probe(ctx, Node{ID: "node-dead", Type: TypeHTTP, Server: "127.0.0.1", Port: 59999}, Credentials{})
	if result.TCPReachable || result.Available || result.ErrorCategory != ErrorCategoryTCPFailed {
		t.Fatalf("expected tcp_failed, got %#v", result)
	}
}

func TestProbeHTTP407AuthRequiredCategorization(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				reader := bufio.NewReader(c)
				line, err := reader.ReadString('\n')
				if err != nil {
					return
				}
				if strings.HasPrefix(strings.TrimSpace(line), "CONNECT") {
					_, _ = c.Write([]byte("HTTP/1.1 407 Proxy Authentication Required\r\nProxy-Authenticate: Basic\r\nContent-Length: 0\r\n\r\n"))
				}
			}(conn)
		}
	}()
	address := listener.Addr().(*net.TCPAddr)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	result := Probe(ctx, Node{ID: "node-auth", Type: TypeHTTP, Server: address.IP.String(), Port: uint16(address.Port)}, Credentials{})
	if !result.TCPReachable || result.Available || result.Status != 407 || result.ErrorCategory != ErrorCategoryAuthFailed {
		t.Fatalf("expected authentication_failed (407), got %#v", result)
	}
}

func TestProbeNonHTTPWithoutCoreFails(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	address := listener.Addr().(*net.TCPAddr)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	for _, proto := range []string{TypeShadowsocks, TypeVMess, TypeVLESS, TypeTrojan} {
		result := ProbeWithOptions(ctx, Node{ID: "node-test", Type: proto, Server: address.IP.String(), Port: uint16(address.Port)}, Credentials{}, ProbeOptions{CorePath: ""})
		if result.Available || result.ProtocolAvailable || result.ErrorCategory != ErrorCategoryCoreFailed {
			t.Fatalf("expected core_failed for proto %s without core, got %#v", proto, result)
		}
	}
}

func TestDialerForEndpointDoesNotBindIPv4SourceToIPv6Endpoint(t *testing.T) {
	dialer := dialerForEndpoint("2001:db8::10", "192.0.2.10")
	if dialer.LocalAddr != nil {
		t.Fatalf("expected no IPv4 LocalAddr for IPv6 endpoint, got %#v", dialer.LocalAddr)
	}

	dialer = dialerForEndpoint("192.0.2.20", "192.0.2.10")
	if dialer.LocalAddr == nil || dialer.LocalAddr.(*net.TCPAddr).IP.String() != "192.0.2.10" {
		t.Fatalf("expected IPv4 source binding, got %#v", dialer.LocalAddr)
	}
}

func TestUsableAddressesRejectUnspecifiedResults(t *testing.T) {
	if usableIPv4(netip.MustParseAddr("0.0.0.0")) {
		t.Fatal("0.0.0.0 must not be treated as a usable IPv4 endpoint")
	}
	if usableIPv6(netip.MustParseAddr("::")) {
		t.Fatal(":: must not be treated as a usable IPv6 endpoint")
	}
}


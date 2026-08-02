package nodes

import (
	"bufio"
	"context"
	"net"
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
		connection, acceptErr := listener.Accept()
		if acceptErr != nil {
			return
		}
		defer connection.Close()
		reader := bufio.NewReader(connection)
		line, _ := reader.ReadString('\n')
		requestLine <- strings.TrimSpace(line)
		for {
			line, _ = reader.ReadString('\n')
			line = strings.TrimSpace(line)
			if line == "" {
				break
			}
			if strings.HasPrefix(strings.ToLower(line), "proxy-authorization:") {
				authorization <- line
			}
		}
		_, _ = connection.Write([]byte("HTTP/1.1 200 Connection established\r\nContent-Length: 0\r\n\r\n"))
	}()
	address := listener.Addr().(*net.TCPAddr)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	result := ProbeHTTP(ctx, Node{ID: "node-1", Server: address.IP.String(), Port: uint16(address.Port)}, Credentials{Username: "alice", Password: "secret"})
	if !result.Available || result.Status != 200 || result.NodeID != "node-1" {
		t.Fatalf("result = %#v", result)
	}
	if line := <-requestLine; line != "CONNECT 1.1.1.1:443 HTTP/1.1" {
		t.Fatalf("request line = %q", line)
	}
	if line := <-authorization; line != "Proxy-Authorization: Basic YWxpY2U6c2VjcmV0" {
		t.Fatalf("authorization = %q", line)
	}
}

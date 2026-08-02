package core

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

type MockHTTPProxy struct {
	listener    net.Listener
	connections atomic.Int64
	wait        sync.WaitGroup
}

func StartMockHTTPProxy(address string) (*MockHTTPProxy, error) {
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("listen mock HTTP proxy: %w", err)
	}
	proxy := &MockHTTPProxy{listener: listener}
	proxy.wait.Add(1)
	go proxy.serve()
	return proxy, nil
}

func (p *MockHTTPProxy) Connections() int64 {
	return p.connections.Load()
}

func (p *MockHTTPProxy) Close() error {
	err := p.listener.Close()
	p.wait.Wait()
	return err
}

func (p *MockHTTPProxy) serve() {
	defer p.wait.Done()
	for {
		connection, err := p.listener.Accept()
		if err != nil {
			return
		}
		p.connections.Add(1)
		p.wait.Add(1)
		go func() {
			defer p.wait.Done()
			defer connection.Close()
			_ = connection.SetDeadline(time.Now().Add(5 * time.Second))
			request, err := http.ReadRequest(bufio.NewReader(connection))
			if err != nil || request.Method != http.MethodConnect {
				return
			}
			_, _ = connection.Write([]byte("HTTP/1.1 200 Connection Established\r\n\r\n"))
			buffer := make([]byte, 1)
			_, _ = connection.Read(buffer)
		}()
	}
}

package helperipc

import (
	"context"
	"encoding/binary"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"winrouter/internal/core"
)

const testToken = "0123456789abcdef0123456789abcdef0123456789abcdef"

type fakeService struct {
	mu                          sync.Mutex
	validated, applied, stopped int
	status                      core.Status
}

func (s *fakeService) Validate([]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.validated++
	return nil
}
func (s *fakeService) Apply([]byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.applied++
	s.status.State = core.StateRunning
	return nil
}
func (s *fakeService) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stopped++
	s.status.State = core.StateStopped
	return nil
}
func (s *fakeService) Status() core.Status { s.mu.Lock(); defer s.mu.Unlock(); return s.status }

func TestServerClientAllowsOnlyStructuredAuthenticatedMethods(t *testing.T) {
	service := &fakeService{}
	client, closeServer := startTestServer(t, testToken, service)
	defer closeServer()
	var status core.Status
	if err := client.Call(MethodApply, ConfigParams{Config: []byte(`{"route":{}}`)}, &status); err != nil {
		t.Fatal(err)
	}
	if status.State != core.StateRunning || service.applied != 1 {
		t.Fatalf("status=%#v applied=%d", status, service.applied)
	}
	if err := client.Call(MethodHeartbeat, nil, &Heartbeat{}); err != nil {
		t.Fatal(err)
	}
	if err := client.Call("execute", map[string]string{"command": "cmd.exe"}, nil); err == nil {
		t.Fatal("arbitrary method accepted")
	}
	if err := client.Call(MethodStatus, map[string]string{"path": `C:\Windows\System32`}, nil); err == nil {
		t.Fatal("status accepted arbitrary path parameter")
	}
	if err := client.Call(MethodApply, ConfigParams{Config: []byte(`"cmd.exe"`)}, nil); err == nil {
		t.Fatal("non-object config accepted")
	}
}

func TestServerRejectsWrongToken(t *testing.T) {
	service := &fakeService{}
	client, closeServer := startTestServer(t, testToken, service)
	defer closeServer()
	client.AuthToken = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	if err := client.Call(MethodStop, nil, nil); err == nil {
		t.Fatal("wrong token accepted")
	}
	if service.stopped != 0 {
		t.Fatal("unauthorized request reached service")
	}
}

func TestServerShutdownStopsServiceAndSignalsExit(t *testing.T) {
	service := &fakeService{status: core.Status{State: core.StateRunning}}
	server, err := NewServer(testToken, service)
	if err != nil {
		t.Fatal(err)
	}
	response := server.dispatch(Request{Method: MethodShutdown, AuthToken: testToken})
	if !response.OK || response.Error != nil {
		t.Fatalf("shutdown response = %#v", response)
	}
	if service.stopped != 1 || service.Status().State != core.StateStopped {
		t.Fatalf("stopped=%d status=%#v", service.stopped, service.Status())
	}
	select {
	case <-server.ShutdownRequested():
	default:
		t.Fatal("shutdown signal was not emitted")
	}
}

func TestServerRejectsUnauthenticatedShutdown(t *testing.T) {
	service := &fakeService{status: core.Status{State: core.StateRunning}}
	server, err := NewServer(testToken, service)
	if err != nil {
		t.Fatal(err)
	}
	response := server.dispatch(Request{Method: MethodShutdown, AuthToken: "wrong-token"})
	if response.Error == nil || service.stopped != 0 {
		t.Fatalf("response=%#v stopped=%d", response, service.stopped)
	}
	select {
	case <-server.ShutdownRequested():
		t.Fatal("unauthenticated shutdown emitted exit signal")
	default:
	}
}

func TestServerRejectsOversizeFrame(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer clientSide.Close()
	server, err := NewServer(testToken, &fakeService{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { defer close(done); defer serverSide.Close(); server.handle(serverSide) }()
	var header [4]byte
	binary.BigEndian.PutUint32(header[:], MaxMessageSize+1)
	if _, err := clientSide.Write(header[:]); err != nil {
		t.Fatal(err)
	}
	if _, err := readFrame(clientSide); err != nil {
		t.Fatalf("read rejection: %v", err)
	}
	<-done
}

func TestClientCallContextEnforcesDeadline(t *testing.T) {
	serverSide, clientSide := net.Pipe()
	defer serverSide.Close()
	client := Client{AuthToken: testToken, Dial: func() (net.Conn, error) { return clientSide, nil }}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := client.CallContext(ctx, MethodStatus, nil, nil); err == nil {
		t.Fatal("CallContext() did not enforce deadline")
	}
}

func TestNewServerRequiresStrongToken(t *testing.T) {
	if _, err := NewServer("short", &fakeService{}); err == nil {
		t.Fatal("short token accepted")
	}
	if token, err := GenerateToken(); err != nil || len(token) < 32 {
		t.Fatalf("GenerateToken() = %q, %v", token, err)
	}
}

func TestControllerServiceRejectsRawSingBoxAndUnknownFields(t *testing.T) {
	controller, err := core.NewController(core.ControllerOptions{
		StateDirectory: t.TempDir(), Validate: func(context.Context, []byte) error { return nil },
		Launch: func(context.Context, string) (core.Process, error) { return nil, errors.New("must not launch") },
		Health: func(context.Context, core.Process, []byte) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	service := ControllerService{Controller: controller}
	if err := service.Validate([]byte(`{"route":{"final":"direct"}}`)); err == nil {
		t.Fatal("raw sing-box configuration accepted")
	}
	if err := service.Apply([]byte(`{"schema_version":1,"mode":"direct-split","path":"C:\\Windows"}`)); err == nil {
		t.Fatal("arbitrary path field accepted")
	}
}

func TestControllerServiceRejectsFaultTerminationByDefault(t *testing.T) {
	controller, err := core.NewController(core.ControllerOptions{
		StateDirectory: t.TempDir(),
		Validate:       func(context.Context, []byte) error { return nil },
		Launch:         func(context.Context, string) (core.Process, error) { return nil, errors.New("must not launch") },
		Health:         func(context.Context, core.Process, []byte) error { return nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	service := ControllerService{Controller: controller}
	if err := service.FaultTerminateCore(); err == nil {
		t.Fatal("fault termination enabled by default")
	}
}

func startTestServer(t *testing.T, token string, service Service) (Client, func()) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(token, service)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	client := Client{AuthToken: token, Dial: func() (net.Conn, error) { return net.DialTimeout("tcp", listener.Addr().String(), time.Second) }}
	return client, func() {
		_ = listener.Close()
		select {
		case err := <-done:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				t.Errorf("Serve() error: %v", err)
			}
		case <-time.After(time.Second):
			t.Error("server did not stop")
		}
	}
}

func TestNamedPipeACLAndAuthentication(t *testing.T) {
	if !isWindowsTest() {
		t.Skip("Windows named pipe test")
	}
	listener, _, err := ListenCurrentUserPipe()
	if err != nil {
		t.Fatal(err)
	}
	server, err := NewServer(testToken, &fakeService{})
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- server.Serve(listener) }()
	client := Client{AuthToken: testToken, Dial: DefaultDialer(time.Second)}
	var heartbeat Heartbeat
	if err := client.Call(MethodHeartbeat, nil, &heartbeat); err != nil {
		t.Fatal(err)
	}
	if heartbeat.UnixMillis == 0 {
		t.Fatal("empty heartbeat")
	}
	_ = listener.Close()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("named pipe server did not stop")
	}
}

package dnssettings

import (
	"context"
	"net"
	"strconv"
	"testing"
	"time"
)

func TestUDPProbeAcceptsDNSResponse(t *testing.T) {
	listener, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.ParseIP("127.0.0.1")})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	done := make(chan struct{})
	go func() {
		defer close(done)
		request := make([]byte, 512)
		count, remote, readErr := listener.ReadFromUDP(request)
		if readErr != nil {
			return
		}
		response := append([]byte(nil), request[:count]...)
		response[2] |= 0x80
		_, _ = listener.WriteToUDP(response, remote)
	}()
	packet, err := queryPacket(probeName)
	if err != nil {
		t.Fatal(err)
	}
	port := uint16(listener.LocalAddr().(*net.UDPAddr).Port)
	if err := testUDP(context.Background(), Server{Type: "udp", Server: "127.0.0.1", Port: port}, packet); err != nil {
		t.Fatalf("test UDP DNS: %v", err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("DNS server did not complete")
	}
}

func TestValidateResponseRejectsMismatchedTransaction(t *testing.T) {
	query, err := queryPacket(probeName)
	if err != nil {
		t.Fatal(err)
	}
	response := append([]byte(nil), query...)
	response[0]++
	response[2] |= 0x80
	if err := validateResponse(query, response); err == nil {
		t.Fatal("accepted mismatched transaction")
	}
}

func TestEndpoint(t *testing.T) {
	if got := endpoint(Server{Server: "1.1.1.1", Port: 53}); got != "1.1.1.1:"+strconv.Itoa(53) {
		t.Fatalf("endpoint = %q", got)
	}
}

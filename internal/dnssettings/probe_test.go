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

func TestLiveEncryptedDNS(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping live network test in short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// AliDNS DoT
	resAli := Test(ctx, Server{Type: "tls", Server: "223.5.5.5", Port: 853, ServerName: "dns.alidns.com"})
	if !resAli.Success {
		t.Logf("AliDNS DoT live probe: %v (latency %dms)", resAli.Error, resAli.DurationMS)
	} else {
		t.Logf("AliDNS DoT success in %dms", resAli.DurationMS)
	}

	// AliDNS DoH
	resAliH := Test(ctx, Server{Type: "https", Server: "223.5.5.5", Port: 443, ServerName: "dns.alidns.com"})
	if !resAliH.Success {
		t.Logf("AliDNS DoH live probe: %v (latency %dms)", resAliH.Error, resAliH.DurationMS)
	} else {
		t.Logf("AliDNS DoH success in %dms", resAliH.DurationMS)
	}

	// DNSPod DoT
	resTencent := Test(ctx, Server{Type: "tls", Server: "1.12.12.12", Port: 853, ServerName: "dot.pub"})
	if !resTencent.Success {
		t.Logf("DNSPod DoT live probe: %v (latency %dms)", resTencent.Error, resTencent.DurationMS)
	} else {
		t.Logf("DNSPod DoT success in %dms", resTencent.DurationMS)
	}
}

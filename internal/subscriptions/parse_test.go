package subscriptions

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"winrouter/internal/nodes"
)

func TestParseMultiProtocolPlaintextAndBase64(t *testing.T) {
	vmessJSON := `{"v":"2","ps":"VMess-Node","add":"vmess.example.com","port":"443","id":"a8e678c0-8903-4402-8e99-20aadf1a7cd1","net":"ws","path":"/ws","host":"vmess.example.com","tls":"tls","sni":"vmess.example.com","alpn":"h2,http/1.1"}`
	vmessB64 := base64.StdEncoding.EncodeToString([]byte(vmessJSON))

	lines := []string{
		"ss://YWVzLTI1Ni1nY206c2VjcmV0cGFzc3dvcmQ=@ss.example.com:8388#SS-Node",
		"vmess://" + vmessB64,
		"vless://a8e678c0-8903-4402-8e99-20aadf1a7cd1@vless.example.com:443?encryption=none&flow=xtls-rprx-vision&security=tls&sni=vless.example.com#VLESS-Node",
		"trojan://trojanpassword@trojan.example.com:443?security=tls&sni=trojan.example.com&type=ws&path=%2Ftrojan-ws#Trojan%20Node",
	}

	plainContent := strings.Join(lines, "\n")

	// 1. Test plaintext parsing
	plainNodes, err := Parse([]byte(plainContent))
	if err != nil {
		t.Fatalf("Parse plaintext failed: %v", err)
	}
	if len(plainNodes) != 4 {
		t.Fatalf("expected 4 nodes, got %d", len(plainNodes))
	}

	// Verify Shadowsocks node
	if plainNodes[0].Type != nodes.TypeShadowsocks || plainNodes[0].Server != "ss.example.com" || plainNodes[0].Port != 8388 || plainNodes[0].Authentication.Method != "aes-256-gcm" || plainNodes[0].Authentication.Password != "secretpassword" || plainNodes[0].Name != "SS-Node" {
		t.Fatalf("unexpected SS node: %#v", plainNodes[0])
	}

	// Verify VMess node
	if plainNodes[1].Type != nodes.TypeVMess || plainNodes[1].Server != "vmess.example.com" || plainNodes[1].Port != 443 || plainNodes[1].Authentication.UUID != "a8e678c0-8903-4402-8e99-20aadf1a7cd1" || plainNodes[1].TLS == nil || !plainNodes[1].TLS.Enabled || plainNodes[1].Transport == nil || plainNodes[1].Transport.Type != "ws" || plainNodes[1].Name != "VMess-Node" {
		t.Fatalf("unexpected VMess node: %#v", plainNodes[1])
	}

	// Verify VLESS node
	if plainNodes[2].Type != nodes.TypeVLESS || plainNodes[2].Server != "vless.example.com" || plainNodes[2].Port != 443 || plainNodes[2].Authentication.UUID != "a8e678c0-8903-4402-8e99-20aadf1a7cd1" || plainNodes[2].Authentication.Flow != "xtls-rprx-vision" || plainNodes[2].TLS == nil || plainNodes[2].Name != "VLESS-Node" {
		t.Fatalf("unexpected VLESS node: %#v", plainNodes[2])
	}

	// Verify Trojan node
	if plainNodes[3].Type != nodes.TypeTrojan || plainNodes[3].Server != "trojan.example.com" || plainNodes[3].Port != 443 || plainNodes[3].Authentication.Password != "trojanpassword" || plainNodes[3].TLS == nil || plainNodes[3].Transport == nil || plainNodes[3].Transport.Path != "/trojan-ws" || plainNodes[3].Name != "Trojan Node" {
		t.Fatalf("unexpected Trojan node: %#v", plainNodes[3])
	}

	// 2. Test full Base64 encoded subscription
	b64Data := base64.StdEncoding.EncodeToString([]byte(plainContent))
	b64Nodes, err := Parse([]byte(b64Data))
	if err != nil {
		t.Fatalf("Parse Base64 subscription failed: %v", err)
	}
	if len(b64Nodes) != 4 {
		t.Fatalf("expected 4 nodes from Base64, got %d", len(b64Nodes))
	}
}

func TestParseLegacyShadowsocksURI(t *testing.T) {
	legacy := "ss://" + base64.StdEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:mypassword@legacy.example.com:8388")) + "#Legacy-SS"
	res, err := parseShadowsocksURI(legacy)
	if err != nil {
		t.Fatalf("parse legacy SS failed: %v", err)
	}
	if res.Type != nodes.TypeShadowsocks || res.Server != "legacy.example.com" || res.Port != 8388 || res.Authentication.Method != "chacha20-ietf-poly1305" || res.Authentication.Password != "mypassword" {
		t.Fatalf("unexpected legacy SS result: %#v", res)
	}
}

func TestParseRejectsInvalidSchemesAndExceedingLimits(t *testing.T) {
	// Unknown scheme
	if _, err := Parse([]byte("socks5://1.2.3.4:1080\n")); err == nil {
		t.Fatal("expected error for unsupported socks5 scheme")
	}

	// Over 500 nodes
	var manyLines []string
	for i := 0; i < 501; i++ {
		manyLines = append(manyLines, "ss://YWVzLTI1Ni1nY206cGFzcw==@1.2.3.4:8388#Node")
	}
	if _, err := Parse([]byte(strings.Join(manyLines, "\n"))); err == nil {
		t.Fatal("expected error when nodes exceed 500")
	}
}

func TestParseClashYAMLProxyList(t *testing.T) {
	clashData := []byte(`
proxies:
  - name: "Clash-SS"
    type: ss
    server: 198.51.100.1
    port: 8388
    cipher: aes-256-gcm
    password: "sspassword"

  - name: "Clash-VMess"
    type: vmess
    server: 198.51.100.2
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    alterId: 0
    cipher: auto
    tls: true
    servername: vmess.clash.com
    network: ws
    ws-opts:
      path: "/vmess-path"
      headers:
        Host: vmess.clash.com

  - name: "Clash-VLESS"
    type: vless
    server: 198.51.100.3
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    flow: xtls-rprx-vision
    tls: true
    servername: vless.clash.com

  - name: "Clash-Trojan"
    type: trojan
    server: 198.51.100.4
    port: 443
    password: "trojanpass"
    sni: trojan.clash.com
    skip-cert-verify: true

  - name: "Clash-HTTP"
    type: http
    server: 198.51.100.5
    port: 8080
    username: "alice"
    password: "httppassword"
`)

	nodesList, err := Parse(clashData)
	if err != nil {
		t.Fatalf("Parse Clash YAML failed: %v", err)
	}
	if len(nodesList) != 5 {
		t.Fatalf("expected 5 nodes from Clash YAML, got %d", len(nodesList))
	}

	// Verify SS
	if nodesList[0].Name != "Clash-SS" || nodesList[0].Type != nodes.TypeShadowsocks || nodesList[0].Authentication.Method != "aes-256-gcm" || nodesList[0].Authentication.Password != "sspassword" {
		t.Fatalf("unexpected Clash SS: %#v", nodesList[0])
	}

	// Verify VMess
	if nodesList[1].Name != "Clash-VMess" || nodesList[1].Type != nodes.TypeVMess || nodesList[1].TLS == nil || !nodesList[1].TLS.Enabled || nodesList[1].TLS.ServerName != "vmess.clash.com" || nodesList[1].Transport == nil || nodesList[1].Transport.Path != "/vmess-path" {
		t.Fatalf("unexpected Clash VMess: %#v", nodesList[1])
	}

	// Verify VLESS
	if nodesList[2].Name != "Clash-VLESS" || nodesList[2].Type != nodes.TypeVLESS || nodesList[2].Authentication.Flow != "xtls-rprx-vision" || nodesList[2].TLS == nil || nodesList[2].TLS.ServerName != "vless.clash.com" {
		t.Fatalf("unexpected Clash VLESS: %#v", nodesList[2])
	}

	// Verify Trojan
	if nodesList[3].Name != "Clash-Trojan" || nodesList[3].Type != nodes.TypeTrojan || nodesList[3].Authentication.Password != "trojanpass" || nodesList[3].TLS == nil || nodesList[3].TLS.Insecure != true {
		t.Fatalf("unexpected Clash Trojan: %#v", nodesList[3])
	}

	// Verify HTTP
	if nodesList[4].Name != "Clash-HTTP" || nodesList[4].Type != nodes.TypeHTTP || nodesList[4].Authentication.Username != "alice" || nodesList[4].Authentication.Password != "httppassword" {
		t.Fatalf("unexpected Clash HTTP: %#v", nodesList[4])
	}
}

func TestParseClashYAMLRejections(t *testing.T) {
	// Missing UUID in VMess
	badVMess := []byte(`
proxies:
  - name: "Bad-VMess"
    type: vmess
    server: 1.2.3.4
    port: 443
`)
	if _, err := Parse(badVMess); err == nil {
		t.Fatal("expected error for VMess without UUID")
	}

	// Unsupported protocol inside YAML
	unsupportedProto := []byte(`
proxies:
  - name: "Hysteria"
    type: hysteria2
    server: 1.2.3.4
    port: 443
`)
	if _, err := Parse(unsupportedProto); err == nil {
		t.Fatal("expected error for unsupported hysteria2 proxy type")
	}

	// Unsupported reality-opts field
	realityOpts := []byte(`
proxies:
  - name: "Reality-VLESS"
    type: vless
    server: 1.2.3.4
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    reality-opts:
      public-key: "xyz"
`)
	if _, err := Parse(realityOpts); err == nil {
		t.Fatal("expected error for reality-opts in Clash YAML")
	}

	// Unsupported grpc-opts field
	grpcOpts := []byte(`
proxies:
  - name: "GRPC-VMess"
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    grpc-opts:
      grpc-service-name: "service"
`)
	if _, err := Parse(grpcOpts); err == nil {
		t.Fatal("expected error for grpc-opts in Clash YAML")
	}

	// Unsupported client-fingerprint field
	fingerprintYAML := []byte(`
proxies:
  - name: "Fingerprinted-Trojan"
    type: trojan
    server: 1.2.3.4
    port: 443
    password: "pass"
    client-fingerprint: "chrome"
`)
	if _, err := Parse(fingerprintYAML); err == nil {
		t.Fatal("expected error for client-fingerprint in Clash YAML")
	}

	// Unsupported VMess alterId > 0
	badAlterIdYAML := []byte(`
proxies:
  - name: "Legacy-AlterId-VMess"
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    alterId: 64
`)
	if _, err := Parse(badAlterIdYAML); err == nil {
		t.Fatal("expected error for VMess alterId > 0 in Clash YAML")
	}

	// Unsupported VMess alter-id (kebab-case) > 0
	badKebabAlterIdYAML := []byte(`
proxies:
  - name: "Legacy-Kebab-AlterId-VMess"
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    alter-id: 64
`)
	if _, err := Parse(badKebabAlterIdYAML); err == nil {
		t.Fatal("expected error for VMess alter-id (kebab-case) > 0 in Clash YAML")
	}

	// Invalid string alterId
	invalidAlterIdYAML := []byte(`
proxies:
  - name: "Invalid-AlterId-VMess"
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    alterId: invalid
`)
	if _, err := Parse(invalidAlterIdYAML); err == nil {
		t.Fatal("expected error for invalid alterId string in Clash YAML")
	}

	// Valid alter-id: 0 and explicit UDP support
	validKebabAlterIdYAML := []byte(`
proxies:
  - name: "Valid-AlterId-VMess"
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    alter-id: 0
    udp: true
`)
	if nodesList, err := Parse(validKebabAlterIdYAML); err != nil || len(nodesList) != 1 {
		t.Fatalf("expected valid parse for alter-id: 0, got %v", err)
	}

	// Per-node UDP disabling cannot be represented and must not be ignored.
	udpDisabledYAML := []byte(`
proxies:
  - name: "UDP-Disabled"
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: "pass"
    udp: false
`)
	if _, err := Parse(udpDisabledYAML); err == nil {
		t.Fatal("expected error for udp: false in Clash YAML")
	}

	// Full Clash configuration sections must not be silently discarded.
	unsupportedTopLevelYAML := []byte(`
proxies:
  - name: "Node"
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: "pass"
rules:
  - MATCH,DIRECT
`)
	if _, err := Parse(unsupportedTopLevelYAML); err == nil {
		t.Fatal("expected error for unsupported Clash top-level rules field")
	}

	// Unsupported plugin-opts field
	pluginOptsYAML := []byte(`
proxies:
  - name: "Plugin-SS"
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: "pass"
    plugin-opts:
      mode: "websocket"
`)
	if _, err := Parse(pluginOptsYAML); err == nil {
		t.Fatal("expected error for plugin-opts in Clash YAML")
	}

	// Unsupported ws-opts field (e.g. max-early-data)
	wsOptsUnknownFieldYAML := []byte(`
proxies:
  - name: "EarlyData-VMess"
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    network: ws
    ws-opts:
      path: "/ws"
      max-early-data: 2048
`)
	if _, err := Parse(wsOptsUnknownFieldYAML); err == nil {
		t.Fatal("expected error for unknown ws-opts field in Clash YAML")
	}
}

func TestParseURIRejectsUnsupportedSemanticParameters(t *testing.T) {
	// VLESS with reality
	vlessReality := "vless://a8e678c0-8903-4402-8e99-20aadf1a7cd1@1.2.3.4:443?security=reality&sni=test.com#Reality"
	if _, err := parseVLESSURI(vlessReality); err == nil {
		t.Fatal("expected error for VLESS reality")
	}

	// VLESS with grpc transport
	vlessGRPC := "vless://a8e678c0-8903-4402-8e99-20aadf1a7cd1@1.2.3.4:443?type=grpc&serviceName=test#GRPC"
	if _, err := parseVLESSURI(vlessGRPC); err == nil {
		t.Fatal("expected error for VLESS grpc transport")
	}

	// VMess with kcp header type
	vmessKCP := `{"v":"2","ps":"VMess-KCP","add":"1.2.3.4","port":443,"id":"a8e678c0-8903-4402-8e99-20aadf1a7cd1","net":"kcp","type":"none"}`
	vmessKCPURI := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessKCP))
	if _, err := parseVMessURI(vmessKCPURI); err == nil {
		t.Fatal("expected error for VMess kcp network")
	}

	// Shadowsocks with SIP003 plugin
	ssPlugin := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@1.2.3.4:8388?plugin=obfs-local#PluginSS"
	if _, err := parseShadowsocksURI(ssPlugin); err == nil {
		t.Fatal("expected error for Shadowsocks plugin")
	}

	// Shadowsocks with unsupported cipher
	ssBadCipher := "ss://YmFkLWNpcGhlcjpwYXNzd29yZA==@1.2.3.4:8388#BadCipher"
	if _, err := parseShadowsocksURI(ssBadCipher); err == nil {
		t.Fatal("expected error for Shadowsocks bad cipher")
	}
}

func TestParseClashYAMLAliasesAndDepthSecurity(t *testing.T) {
	// Deep nesting (>10 levels)
	deepYAML := []byte(`
a:
 b:
  c:
   d:
    e:
     f:
      g:
       h:
        i:
         j:
          k:
           l:
            proxies:
              - name: test
                type: ss
                server: 1.2.3.4
                port: 8388
                cipher: aes-256-gcm
                password: p
`)
	if _, err := Parse(deepYAML); err == nil {
		t.Fatal("expected error for deep YAML nesting > 10 levels")
	}

	// Alias bomb (>20 aliases)
	var sb strings.Builder
	sb.WriteString("anchor: &anchor\n  cipher: aes-256-gcm\n  password: p\n  type: ss\n  server: 1.2.3.4\n  port: 8388\nproxies:\n")
	for i := 0; i < 25; i++ {
		sb.WriteString(fmt.Sprintf("  - name: node%d\n    <<: *anchor\n", i))
	}
	if _, err := Parse([]byte(sb.String())); err == nil {
		t.Fatal("expected error for YAML exceeding 20 aliases")
	}
}

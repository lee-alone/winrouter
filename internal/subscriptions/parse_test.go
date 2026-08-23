package subscriptions

import (
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"winrouter/internal/nodes"
)

func TestParseMultiProtocolPlaintextAndBase64(t *testing.T) {
	vmessJSON := `{"v":"2","ps":"VMess-Node","add":"vmess.example.com","port":"443","id":"a8e678c0-8903-4402-8e99-20aadf1a7cd1","net":"ws","path":"ws-path","host":"vmess.example.com","tls":"tls","sni":"vmess.example.com","alpn":"h2,http/1.1"}`
	vmessB64 := base64.StdEncoding.EncodeToString([]byte(vmessJSON))

	lines := []string{
		"# Header comment",
		"ss://YWVzLTI1Ni1nY206c2VjcmV0cGFzc3dvcmQ=@ss.example.com:8388#SS-Node",
		"// Another comment",
		"vmess://" + vmessB64,
		"vless://a8e678c0-8903-4402-8e99-20aadf1a7cd1@vless.example.com:443?encryption=none&flow=xtls-rprx-vision&security=tls&sni=vless.example.com#VLESS-Node",
		"trojan://trojanpassword@trojan.example.com:443?security=tls&sni=trojan.example.com&type=ws&path=trojan-ws#Trojan%20Node",
		"https://alice:secret@http.example.com:8443#HTTP-Node",
		"unsupported-scheme://example.com:1234",
	}

	plainContent := strings.Join(lines, "\n")

	// 1. Test plaintext parsing with comments and unsupported lines skipped
	plainNodes, err := Parse([]byte(plainContent))
	if err != nil {
		t.Fatalf("Parse plaintext failed: %v", err)
	}
	if len(plainNodes) != 5 {
		t.Fatalf("expected 5 valid nodes, got %d", len(plainNodes))
	}

	// Verify Shadowsocks node
	if plainNodes[0].Type != nodes.TypeShadowsocks || plainNodes[0].Server != "ss.example.com" || plainNodes[0].Port != 8388 || plainNodes[0].Authentication.Method != "aes-256-gcm" || plainNodes[0].Authentication.Password != "secretpassword" || plainNodes[0].Name != "SS-Node" {
		t.Fatalf("unexpected SS node: %#v", plainNodes[0])
	}

	// Verify VMess node with normalized path
	if plainNodes[1].Type != nodes.TypeVMess || plainNodes[1].Server != "vmess.example.com" || plainNodes[1].Port != 443 || plainNodes[1].Authentication.UUID != "a8e678c0-8903-4402-8e99-20aadf1a7cd1" || plainNodes[1].TLS == nil || !plainNodes[1].TLS.Enabled || plainNodes[1].Transport == nil || plainNodes[1].Transport.Path != "/ws-path" || plainNodes[1].Name != "VMess-Node" {
		t.Fatalf("unexpected VMess node: %#v", plainNodes[1])
	}

	// Verify VLESS node
	if plainNodes[2].Type != nodes.TypeVLESS || plainNodes[2].Server != "vless.example.com" || plainNodes[2].Port != 443 || plainNodes[2].Authentication.UUID != "a8e678c0-8903-4402-8e99-20aadf1a7cd1" || plainNodes[2].Authentication.Flow != "xtls-rprx-vision" || plainNodes[2].TLS == nil || plainNodes[2].Name != "VLESS-Node" {
		t.Fatalf("unexpected VLESS node: %#v", plainNodes[2])
	}

	// Verify Trojan node with normalized path
	if plainNodes[3].Type != nodes.TypeTrojan || plainNodes[3].Server != "trojan.example.com" || plainNodes[3].Port != 443 || plainNodes[3].Authentication.Password != "trojanpassword" || plainNodes[3].TLS == nil || plainNodes[3].Transport == nil || plainNodes[3].Transport.Path != "/trojan-ws" || plainNodes[3].Name != "Trojan Node" {
		t.Fatalf("unexpected Trojan node: %#v", plainNodes[3])
	}

	// Verify HTTP node
	if plainNodes[4].Type != nodes.TypeHTTP || plainNodes[4].Server != "http.example.com" || plainNodes[4].Port != 8443 || plainNodes[4].Username != "alice" || plainNodes[4].Password != "secret" || plainNodes[4].TLS == nil || !plainNodes[4].TLS.Enabled {
		t.Fatalf("unexpected HTTP node: %#v", plainNodes[4])
	}

	// 2. Test Base64 encoded subscription with internal newlines and missing padding
	b64Data := base64.RawStdEncoding.EncodeToString([]byte(plainContent))
	b64WithNewlines := b64Data[:len(b64Data)/2] + "\r\n" + b64Data[len(b64Data)/2:]
	b64Nodes, err := Parse([]byte(b64WithNewlines))
	if err != nil {
		t.Fatalf("Parse Base64 subscription with newlines failed: %v", err)
	}
	if len(b64Nodes) != 5 {
		t.Fatalf("expected 5 nodes from Base64, got %d", len(b64Nodes))
	}
}

func TestParseLegacyAnd2022ShadowsocksURI(t *testing.T) {
	legacy := "ss://" + base64.StdEncoding.EncodeToString([]byte("chacha20-ietf-poly1305:mypassword@legacy.example.com:8388")) + "#Legacy-SS"
	res, err := parseShadowsocksURI(legacy)
	if err != nil {
		t.Fatalf("parse legacy SS failed: %v", err)
	}
	if res.Type != nodes.TypeShadowsocks || res.Server != "legacy.example.com" || res.Port != 8388 || res.Authentication.Method != "chacha20-ietf-poly1305" || res.Authentication.Password != "mypassword" {
		t.Fatalf("unexpected legacy SS result: %#v", res)
	}

	ss2022 := "ss://" + base64.StdEncoding.EncodeToString([]byte("2022-blake3-aes-128-gcm:mysecretpassword")) + "@ss2022.example.com:8388#SS2022"
	res2022, err := parseShadowsocksURI(ss2022)
	if err != nil {
		t.Fatalf("parse ss2022 failed: %v", err)
	}
	if res2022.Authentication.Method != "2022-blake3-aes-128-gcm" {
		t.Fatalf("unexpected ss2022 method: %s", res2022.Authentication.Method)
	}
}

func TestParseRealWorldClashYAMLSubscription(t *testing.T) {
	clashData := []byte(`
port: 7890
socks-port: 7891
mixed-port: 7890
allow-lan: false
mode: rule
log-level: info
ipv6: false

dns:
  enable: true
  listen: 0.0.0.0:53
  nameserver:
    - 223.5.5.5
    - 119.29.29.29

proxies:
  - name: "HK 01"
    type: ss
    server: hk.example.com
    port: 8388
    cipher: aes-256-gcm
    password: "sspassword"
    udp: false

  - name: "JP 02"
    type: vmess
    server: jp.example.com
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    alterId: 0
    cipher: auto
    tls: true
    servername: jp.example.com
    network: ws
    ws-opts:
      path: "/vmess-path"
      headers:
        Host: jp.example.com

  - name: "US 03"
    type: vless
    server: us.example.com
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    flow: xtls-rprx-vision
    tls: true
    servername: us.example.com
    reality-opts:
      public-key: "something"

  - name: "SG 04"
    type: trojan
    server: sg.example.com
    port: 443
    password: "trojanpass"
    sni: sg.example.com
    skip-cert-verify: true

  - name: "Unsupported-Node"
    type: tuic
    server: tuic.example.com
    port: 8443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"

proxy-groups:
  - name: PROXY
    type: select
    proxies:
      - "HK 01"
      - "JP 02"

rules:
  - MATCH,DIRECT
`)

	nodesList, err := Parse(clashData)
	if err != nil {
		t.Fatalf("Parse real-world Clash YAML failed: %v", err)
	}
	// HK 01 (udp: false), US 03 (reality-opts), Unsupported-Node (tuic) must be rejected/skipped
	// JP 02 and SG 04 are valid and must be imported
	if len(nodesList) != 2 {
		t.Fatalf("expected 2 valid nodes from Clash YAML (skipping invalid/unsupported), got %d", len(nodesList))
	}

	if nodesList[0].Name != "JP 02" || nodesList[0].Type != nodes.TypeVMess {
		t.Fatalf("unexpected first node: %#v", nodesList[0])
	}
	if nodesList[1].Name != "SG 04" || nodesList[1].Type != nodes.TypeTrojan {
		t.Fatalf("unexpected second node: %#v", nodesList[1])
	}

	// Also test Base64-encoded Clash YAML
	b64Clash := base64.StdEncoding.EncodeToString(clashData)
	b64Nodes, err := Parse([]byte(b64Clash))
	if err != nil {
		t.Fatalf("Parse Base64-encoded Clash YAML failed: %v", err)
	}
	if len(b64Nodes) != 2 {
		t.Fatalf("expected 2 nodes from Base64 Clash YAML, got %d", len(b64Nodes))
	}
}

func TestParseURIRejectsUnsupportedSemanticParameters(t *testing.T) {
	// VLESS with reality must fail
	vlessReality := "vless://a8e678c0-8903-4402-8e99-20aadf1a7cd1@1.2.3.4:443?security=reality&sni=test.com#Reality"
	if _, err := parseVLESSURI(vlessReality); err == nil {
		t.Fatal("expected error for VLESS reality")
	}

	// VLESS with grpc transport must fail
	vlessGRPC := "vless://a8e678c0-8903-4402-8e99-20aadf1a7cd1@1.2.3.4:443?type=grpc&serviceName=test#GRPC"
	if _, err := parseVLESSURI(vlessGRPC); err == nil {
		t.Fatal("expected error for VLESS grpc transport")
	}

	// VMess with kcp network must fail
	vmessKCP := `{"v":"2","ps":"VMess-KCP","add":"1.2.3.4","port":443,"id":"a8e678c0-8903-4402-8e99-20aadf1a7cd1","net":"kcp","type":"none"}`
	vmessKCPURI := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessKCP))
	if _, err := parseVMessURI(vmessKCPURI); err == nil {
		t.Fatal("expected error for VMess kcp network")
	}

	// VMess with alterId 64 must fail
	vmessAlterID := `{"v":"2","ps":"VMess-Aid","add":"1.2.3.4","port":443,"id":"a8e678c0-8903-4402-8e99-20aadf1a7cd1","aid":64,"net":"ws"}`
	vmessAlterIDURI := "vmess://" + base64.StdEncoding.EncodeToString([]byte(vmessAlterID))
	if _, err := parseVMessURI(vmessAlterIDURI); err == nil {
		t.Fatal("expected error for VMess non-zero alterId")
	}

	// Shadowsocks with plugin must fail
	ssPlugin := "ss://YWVzLTI1Ni1nY206cGFzc3dvcmQ=@1.2.3.4:8388?plugin=obfs-local#PluginSS"
	if _, err := parseShadowsocksURI(ssPlugin); err == nil {
		t.Fatal("expected error for Shadowsocks plugin")
	}

	// Trojan with non-TLS security must fail
	trojanNoTLS := "trojan://password@1.2.3.4:443?security=none#NoTLS"
	if _, err := parseTrojanURI(trojanNoTLS); err == nil {
		t.Fatal("expected error for Trojan security=none")
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

func TestParseClashYAMLRejectsUnknownAndUnmodeledFieldsPerNode(t *testing.T) {
	yamlData := []byte(`
proxies:
  - name: "Node-Fingerprint"
    type: vmess
    server: 1.2.3.4
    port: 443
    uuid: "a8e678c0-8903-4402-8e99-20aadf1a7cd1"
    tls: true
    client-fingerprint: chrome

  - name: "Node-SMux"
    type: ss
    server: 1.2.3.4
    port: 8388
    cipher: aes-256-gcm
    password: pass
    smux:
      enabled: true

  - name: "Node-TFO"
    type: trojan
    server: 1.2.3.4
    port: 443
    password: pass
    tfo: true

  - name: "Node-Valid"
    type: trojan
    server: 1.2.3.4
    port: 443
    password: pass
    sni: test.com
`)

	nodesList, err := Parse(yamlData)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}
	if len(nodesList) != 1 {
		t.Fatalf("expected exactly 1 valid node (Node-Valid), got %d: %#v", len(nodesList), nodesList)
	}
	if nodesList[0].Name != "Node-Valid" {
		t.Fatalf("expected Node-Valid, got %s", nodesList[0].Name)
	}
}

func TestParseHTTPProxyURIStrictPortValidation(t *testing.T) {
	// Valid ports
	for _, uri := range []string{
		"http://proxy.example.com:8080#HTTP",
		"https://proxy.example.com:8443#HTTPS",
		"http://proxy.example.com#Default80",
		"https://proxy.example.com#Default443",
	} {
		if _, err := parseHTTPNodeURI(uri); err != nil {
			t.Fatalf("expected valid HTTP URI %q, got: %v", uri, err)
		}
	}

	// Invalid ports must fail
	for _, uri := range []string{
		"http://proxy.example.com:0#ZeroPort",
		"http://proxy.example.com:70000#OverflowPort",
		"https://proxy.example.com:abc#NonNumericPort",
		"http://proxy.example.com:-1#NegativePort",
	} {
		if _, err := parseHTTPNodeURI(uri); err == nil {
			t.Fatalf("expected error for invalid port in URI %q", uri)
		}
	}
}

func TestParseAlterIDStrictValidation(t *testing.T) {
	// Valid alterId 0
	if aid, err := parseAlterID(0); err != nil || aid != 0 {
		t.Fatalf("expected 0, got %d, err: %v", aid, err)
	}
	if aid, err := parseAlterID(float64(0)); err != nil || aid != 0 {
		t.Fatalf("expected 0, got %d, err: %v", aid, err)
	}
	if aid, err := parseAlterID("0"); err != nil || aid != 0 {
		t.Fatalf("expected 0, got %d, err: %v", aid, err)
	}

	// Invalid fractional float64 values must fail
	if _, err := parseAlterID(0.5); err == nil {
		t.Fatal("expected error for float64(0.5)")
	}
	if _, err := parseAlterID(0.1); err == nil {
		t.Fatal("expected error for float64(0.1)")
	}
	if _, err := parseAlterID(float64(64.9)); err == nil {
		t.Fatal("expected error for float64(64.9)")
	}

	// Out of range or invalid types
	if _, err := parseAlterID(-1); err == nil {
		t.Fatal("expected error for -1")
	}
	if _, err := parseAlterID(70000); err == nil {
		t.Fatal("expected error for 70000")
	}
	if _, err := parseAlterID("abc"); err == nil {
		t.Fatal("expected error for 'abc'")
	}
}

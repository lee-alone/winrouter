package subscriptions

import (
	"errors"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
	"winrouter/internal/nodes"
)

type clashConfig struct {
	Proxies []clashProxy `yaml:"proxies"`
}

type clashProxy struct {
	Name           string            `yaml:"name"`
	Type           string            `yaml:"type"`
	Server         string            `yaml:"server"`
	Port           any               `yaml:"port"`
	Username       string            `yaml:"username"`
	Password       string            `yaml:"password"`
	Cipher         string            `yaml:"cipher"`
	Method         string            `yaml:"method"`
	UUID           string            `yaml:"uuid"`
	AlterID        any               `yaml:"alterId"`
	AlterIDKebab   any               `yaml:"alter-id"`
	Flow           string            `yaml:"flow"`
	TLS            bool              `yaml:"tls"`
	ServerName     string            `yaml:"servername"`
	SNI            string            `yaml:"sni"`
	SkipCertVerify bool              `yaml:"skip-cert-verify"`
	ALPN           []string          `yaml:"alpn"`
	Network        string            `yaml:"network"`
	WSOpts         *clashWSOpts      `yaml:"ws-opts"`
	WSPath         string            `yaml:"ws-path"`
	WSHeaders      map[string]string `yaml:"ws-headers"`
	Plugin         string            `yaml:"plugin"`
	PluginOpts     any               `yaml:"plugin-opts"`
	UDP            any               `yaml:"udp"`
	RealityOpts    any               `yaml:"reality-opts"`
	GRPCOpts       any               `yaml:"grpc-opts"`
	H2Opts         any               `yaml:"h2-opts"`
	HTTPOpts       any               `yaml:"http-opts"`
}

type clashWSOpts struct {
	Path    string            `yaml:"path"`
	Headers map[string]string `yaml:"headers"`
}

const (
	maxYAMLDepth   = 10
	maxYAMLNodes   = 5000
	maxYAMLAliases = 20
)

func validateYAMLNodeTree(node *yaml.Node, currentDepth int, nodeCount, aliasCount *int) error {
	if node == nil {
		return nil
	}
	*nodeCount++
	if *nodeCount > maxYAMLNodes {
		return fmt.Errorf("YAML node count exceeds safety limit of %d", maxYAMLNodes)
	}
	if currentDepth > maxYAMLDepth {
		return fmt.Errorf("YAML structure exceeds maximum depth limit of %d", maxYAMLDepth)
	}
	if node.Kind == yaml.AliasNode {
		*aliasCount++
		if *aliasCount > maxYAMLAliases {
			return fmt.Errorf("YAML alias count exceeds safety limit of %d", maxYAMLAliases)
		}
	}
	for _, child := range node.Content {
		if err := validateYAMLNodeTree(child, currentDepth+1, nodeCount, aliasCount); err != nil {
			return err
		}
	}
	return nil
}

var allowedClashProxyKeys = map[string]bool{
	"name":             true,
	"type":             true,
	"server":           true,
	"port":             true,
	"username":         true,
	"password":         true,
	"cipher":           true,
	"method":           true,
	"uuid":             true,
	"alterid":          true,
	"alter-id":         true,
	"alter_id":         true,
	"flow":             true,
	"tls":              true,
	"servername":       true,
	"sni":              true,
	"skip-cert-verify": true,
	"alpn":             true,
	"network":          true,
	"ws-opts":          true,
	"ws-path":          true,
	"ws-headers":       true,
	"udp":              true,
}

func findProxiesSeq(root *yaml.Node) (*yaml.Node, error) {
	if root == nil {
		return nil, errors.New("nil YAML root")
	}
	var docMap *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		docMap = root.Content[0]
	} else if root.Kind == yaml.MappingNode {
		docMap = root
	}
	if docMap == nil || docMap.Kind != yaml.MappingNode {
		return nil, errors.New("root of Clash YAML must be a mapping")
	}

	for i := 0; i < len(docMap.Content); i += 2 {
		keyNode := docMap.Content[i]
		key := strings.ToLower(strings.TrimSpace(keyNode.Value))
		if key == "proxies" {
			proxiesSeq := docMap.Content[i+1]
			for proxiesSeq.Kind == yaml.AliasNode {
				proxiesSeq = proxiesSeq.Alias
			}
			if proxiesSeq.Kind != yaml.SequenceNode {
				return nil, errors.New("proxies section must be a list")
			}
			return proxiesSeq, nil
		}
	}
	return nil, errors.New("YAML contains no proxies section")
}

func validateProxyNodeAST(proxyNode *yaml.Node) error {
	actual := proxyNode
	for actual.Kind == yaml.AliasNode {
		actual = actual.Alias
	}
	if actual.Kind != yaml.MappingNode {
		return errors.New("proxy entry must be a YAML mapping")
	}
	for j := 0; j < len(actual.Content); j += 2 {
		kNode := actual.Content[j]
		vNode := actual.Content[j+1]
		key := strings.ToLower(strings.TrimSpace(kNode.Value))
		if !allowedClashProxyKeys[key] {
			return fmt.Errorf("unsupported or unknown proxy field %q", kNode.Value)
		}
		if key == "ws-opts" && vNode != nil {
			wsActual := vNode
			for wsActual.Kind == yaml.AliasNode {
				wsActual = wsActual.Alias
			}
			if wsActual.Kind == yaml.MappingNode {
				for w := 0; w < len(wsActual.Content); w += 2 {
					wKey := strings.ToLower(strings.TrimSpace(wsActual.Content[w].Value))
					if wKey != "path" && wKey != "headers" {
						return fmt.Errorf("unsupported ws-opts field %q (only path and headers are supported)", wsActual.Content[w].Value)
					}
				}
			}
		}
	}
	return nil
}

func parseClashYAML(data []byte) ([]nodes.Input, error) {
	if len(data) > 10*1024*1024 {
		return nil, errors.New("YAML configuration exceeds 10MB limit")
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("invalid YAML syntax: %w", err)
	}

	nodeCount := 0
	aliasCount := 0
	if err := validateYAMLNodeTree(&root, 1, &nodeCount, &aliasCount); err != nil {
		return nil, fmt.Errorf("security check failed: %w", err)
	}

	proxiesSeq, err := findProxiesSeq(&root)
	if err != nil {
		return nil, err
	}
	if len(proxiesSeq.Content) == 0 {
		return nil, errors.New("YAML contains no proxies")
	}
	if len(proxiesSeq.Content) > 500 {
		return nil, errors.New("subscription must contain 1 to 500 nodes")
	}

	result := make([]nodes.Input, 0, len(proxiesSeq.Content))
	var lastErr error
	for idx, proxyNode := range proxiesSeq.Content {
		if err := validateProxyNodeAST(proxyNode); err != nil {
			lastErr = fmt.Errorf("proxy node %d: %w", idx+1, err)
			continue
		}
		var p clashProxy
		if err := proxyNode.Decode(&p); err != nil {
			lastErr = fmt.Errorf("proxy node %d: %w", idx+1, err)
			continue
		}
		item, err := mapClashProxy(p)
		if err != nil {
			lastErr = fmt.Errorf("proxy node %d (%q): %w", idx+1, p.Name, err)
			continue
		}
		result = append(result, item)
	}
	if len(result) == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("all Clash proxies failed to parse: %w", lastErr)
		}
		return nil, errors.New("no valid proxies found in Clash YAML")
	}
	return result, nil
}

func mapClashProxy(p clashProxy) (nodes.Input, error) {
	protoType := strings.ToLower(strings.TrimSpace(p.Type))
	server := strings.TrimSpace(p.Server)
	if server == "" {
		return nodes.Input{}, errors.New("missing server address")
	}
	portNum, err := parsePort(p.Port)
	if err != nil || portNum == 0 {
		return nodes.Input{}, errors.New("missing or invalid port")
	}

	name := normalizeName(p.Name, "Clash Node")

	var tlsInput *nodes.TLSInput
	sni := strings.TrimSpace(p.SNI)
	if sni == "" {
		sni = strings.TrimSpace(p.ServerName)
	}
	if p.TLS || protoType == "trojan" {
		if sni == "" {
			sni = server
		}
		var cleanALPN []string
		for _, a := range p.ALPN {
			trimmed := strings.TrimSpace(a)
			if trimmed != "" && !strings.EqualFold(trimmed, "default") {
				cleanALPN = append(cleanALPN, trimmed)
			}
		}
		tlsInput = &nodes.TLSInput{
			Enabled:    true,
			ServerName: sni,
			Insecure:   p.SkipCertVerify,
			ALPN:       cleanALPN,
		}
	}

	if strings.TrimSpace(p.Plugin) != "" || p.PluginOpts != nil {
		return nodes.Input{}, errors.New("plugins (plugin / plugin-opts) are not supported in Clash YAML")
	}

	if p.UDP != nil {
		udpStr := strings.ToLower(fmt.Sprintf("%v", p.UDP))
		if udpStr == "false" || udpStr == "0" {
			return nodes.Input{}, errors.New("udp=false is not supported because per-node UDP disabling cannot be represented")
		}
	}

	if p.RealityOpts != nil {
		return nodes.Input{}, errors.New("reality-opts is not supported; cannot import as plain TLS")
	}
	if p.GRPCOpts != nil {
		return nodes.Input{}, errors.New("grpc-opts is not supported in Clash YAML")
	}
	if p.H2Opts != nil {
		return nodes.Input{}, errors.New("h2-opts is not supported in Clash YAML")
	}
	if p.HTTPOpts != nil {
		return nodes.Input{}, errors.New("http-opts is not supported in Clash YAML")
	}

	var transportInput *nodes.TransportInput
	network := strings.ToLower(strings.TrimSpace(p.Network))
	switch network {
	case "ws", "websocket":
		var path, host string
		if p.WSOpts != nil {
			path = p.WSOpts.Path
			if p.WSOpts.Headers != nil {
				host = p.WSOpts.Headers["Host"]
				if host == "" {
					host = p.WSOpts.Headers["host"]
				}
			}
		}
		if path == "" && p.WSPath != "" {
			path = p.WSPath
		}
		if host == "" && p.WSHeaders != nil {
			host = p.WSHeaders["Host"]
			if host == "" {
				host = p.WSHeaders["host"]
			}
		}
		transportInput = &nodes.TransportInput{
			Type: "ws",
			Path: normalizePath(path),
			Host: strings.TrimSpace(host),
		}
	case "tcp", "":
		if p.WSOpts != nil || p.WSPath != "" || p.WSHeaders != nil {
			var path, host string
			if p.WSOpts != nil {
				path = p.WSOpts.Path
				if p.WSOpts.Headers != nil {
					host = p.WSOpts.Headers["Host"]
					if host == "" {
						host = p.WSOpts.Headers["host"]
					}
				}
			}
			if path == "" && p.WSPath != "" {
				path = p.WSPath
			}
			if host == "" && p.WSHeaders != nil {
				host = p.WSHeaders["Host"]
				if host == "" {
					host = p.WSHeaders["host"]
				}
			}
			if path != "" || host != "" {
				transportInput = &nodes.TransportInput{
					Type: "ws",
					Path: normalizePath(path),
					Host: strings.TrimSpace(host),
				}
			}
		}
	default:
		return nodes.Input{}, fmt.Errorf("unsupported network type %q in Clash YAML (only tcp and ws are supported)", p.Network)
	}

	switch protoType {
	case "ss", "shadowsocks":
		method := strings.ToLower(strings.TrimSpace(p.Cipher))
		if method == "" {
			method = strings.ToLower(strings.TrimSpace(p.Method))
		}
		if !isSupportedSSCipher(method) {
			return nodes.Input{}, fmt.Errorf("unsupported Shadowsocks method/cipher %q in Clash YAML", method)
		}
		if p.Password == "" {
			return nodes.Input{}, errors.New("Shadowsocks password is required")
		}
		return nodes.Input{
			Name:     name,
			Type:     nodes.TypeShadowsocks,
			Server:   server,
			Port:     portNum,
			Egress:   nodes.EgressB,
			Username: method,
			Password: p.Password,
			Authentication: nodes.AuthenticationInput{
				Method:   method,
				Password: p.Password,
			},
		}, nil

	case "http":
		return nodes.Input{
			Name:     name,
			Type:     nodes.TypeHTTP,
			Server:   server,
			Port:     portNum,
			Egress:   nodes.EgressB,
			Username: p.Username,
			Password: p.Password,
			Authentication: nodes.AuthenticationInput{
				Username: p.Username,
				Password: p.Password,
			},
			TLS: tlsInput,
		}, nil

	case "vmess":
		uuid := strings.TrimSpace(p.UUID)
		if uuid == "" {
			return nodes.Input{}, errors.New("VMess UUID is required")
		}
		if p.Cipher != "" {
			c := strings.ToLower(strings.TrimSpace(p.Cipher))
			if c != "auto" && c != "none" && c != "zero" && c != "aes-128-gcm" && c != "chacha20-poly1305" {
				return nodes.Input{}, fmt.Errorf("unsupported VMess cipher %q in Clash YAML", p.Cipher)
			}
		}
		if p.AlterID != nil {
			aid, err := parseAlterID(p.AlterID)
			if err != nil || aid != 0 {
				return nodes.Input{}, fmt.Errorf("unsupported VMess alterId %v (only alterId=0 / AEAD is supported)", p.AlterID)
			}
		}
		if p.AlterIDKebab != nil {
			aid, err := parseAlterID(p.AlterIDKebab)
			if err != nil || aid != 0 {
				return nodes.Input{}, fmt.Errorf("unsupported VMess alter-id %v (only alterId=0 / AEAD is supported)", p.AlterIDKebab)
			}
		}
		return nodes.Input{
			Name:   name,
			Type:   nodes.TypeVMess,
			Server: server,
			Port:   portNum,
			Egress: nodes.EgressB,
			Authentication: nodes.AuthenticationInput{
				UUID: uuid,
			},
			TLS:       tlsInput,
			Transport: transportInput,
		}, nil

	case "vless":
		uuid := strings.TrimSpace(p.UUID)
		if uuid == "" {
			return nodes.Input{}, errors.New("VLESS UUID is required")
		}
		flow := strings.TrimSpace(p.Flow)
		if flow != "" && flow != "xtls-rprx-vision" {
			return nodes.Input{}, fmt.Errorf("unsupported VLESS flow %q in Clash YAML", flow)
		}
		return nodes.Input{
			Name:   name,
			Type:   nodes.TypeVLESS,
			Server: server,
			Port:   portNum,
			Egress: nodes.EgressB,
			Authentication: nodes.AuthenticationInput{
				UUID: uuid,
				Flow: flow,
			},
			TLS:       tlsInput,
			Transport: transportInput,
		}, nil

	case "trojan":
		password := strings.TrimSpace(p.Password)
		if password == "" {
			return nodes.Input{}, errors.New("Trojan password is required")
		}
		return nodes.Input{
			Name:     name,
			Type:     nodes.TypeTrojan,
			Server:   server,
			Port:     portNum,
			Egress:   nodes.EgressB,
			Password: password,
			Authentication: nodes.AuthenticationInput{
				Password: password,
			},
			TLS:       tlsInput,
			Transport: transportInput,
		}, nil

	default:
		return nodes.Input{}, fmt.Errorf("unsupported proxy type %q in Clash YAML", p.Type)
	}
}

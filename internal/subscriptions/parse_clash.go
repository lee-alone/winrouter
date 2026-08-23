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
	"plugin":           true,
	"plugin-opts":      true,
	"udp":              true,
}

func validateClashProxiesAST(root *yaml.Node) error {
	if root == nil {
		return nil
	}
	var docMap *yaml.Node
	if root.Kind == yaml.DocumentNode && len(root.Content) > 0 {
		docMap = root.Content[0]
	} else if root.Kind == yaml.MappingNode {
		docMap = root
	}
	if docMap == nil || docMap.Kind != yaml.MappingNode {
		return errors.New("root of Clash YAML must be a mapping")
	}

	var proxiesSeq *yaml.Node
	for i := 0; i < len(docMap.Content); i += 2 {
		keyNode := docMap.Content[i]
		key := strings.ToLower(strings.TrimSpace(keyNode.Value))
		if key == "proxies" {
			proxiesSeq = docMap.Content[i+1]
			continue
		}
		return fmt.Errorf("unsupported Clash top-level field %q; only proxies can be imported", keyNode.Value)
	}
	if proxiesSeq == nil {
		return errors.New("YAML contains no proxies section")
	}
	for proxiesSeq.Kind == yaml.AliasNode {
		proxiesSeq = proxiesSeq.Alias
	}
	if proxiesSeq.Kind != yaml.SequenceNode {
		return errors.New("proxies section must be a list")
	}

	for idx, proxyNode := range proxiesSeq.Content {
		actual := proxyNode
		for actual.Kind == yaml.AliasNode {
			actual = actual.Alias
		}
		if actual.Kind != yaml.MappingNode {
			return fmt.Errorf("proxy node %d: entry must be a YAML mapping", idx+1)
		}
		for j := 0; j < len(actual.Content); j += 2 {
			kNode := actual.Content[j]
			vNode := actual.Content[j+1]
			key := strings.ToLower(strings.TrimSpace(kNode.Value))
			if !allowedClashProxyKeys[key] {
				return fmt.Errorf("proxy node %d: unsupported or unknown field %q", idx+1, kNode.Value)
			}
			if key == "alterid" || key == "alter-id" || key == "alter_id" {
				if vNode != nil && strings.TrimSpace(vNode.Value) != "0" {
					return fmt.Errorf("proxy node %d: unsupported alterId %q (only alterId=0 / AEAD is supported)", idx+1, vNode.Value)
				}
			}
			if key == "plugin" || key == "plugin-opts" || key == "plugin_opts" {
				if vNode != nil && (strings.TrimSpace(vNode.Value) != "" || len(vNode.Content) > 0) {
					return fmt.Errorf("proxy node %d: plugins (%s) are not supported in Clash YAML", idx+1, kNode.Value)
				}
			}
			if key == "udp" && vNode != nil {
				udpVal := strings.ToLower(strings.TrimSpace(vNode.Value))
				if udpVal == "false" || udpVal == "0" {
					return fmt.Errorf("proxy node %d: udp=false is not supported because per-node UDP disabling cannot be represented", idx+1)
				}
				if udpVal != "true" && udpVal != "1" {
					return fmt.Errorf("proxy node %d: invalid udp boolean value %q", idx+1, vNode.Value)
				}
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
							return fmt.Errorf("proxy node %d: unsupported ws-opts field %q (only path and headers are supported)", idx+1, wsActual.Content[w].Value)
						}
					}
				}
			}
		}
	}
	return nil
}

func parseClashYAML(data []byte) ([]nodes.Input, error) {
	if len(data) > 1024*1024 {
		return nil, errors.New("YAML configuration exceeds 1MB limit")
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

	if err := validateClashProxiesAST(&root); err != nil {
		return nil, err
	}

	var config clashConfig
	if err := root.Decode(&config); err != nil {
		return nil, fmt.Errorf("invalid Clash YAML structure: %w", err)
	}

	if len(config.Proxies) == 0 {
		return nil, errors.New("YAML contains no proxies")
	}
	if len(config.Proxies) > 500 {
		return nil, errors.New("subscription must contain 1 to 500 nodes")
	}

	result := make([]nodes.Input, 0, len(config.Proxies))
	for index, p := range config.Proxies {
		item, err := mapClashProxy(p)
		if err != nil {
			return nil, fmt.Errorf("proxy node %d (%q): %w", index+1, p.Name, err)
		}
		result = append(result, item)
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
			Path: strings.TrimSpace(path),
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
			transportInput = &nodes.TransportInput{
				Type: "ws",
				Path: strings.TrimSpace(path),
				Host: strings.TrimSpace(host),
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
		if p.AlterID != nil {
			alterIDNum, err := parsePort(p.AlterID)
			if err == nil && alterIDNum != 0 {
				return nodes.Input{}, fmt.Errorf("unsupported VMess alterId %v (only alterId=0 / AEAD is supported)", p.AlterID)
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

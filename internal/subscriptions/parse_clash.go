package subscriptions

import (
	"errors"
	"fmt"
	"strings"

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

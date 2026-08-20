package subscriptions

import (
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"winrouter/internal/nodes"
)

type vmessJSON struct {
	V    any `json:"v"`
	PS   string `json:"ps"`
	Add  string `json:"add"`
	Port any    `json:"port"`
	ID   string `json:"id"`
	Aid  any    `json:"aid"`
	Scy  string `json:"scy"`
	Net  string `json:"net"`
	Type string `json:"type"`
	Host string `json:"host"`
	Path string `json:"path"`
	TLS  string `json:"tls"`
	SNI  string `json:"sni"`
	ALPN string `json:"alpn"`
}

func parseVMessURI(value string) (nodes.Input, error) {
	if !strings.HasPrefix(strings.ToLower(value), "vmess://") {
		return nodes.Input{}, errors.New("unsupported VMess protocol scheme")
	}
	payload := strings.TrimPrefix(value, "vmess://")
	payload = strings.TrimPrefix(payload, "VMESS://")
	if idx := strings.Index(payload, "#"); idx != -1 {
		payload = payload[:idx]
	}
	decoded, err := decodeBase64(payload)
	if err != nil {
		return nodes.Input{}, fmt.Errorf("invalid VMess base64: %w", err)
	}

	var data vmessJSON
	if err := json.Unmarshal(decoded, &data); err != nil {
		return nodes.Input{}, fmt.Errorf("invalid VMess JSON: %w", err)
	}

	server := strings.TrimSpace(data.Add)
	if server == "" {
		return nodes.Input{}, errors.New("VMess server address is required")
	}

	uuid := strings.TrimSpace(data.ID)
	if uuid == "" {
		return nodes.Input{}, errors.New("VMess UUID is required")
	}

	portNum, err := parsePort(data.Port)
	if err != nil || portNum == 0 {
		return nodes.Input{}, errors.New("VMess port is required and must be 1-65535")
	}

	name := normalizeName(data.PS, "VMess")

	typeLower := strings.ToLower(strings.TrimSpace(data.Type))
	switch typeLower {
	case "", "none", "tcp":
	default:
		return nodes.Input{}, fmt.Errorf("unsupported VMess header type %q", data.Type)
	}

	var tlsInput *nodes.TLSInput
	tlsLower := strings.ToLower(strings.TrimSpace(data.TLS))
	switch tlsLower {
	case "tls", "1", "true":
		var alpn []string
		if strings.TrimSpace(data.ALPN) != "" {
			for _, item := range strings.Split(data.ALPN, ",") {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					alpn = append(alpn, trimmed)
				}
			}
		}
		tlsInput = &nodes.TLSInput{
			Enabled:    true,
			ServerName: strings.TrimSpace(data.SNI),
			ALPN:       alpn,
		}
	case "", "none", "0", "false":
		// TLS not enabled
	default:
		return nodes.Input{}, fmt.Errorf("unsupported VMess TLS configuration %q", data.TLS)
	}

	var transportInput *nodes.TransportInput
	netLower := strings.ToLower(strings.TrimSpace(data.Net))
	switch netLower {
	case "ws", "websocket":
		transportInput = &nodes.TransportInput{
			Type: "ws",
			Path: strings.TrimSpace(data.Path),
			Host: strings.TrimSpace(data.Host),
		}
	case "tcp", "":
		// standard TCP
	default:
		return nodes.Input{}, fmt.Errorf("unsupported VMess network type %q (only tcp and ws are supported)", data.Net)
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
}

func parsePort(val any) (uint16, error) {
	if val == nil {
		return 0, errors.New("nil port")
	}
	switch v := val.(type) {
	case float64:
		if v < 1 || v > 65535 {
			return 0, errors.New("port out of range")
		}
		return uint16(v), nil
	case int:
		if v < 1 || v > 65535 {
			return 0, errors.New("port out of range")
		}
		return uint16(v), nil
	case string:
		p, err := strconv.ParseUint(strings.TrimSpace(v), 10, 16)
		if err != nil || p == 0 {
			return 0, errors.New("invalid port string")
		}
		return uint16(p), nil
	default:
		return 0, errors.New("unsupported port type")
	}
}

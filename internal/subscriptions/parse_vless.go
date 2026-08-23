package subscriptions

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"winrouter/internal/nodes"
)

func parseVLESSURI(value string) (nodes.Input, error) {
	if !strings.HasPrefix(strings.ToLower(value), "vless://") {
		return nodes.Input{}, errors.New("unsupported VLESS protocol scheme")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nodes.Input{}, errors.New("invalid VLESS URI")
	}

	uuid := parsed.User.Username()
	if strings.TrimSpace(uuid) == "" {
		return nodes.Input{}, errors.New("missing VLESS UUID")
	}

	server := parsed.Hostname()
	if strings.TrimSpace(server) == "" {
		return nodes.Input{}, errors.New("missing VLESS server host")
	}

	portStr := parsed.Port()
	portNum, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || portNum == 0 {
		return nodes.Input{}, errors.New("missing or invalid VLESS port")
	}

	query := parsed.Query()
	flow := strings.TrimSpace(query.Get("flow"))
	switch flow {
	case "xtls-rprx-vision", "":
	default:
		return nodes.Input{}, fmt.Errorf("unsupported VLESS flow %q", flow)
	}

	encryption := strings.ToLower(strings.TrimSpace(query.Get("encryption")))
	switch encryption {
	case "none", "":
	default:
		return nodes.Input{}, fmt.Errorf("unsupported VLESS encryption %q", encryption)
	}

	security := strings.ToLower(strings.TrimSpace(query.Get("security")))
	var tlsInput *nodes.TLSInput
	switch security {
	case "tls":
		sni := strings.TrimSpace(query.Get("sni"))
		if sni == "" {
			sni = strings.TrimSpace(query.Get("peer"))
		}
		if sni == "" {
			sni = server
		}
		var alpn []string
		if alpnStr := strings.TrimSpace(query.Get("alpn")); alpnStr != "" {
			for _, item := range strings.Split(alpnStr, ",") {
				if trimmed := strings.TrimSpace(item); trimmed != "" && !strings.EqualFold(trimmed, "default") {
					alpn = append(alpn, trimmed)
				}
			}
		}
		tlsInput = &nodes.TLSInput{
			Enabled:    true,
			ServerName: sni,
			ALPN:       alpn,
		}
	case "", "none":
		// TLS not enabled
	default:
		return nodes.Input{}, fmt.Errorf("unsupported VLESS security %q (only standard TLS is supported)", security)
	}

	var transportInput *nodes.TransportInput
	transportType := strings.ToLower(strings.TrimSpace(query.Get("type")))
	switch transportType {
	case "ws", "websocket":
		path := query.Get("path")
		host := query.Get("host")
		transportInput = &nodes.TransportInput{
			Type: "ws",
			Path: path,
			Host: host,
		}
	case "tcp", "":
		// standard TCP
	default:
		return nodes.Input{}, fmt.Errorf("unsupported VLESS transport type %q (only tcp and ws are supported)", transportType)
	}

	name := normalizeName(parsed.Fragment, "VLESS")

	return nodes.Input{
		Name:   name,
		Type:   nodes.TypeVLESS,
		Server: server,
		Port:   uint16(portNum),
		Egress: nodes.EgressB,
		Authentication: nodes.AuthenticationInput{
			UUID: uuid,
			Flow: flow,
		},
		TLS:       tlsInput,
		Transport: transportInput,
	}, nil
}

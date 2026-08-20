package subscriptions

import (
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"

	"winrouter/internal/nodes"
)

func parseTrojanURI(value string) (nodes.Input, error) {
	if !strings.HasPrefix(strings.ToLower(value), "trojan://") {
		return nodes.Input{}, errors.New("unsupported Trojan protocol scheme")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nodes.Input{}, errors.New("invalid Trojan URI")
	}

	password := parsed.User.Username()
	if strings.TrimSpace(password) == "" {
		return nodes.Input{}, errors.New("missing Trojan password")
	}

	server := parsed.Hostname()
	if strings.TrimSpace(server) == "" {
		return nodes.Input{}, errors.New("missing Trojan server host")
	}

	portStr := parsed.Port()
	portNum, err := strconv.ParseUint(portStr, 10, 16)
	if err != nil || portNum == 0 {
		return nodes.Input{}, errors.New("missing or invalid Trojan port")
	}

	query := parsed.Query()
	security := strings.ToLower(strings.TrimSpace(query.Get("security")))
	switch security {
	case "tls", "":
	default:
		return nodes.Input{}, fmt.Errorf("unsupported Trojan security %q (only tls is supported)", security)
	}

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
			if trimmed := strings.TrimSpace(item); trimmed != "" {
				alpn = append(alpn, trimmed)
			}
		}
	}

	tlsInput := &nodes.TLSInput{
		Enabled:    true,
		ServerName: sni,
		ALPN:       alpn,
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
		return nodes.Input{}, fmt.Errorf("unsupported Trojan transport type %q (only tcp and ws are supported)", transportType)
	}

	name := normalizeName(parsed.Fragment, "Trojan")

	return nodes.Input{
		Name:     name,
		Type:     nodes.TypeTrojan,
		Server:   server,
		Port:     uint16(portNum),
		Egress:   nodes.EgressB,
		Password: password,
		Authentication: nodes.AuthenticationInput{
			Password: password,
		},
		TLS:       tlsInput,
		Transport: transportInput,
	}, nil
}

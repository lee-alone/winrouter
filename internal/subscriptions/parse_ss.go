package subscriptions

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"winrouter/internal/nodes"
)

func parseShadowsocksURI(value string) (nodes.Input, error) {
	if !strings.HasPrefix(strings.ToLower(value), "ss://") {
		return nodes.Input{}, errors.New("unsupported Shadowsocks protocol scheme")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nodes.Input{}, errors.New("invalid ss URI format")
	}
	name := normalizeName(parsed.Fragment, "Shadowsocks")

	var method, password, host, portText string
	if parsed.User != nil && parsed.User.String() != "" && parsed.Host != "" {
		// SIP002 format: ss://base64(method:password)@host:port
		userPart := parsed.User.String()
		credential, err := decodeBase64(userPart)
		if err != nil {
			// Might be raw method:password (rare, but handle gracefully)
			credential = []byte(userPart)
		}
		method, password, err = splitCredential(string(credential))
		if err != nil {
			return nodes.Input{}, err
		}
		host, portText, err = net.SplitHostPort(parsed.Host)
		if err != nil {
			return nodes.Input{}, errors.New("invalid server and port")
		}
	} else {
		// Legacy format: ss://base64(method:password@host:port)#name
		payload := strings.TrimPrefix(strings.SplitN(value, "#", 2)[0], "ss://")
		if idx := strings.Index(payload, "?"); idx != -1 {
			payload = payload[:idx]
		}
		decoded, err := decodeBase64(payload)
		if err != nil {
			return nodes.Input{}, errors.New("invalid legacy ss payload")
		}
		credential, endpoint, ok := strings.Cut(string(decoded), "@")
		if !ok {
			return nodes.Input{}, errors.New("invalid legacy ss payload: missing @ separator")
		}
		method, password, err = splitCredential(credential)
		if err != nil {
			return nodes.Input{}, err
		}
		host, portText, err = net.SplitHostPort(endpoint)
		if err != nil {
			return nodes.Input{}, errors.New("invalid server and port")
		}
	}

	if parsed.Query().Get("plugin") != "" {
		return nodes.Input{}, errors.New("Shadowsocks plugins are not supported")
	}

	if !isSupportedSSCipher(method) {
		return nodes.Input{}, fmt.Errorf("unsupported Shadowsocks cipher/method %q", method)
	}

	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return nodes.Input{}, errors.New("invalid port")
	}
	if strings.TrimSpace(host) == "" {
		return nodes.Input{}, errors.New("missing server address")
	}

	return nodes.Input{
		Name:     name,
		Type:     nodes.TypeShadowsocks,
		Server:   host,
		Port:     uint16(port),
		Egress:   nodes.EgressB,
		Username: method,
		Password: password,
		Authentication: nodes.AuthenticationInput{
			Method:   method,
			Password: password,
		},
	}, nil
}

func isSupportedSSCipher(cipher string) bool {
	switch strings.ToLower(strings.TrimSpace(cipher)) {
	case "aes-128-gcm", "aes-192-gcm", "aes-256-gcm", "chacha20-ietf-poly1305",
		"2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305",
		"none", "plain":
		return true
	default:
		return false
	}
}

func splitCredential(value string) (string, string, error) {
	method, password, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(method) == "" || password == "" {
		return "", "", errors.New("method and password are required")
	}
	return strings.ToLower(strings.TrimSpace(method)), password, nil
}

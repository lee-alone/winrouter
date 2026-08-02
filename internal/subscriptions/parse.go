package subscriptions

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"strconv"
	"strings"

	"winrouter/internal/nodes"
)

func Parse(data []byte) ([]nodes.Input, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) > 0 && trimmed[0] != '{' {
		return parseEncodedShadowsocks(trimmed)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var value document
	if err := decoder.Decode(&value); err != nil {
		return nil, fmt.Errorf("decode subscription: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return nil, errors.New("subscription has trailing JSON")
	}
	if value.Version != SchemaVersion {
		return nil, fmt.Errorf("unsupported subscription version %d", value.Version)
	}
	if len(value.Nodes) == 0 || len(value.Nodes) > 500 {
		return nil, errors.New("subscription must contain 1 to 500 nodes")
	}
	result := make([]nodes.Input, 0, len(value.Nodes))
	for _, item := range value.Nodes {
		result = append(result, nodes.Input{Name: item.Name, Type: item.Type, Server: item.Server, Port: item.Port, Username: item.Username, Password: item.Password})
	}
	return result, nil
}

func parseEncodedShadowsocks(data []byte) ([]nodes.Input, error) {
	decoded, err := decodeBase64(string(data))
	if err != nil {
		return nil, errors.New("subscription is neither versioned JSON nor valid Base64")
	}
	lines := strings.Fields(string(decoded))
	if len(lines) == 0 || len(lines) > 500 {
		return nil, errors.New("subscription must contain 1 to 500 nodes")
	}
	result := make([]nodes.Input, 0, len(lines))
	for index, line := range lines {
		item, err := parseShadowsocksURI(line)
		if err != nil {
			return nil, fmt.Errorf("Shadowsocks node %d: %w", index+1, err)
		}
		result = append(result, item)
	}
	return result, nil
}

func parseShadowsocksURI(value string) (nodes.Input, error) {
	if !strings.HasPrefix(strings.ToLower(value), "ss://") {
		return nodes.Input{}, errors.New("unsupported subscription node protocol")
	}
	parsed, err := url.Parse(value)
	if err != nil {
		return nodes.Input{}, errors.New("invalid ss URI")
	}
	name := strings.TrimSpace(parsed.Fragment)
	if name == "" {
		name = "Shadowsocks"
	}
	var method, password, host, portText string
	if parsed.Host != "" {
		credential, err := decodeBase64(parsed.User.String())
		if err != nil {
			return nodes.Input{}, errors.New("invalid encoded method and password")
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
		payload := strings.TrimPrefix(strings.SplitN(value, "#", 2)[0], "ss://")
		decoded, err := decodeBase64(payload)
		if err != nil {
			return nodes.Input{}, errors.New("invalid legacy ss payload")
		}
		credential, endpoint, ok := strings.Cut(string(decoded), "@")
		if !ok {
			return nodes.Input{}, errors.New("invalid legacy ss payload")
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
	port, err := strconv.ParseUint(portText, 10, 16)
	if err != nil || port == 0 {
		return nodes.Input{}, errors.New("invalid port")
	}
	return nodes.Input{Name: name, Type: nodes.TypeShadowsocks, Server: host, Port: uint16(port), Username: method, Password: password}, nil
}

func splitCredential(value string) (string, string, error) {
	method, password, ok := strings.Cut(value, ":")
	if !ok || strings.TrimSpace(method) == "" || password == "" {
		return "", "", errors.New("method and password are required")
	}
	return strings.ToLower(strings.TrimSpace(method)), password, nil
}

func decodeBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	for _, encoding := range []*base64.Encoding{base64.StdEncoding, base64.RawStdEncoding, base64.URLEncoding, base64.RawURLEncoding} {
		if decoded, err := encoding.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	return nil, errors.New("invalid Base64")
}

package subscriptions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	"winrouter/internal/nodes"
)

func Parse(data []byte) ([]nodes.Input, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, errors.New("empty subscription data")
	}

	// 1. Strict versioned JSON format
	if trimmed[0] == '{' {
		if n, err := parseVersionedJSON(trimmed); err == nil {
			return n, nil
		}
	}

	// 2. Clash/Mihomo YAML format
	if isYAMLSubscription(trimmed) {
		if nodesList, err := parseClashYAML(trimmed); err == nil && len(nodesList) > 0 {
			return nodesList, nil
		}
	}

	// 3. Multi-line plain text or Base64 URI list
	return parseURIsSubscription(trimmed)
}

func isYAMLSubscription(data []byte) bool {
	return bytes.Contains(data, []byte("proxies:")) ||
		bytes.Contains(data, []byte("Proxy:")) ||
		bytes.HasPrefix(data, []byte("port:")) ||
		bytes.HasPrefix(data, []byte("mixed-port:")) ||
		bytes.HasPrefix(data, []byte("mode:")) ||
		bytes.HasPrefix(data, []byte("rules:"))
}

func parseVersionedJSON(data []byte) ([]nodes.Input, error) {
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
		result = append(result, nodes.Input{
			Name:     normalizeName(item.Name, "Node"),
			Type:     item.Type,
			Server:   item.Server,
			Port:     item.Port,
			Egress:   nodes.EgressB,
			Username: item.Username,
			Password: item.Password,
			Authentication: nodes.AuthenticationInput{
				Username: item.Username,
				Password: item.Password,
				Method:   item.Username,
			},
		})
	}
	return result, nil
}

func parseURIsSubscription(data []byte) ([]nodes.Input, error) {
	text := string(data)
	lines := extractNonEmptyLines(text)

	// Check if the lines directly contain valid proxy URI schemes
	if hasSupportedURIScheme(lines) {
		if result, err := parseLines(lines); err == nil && len(result) > 0 {
			return result, nil
		}
	}

	// Otherwise, try Base64 decoding the whole body
	decoded, err := decodeBase64(text)
	if err == nil {
		trimmedDecoded := bytes.TrimSpace(decoded)
		if len(trimmedDecoded) > 0 && trimmedDecoded[0] == '{' {
			if n, err := parseVersionedJSON(trimmedDecoded); err == nil {
				return n, nil
			}
		}
		if isYAMLSubscription(trimmedDecoded) {
			if nodesList, err := parseClashYAML(trimmedDecoded); err == nil && len(nodesList) > 0 {
				return nodesList, nil
			}
		}
		decodedLines := extractNonEmptyLines(string(decoded))
		if len(decodedLines) > 0 {
			if result, err := parseLines(decodedLines); err == nil && len(result) > 0 {
				return result, nil
			}
		}
	}

	// Fallback to parse lines directly if not already succeeded
	if len(lines) > 0 {
		if result, err := parseLines(lines); err == nil && len(result) > 0 {
			return result, nil
		}
	}

	return nil, errors.New("subscription contains no valid or supported proxy nodes")
}

func extractNonEmptyLines(content string) []string {
	raw := strings.Split(content, "\n")
	lines := make([]string, 0, len(raw))
	for _, line := range raw {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "//") {
			lines = append(lines, trimmed)
		}
	}
	return lines
}

func hasSupportedURIScheme(lines []string) bool {
	for _, line := range lines {
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "ss://") ||
			strings.HasPrefix(lower, "vmess://") ||
			strings.HasPrefix(lower, "vless://") ||
			strings.HasPrefix(lower, "trojan://") ||
			strings.HasPrefix(lower, "http://") ||
			strings.HasPrefix(lower, "https://") {
			return true
		}
	}
	return false
}

func parseLines(lines []string) ([]nodes.Input, error) {
	if len(lines) == 0 {
		return nil, errors.New("subscription contains no lines")
	}
	result := make([]nodes.Input, 0, len(lines))
	var lastErr error
	for _, line := range lines {
		item, err := parseSingleNodeURI(line)
		if err != nil {
			lastErr = err
			continue
		}
		result = append(result, item)
		if len(result) >= 500 {
			break
		}
	}
	if len(result) == 0 {
		if lastErr != nil {
			return nil, fmt.Errorf("no valid proxy nodes found: %w", lastErr)
		}
		return nil, errors.New("no valid proxy nodes found")
	}
	return result, nil
}

func parseSingleNodeURI(line string) (nodes.Input, error) {
	lower := strings.ToLower(line)
	switch {
	case strings.HasPrefix(lower, "ss://"):
		return parseShadowsocksURI(line)
	case strings.HasPrefix(lower, "vmess://"):
		return parseVMessURI(line)
	case strings.HasPrefix(lower, "vless://"):
		return parseVLESSURI(line)
	case strings.HasPrefix(lower, "trojan://"):
		return parseTrojanURI(line)
	case strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://"):
		return parseHTTPNodeURI(line)
	default:
		return nodes.Input{}, fmt.Errorf("unsupported protocol scheme in line: %s", truncateString(line, 30))
	}
}

func parseHTTPNodeURI(value string) (nodes.Input, error) {
	parsed, err := url.Parse(value)
	if err != nil || parsed.Hostname() == "" {
		return nodes.Input{}, errors.New("invalid HTTP proxy URI")
	}
	name := normalizeName(parsed.Fragment, "HTTP Proxy")
	server := parsed.Hostname()
	portNum := uint16(80)
	if strings.EqualFold(parsed.Scheme, "https") {
		portNum = 443
	}
	if portStr := parsed.Port(); portStr != "" {
		p, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil || p == 0 {
			return nodes.Input{}, fmt.Errorf("invalid port %q in %s proxy URI", portStr, parsed.Scheme)
		}
		portNum = uint16(p)
	}
	var username, password string
	if parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
	}
	var tlsInput *nodes.TLSInput
	if strings.EqualFold(parsed.Scheme, "https") {
		tlsInput = &nodes.TLSInput{
			Enabled:    true,
			ServerName: server,
		}
	}
	return nodes.Input{
		Name:     name,
		Type:     nodes.TypeHTTP,
		Server:   server,
		Port:     portNum,
		Egress:   nodes.EgressB,
		Username: username,
		Password: password,
		Authentication: nodes.AuthenticationInput{
			Username: username,
			Password: password,
		},
		TLS: tlsInput,
	}, nil
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

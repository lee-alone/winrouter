package subscriptions

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
		return parseVersionedJSON(trimmed)
	}

	// 2. Clash/Mihomo YAML format
	if isYAMLSubscription(trimmed) {
		nodes, err := parseClashYAML(trimmed)
		if err == nil {
			return nodes, nil
		}
		// If it has explicit "proxies:" keyword and failed parsing, return the YAML error
		if bytes.Contains(trimmed, []byte("proxies:")) {
			return nil, err
		}
	}

	// 3. Multi-line plain text or Base64 URI list
	return parseURIsSubscription(trimmed)
}

func isYAMLSubscription(data []byte) bool {
	return bytes.Contains(data, []byte("proxies:")) ||
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
		return parseLines(lines)
	}

	// Otherwise, try Base64 decoding the whole body
	decoded, err := decodeBase64(text)
	if err != nil {
		return nil, errors.New("subscription is neither versioned JSON nor valid Base64 / URI list")
	}

	decodedLines := extractNonEmptyLines(string(decoded))
	if len(decodedLines) == 0 {
		return nil, errors.New("subscription contains no nodes")
	}
	return parseLines(decodedLines)
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
			strings.HasPrefix(lower, "trojan://") {
			return true
		}
	}
	return false
}

func parseLines(lines []string) ([]nodes.Input, error) {
	if len(lines) == 0 || len(lines) > 500 {
		return nil, errors.New("subscription must contain 1 to 500 nodes")
	}
	result := make([]nodes.Input, 0, len(lines))
	for index, line := range lines {
		item, err := parseSingleNodeURI(line)
		if err != nil {
			return nil, fmt.Errorf("subscription node %d: %w", index+1, err)
		}
		result = append(result, item)
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
	default:
		return nodes.Input{}, fmt.Errorf("unsupported protocol scheme in line: %s", truncateString(line, 30))
	}
}

func truncateString(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

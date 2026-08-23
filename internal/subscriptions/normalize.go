package subscriptions

import (
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

func normalizeName(raw string, defaultName string) string {
	name := strings.TrimSpace(raw)
	if unescaped, err := url.QueryUnescape(name); err == nil && unescaped != "" {
		name = unescaped
	}
	// Strip control characters
	var b strings.Builder
	for _, r := range name {
		if !unicode.IsControl(r) {
			b.WriteRune(r)
		}
	}
	name = strings.TrimSpace(b.String())
	if name == "" {
		name = defaultName
	}
	if len([]rune(name)) > 80 {
		name = string([]rune(name)[:80])
	}
	return name
}

func normalizePath(raw string) string {
	path := strings.TrimSpace(raw)
	if path == "" {
		return ""
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}
	return path
}

func decodeBase64(value string) ([]byte, error) {
	// Strip all internal and external whitespace / newlines (common in MIME or airport base64 payloads)
	var b strings.Builder
	for _, r := range value {
		if r != '\r' && r != '\n' && r != ' ' && r != '\t' {
			b.WriteRune(r)
		}
	}
	cleaned := b.String()
	if cleaned == "" {
		return nil, errors.New("empty base64 string")
	}

	// Try standard and URL encodings with or without padding
	encodings := []*base64.Encoding{
		base64.StdEncoding,
		base64.RawStdEncoding,
		base64.URLEncoding,
		base64.RawURLEncoding,
	}
	for _, encoding := range encodings {
		if decoded, err := encoding.DecodeString(cleaned); err == nil {
			return decoded, nil
		}
	}
	// In case of missing padding, pad to multiple of 4
	if rem := len(cleaned) % 4; rem > 0 {
		padded := cleaned + strings.Repeat("=", 4-rem)
		for _, encoding := range encodings {
			if decoded, err := encoding.DecodeString(padded); err == nil {
				return decoded, nil
			}
		}
	}
	return nil, errors.New("invalid Base64")
}

func parseAlterID(val any) (int, error) {
	if val == nil {
		return 0, nil
	}
	switch v := val.(type) {
	case int:
		if v < 0 || v > 65535 {
			return 0, fmt.Errorf("alterId %d out of range", v)
		}
		return v, nil
	case float64:
		if math.IsNaN(v) || math.IsInf(v, 0) || math.Trunc(v) != v || v < 0 || v > 65535 {
			return 0, fmt.Errorf("invalid alterId float value %v", v)
		}
		return int(v), nil
	case string:
		trimmed := strings.TrimSpace(v)
		if trimmed == "" {
			return 0, nil
		}
		n, err := strconv.Atoi(trimmed)
		if err != nil || n < 0 || n > 65535 {
			return 0, fmt.Errorf("invalid alterId string %q", v)
		}
		return n, nil
	default:
		return 0, fmt.Errorf("invalid alterId type %T", val)
	}
}

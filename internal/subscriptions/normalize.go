package subscriptions

import (
	"encoding/base64"
	"errors"
	"net/url"
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

func decodeBase64(value string) ([]byte, error) {
	value = strings.TrimSpace(value)
	if value == "" {
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
		if decoded, err := encoding.DecodeString(value); err == nil {
			return decoded, nil
		}
	}
	// In case of missing padding, pad to multiple of 4
	if rem := len(value) % 4; rem > 0 {
		padded := value + strings.Repeat("=", 4-rem)
		for _, encoding := range encodings {
			if decoded, err := encoding.DecodeString(padded); err == nil {
				return decoded, nil
			}
		}
	}
	return nil, errors.New("invalid Base64")
}

package nodes

import (
	"net/netip"
	"strings"
)

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
		} else if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
			return false
		}
	}
	return true
}

func supportedShadowsocksMethod(value string) bool {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "aes-128-gcm", "aes-192-gcm", "aes-256-gcm", "chacha20-ietf-poly1305", "xchacha20-ietf-poly1305",
		"2022-blake3-aes-128-gcm", "2022-blake3-aes-256-gcm", "2022-blake3-chacha20-poly1305":
		return true
	default:
		return false
	}
}

func usableIPv4(address netip.Addr) bool {
	return address.Is4() && !address.IsUnspecified() && !address.IsLoopback() && !address.IsMulticast()
}

func usableIPv6(address netip.Addr) bool {
	return address.Is6() && !address.IsLoopback() && !address.IsUnspecified() && !address.IsLinkLocalUnicast() && !address.IsMulticast() && !address.IsPrivate()
}

func validDNSName(value string) bool {
	if len(value) == 0 || len(value) > 253 || !strings.Contains(value, ".") {
		return false
	}
	for _, label := range strings.Split(value, ".") {
		if len(label) == 0 || len(label) > 63 || label[0] == '-' || label[len(label)-1] == '-' {
			return false
		}
		for _, character := range label {
			if (character < 'a' || character > 'z') && (character < '0' || character > '9') && character != '-' {
				return false
			}
		}
	}
	return true
}

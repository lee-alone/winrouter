package config

import (
	"sort"
	"strings"
)

func dnsStrategy(ipv6Policy string) string {
	if ipv6Policy == IPv6Split {
		return "prefer_ipv4"
	}
	return "ipv4_only"
}

func dnsServer(tag string, input MVPDNSServer, detour string) DNSServer {
	server := DNSServer{Type: input.Type, Tag: tag, Server: input.Server, ServerPort: input.Port, Detour: detour}
	if input.Type == "tls" || input.Type == "https" {
		server.TLS = &TLSConfig{Enabled: true, ServerName: input.ServerName}
	}
	if input.Type == "https" {
		server.Path = "/dns-query"
	}
	return server
}

func normalizedDomainCopy(values []string) []string {
	result := make([]string, 0, len(values))
	seen := make(map[string]struct{})
	for _, value := range values {
		value = strings.ToLower(strings.Trim(strings.TrimSpace(value), "."))
		if value != "" {
			if _, ok := seen[value]; !ok {
				seen[value] = struct{}{}
				result = append(result, value)
			}
		}
	}
	sort.Strings(result)
	return result
}

func finalDNSResolver(defaultOutbound string) string {
	if defaultOutbound == "a" {
		return "dns-domestic"
	}
	if defaultOutbound == "c" {
		return "dns-proxy"
	}
	return "dns-global"
}

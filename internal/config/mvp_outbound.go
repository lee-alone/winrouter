package config

import (
	"net"
	"strings"
)

func buildOutboundFromProxy(p MVPProxy, tag, bindInterface, detour string) Outbound {
	out := Outbound{
		Type:          p.Type,
		Tag:           tag,
		Server:        p.Server,
		ServerPort:    p.Port,
		BindInterface: bindInterface,
		Detour:        detour,
		Method:        p.Method,
		Username:      p.Username,
		Password:      p.Password,
		UUID:          p.UUID,
		Flow:          p.Flow,
		Security:      p.Security,
	}
	if p.TLS != nil && p.TLS.Enabled {
		sni := p.TLS.ServerName
		if sni == "" && p.Server != "" && net.ParseIP(p.Server) == nil {
			sni = p.Server
		}
		var cleanALPN []string
		for _, a := range p.TLS.ALPN {
			trimmed := strings.TrimSpace(a)
			if trimmed != "" && !strings.EqualFold(trimmed, "default") {
				cleanALPN = append(cleanALPN, trimmed)
			}
		}
		out.TLS = &TLSConfig{
			Enabled:    true,
			ServerName: sni,
			Insecure:   p.TLS.Insecure,
			ALPN:       cleanALPN,
		}
	}
	if p.Type == "trojan" && out.TLS == nil {
		out.TLS = &TLSConfig{
			Enabled:    true,
			ServerName: p.Server,
		}
	}
	if p.Transport != nil {
		var headers map[string][]string
		if p.Transport.Host != "" {
			headers = map[string][]string{
				"Host": {p.Transport.Host},
			}
		}
		out.Transport = &TransportConfig{
			Type:    p.Transport.Type,
			Path:    p.Transport.Path,
			Headers: headers,
		}
	}
	return out
}

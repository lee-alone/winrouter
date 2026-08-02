package nodes

import (
	"context"
	"errors"
	"net"
	"net/netip"
)

func ResolveIPv4(ctx context.Context, hostname, dnsAddress, sourceAddress string) (string, error) {
	if !validDNSName(hostname) {
		return "", errors.New("invalid proxy DNS name")
	}
	dns, err := netip.ParseAddr(dnsAddress)
	if err != nil || !dns.Is4() {
		return "", errors.New("proxy bootstrap DNS must be a fixed IPv4 address")
	}
	source, err := netip.ParseAddr(sourceAddress)
	if err != nil || !usableIPv4(source) {
		return "", errors.New("interface B requires a usable source IPv4 address")
	}
	resolver := net.Resolver{PreferGo: true, StrictErrors: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		dialer := net.Dialer{LocalAddr: &net.UDPAddr{IP: net.IP(source.AsSlice())}}
		return dialer.DialContext(ctx, "udp", net.JoinHostPort(dns.String(), "53"))
	}}
	addresses, err := resolver.LookupNetIP(ctx, "ip4", hostname)
	if err != nil {
		return "", err
	}
	for _, address := range addresses {
		if usableIPv4(address) {
			return address.String(), nil
		}
	}
	return "", errors.New("proxy DNS returned no usable IPv4 address")
}

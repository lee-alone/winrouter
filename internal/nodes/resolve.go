package nodes

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
)

func ResolveIPv4(ctx context.Context, hostname, dnsAddress, sourceAddress string) (string, error) {
	address, err := resolveIP(ctx, hostname, dnsAddress, sourceAddress, "ip4")
	if err != nil {
		return "", err
	}
	return address, nil
}

// ResolveEndpoint resolves a proxy hostname over the configured bootstrap
// DNS transport, preferring IPv4 but falling back to IPv6 for IPv6-only names.
func ResolveEndpoint(ctx context.Context, hostname, dnsAddress, sourceAddress string) (string, error) {
	if address, err := resolveIP(ctx, hostname, dnsAddress, sourceAddress, "ip4"); err == nil {
		return address, nil
	}
	return resolveIP(ctx, hostname, dnsAddress, sourceAddress, "ip6")
}

func resolveIP(ctx context.Context, hostname, dnsAddress, sourceAddress, network string) (string, error) {
	if !validDNSName(hostname) {
		return "", errors.New("invalid proxy DNS name")
	}
	dns, err := netip.ParseAddr(dnsAddress)
	if err != nil || (!dns.Is4() && !dns.Is6()) {
		return "", errors.New("proxy bootstrap DNS must be a usable fixed IP address")
	}
	source, err := netip.ParseAddr(sourceAddress)
	if err != nil || (dns.Is4() && !usableIPv4(source)) || (dns.Is6() && !usableIPv6(source)) {
		return "", errors.New("interface requires a usable source address matching DNS address family")
	}
	resolver := net.Resolver{PreferGo: true, StrictErrors: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
		dialer := net.Dialer{LocalAddr: &net.UDPAddr{IP: net.IP(source.AsSlice())}}
		return dialer.DialContext(ctx, "udp", net.JoinHostPort(dns.String(), "53"))
	}}
	addresses, err := resolver.LookupNetIP(ctx, network, hostname)
	if err != nil {
		return "", err
	}
	for _, address := range addresses {
		if (network == "ip4" && usableIPv4(address)) || (network == "ip6" && usableIPv6(address)) {
			return address.String(), nil
		}
	}
	return "", fmt.Errorf("proxy DNS returned no usable %s address", network)
}

func usableIPv6(address netip.Addr) bool {
	return address.Is6() && !address.IsLoopback() && !address.IsUnspecified() && !address.IsLinkLocalUnicast() && !address.IsMulticast() && !address.IsPrivate()
}

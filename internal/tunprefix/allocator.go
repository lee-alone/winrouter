package tunprefix

import (
	"errors"
	"fmt"
	"net/netip"

	"winrouter/internal/interfaces"
	"winrouter/internal/routes"
)

var ErrPoolExhausted = errors.New("TUN prefix pool is exhausted")

var DefaultPool = []string{
	"172.19.0.0/30", "172.19.0.4/30", "172.19.0.8/30", "172.19.0.12/30",
	"172.19.0.16/30", "172.19.0.20/30", "172.19.0.24/30", "172.19.0.28/30",
	"10.255.255.0/30", "10.255.255.4/30", "10.255.255.8/30", "10.255.255.12/30",
}

var privateBlocks = []netip.Prefix{
	netip.MustParsePrefix("10.0.0.0/8"),
	netip.MustParsePrefix("172.16.0.0/12"),
	netip.MustParsePrefix("192.168.0.0/16"),
	netip.MustParsePrefix("fc00::/7"),
}

type ConflictSource string

const (
	ConflictInterface ConflictSource = "interface"
	ConflictRoute     ConflictSource = "route"
)

type Conflict struct {
	Candidate      string         `json:"candidate"`
	ExistingPrefix string         `json:"existing_prefix"`
	Source         ConflictSource `json:"source"`
	InterfaceIndex uint32         `json:"interface_index,omitempty"`
	InterfaceName  string         `json:"interface_name,omitempty"`
}

type Allocation struct {
	Prefix    string     `json:"prefix"`
	Reused    bool       `json:"reused"`
	Conflicts []Conflict `json:"conflicts"`
}

type PoolExhaustedError struct {
	Conflicts []Conflict `json:"conflicts"`
}

func (e *PoolExhaustedError) Error() string {
	return fmt.Sprintf("%v (%d conflict(s))", ErrPoolExhausted, len(e.Conflicts))
}

func (e *PoolExhaustedError) Unwrap() error { return ErrPoolExhausted }

func Allocate(pool []string, preferred string, adapters []interfaces.Adapter, routeTable []routes.Route) (Allocation, error) {
	parsedPool, err := parsePool(pool)
	if err != nil {
		return Allocation{}, err
	}
	ordered := preferFirst(parsedPool, preferred)
	allConflicts := make([]Conflict, 0)
	for _, candidate := range ordered {
		conflicts := conflictsFor(candidate, adapters, routeTable)
		if len(conflicts) == 0 {
			return Allocation{Prefix: candidate.String(), Reused: samePrefix(candidate, preferred), Conflicts: allConflicts}, nil
		}
		allConflicts = append(allConflicts, conflicts...)
	}
	return Allocation{}, &PoolExhaustedError{Conflicts: allConflicts}
}

func parsePool(pool []string) ([]netip.Prefix, error) {
	result := make([]netip.Prefix, 0, len(pool))
	seen := make(map[netip.Prefix]struct{})
	for _, value := range pool {
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			return nil, fmt.Errorf("invalid TUN pool prefix %q: %w", value, err)
		}
		prefix = prefix.Masked()
		if !isPrivatePrefix(prefix) {
			return nil, fmt.Errorf("TUN pool prefix %q is not private", value)
		}
		if _, exists := seen[prefix]; exists {
			return nil, fmt.Errorf("duplicate TUN pool prefix %q", prefix)
		}
		seen[prefix] = struct{}{}
		result = append(result, prefix)
	}
	if len(result) == 0 {
		return nil, errors.New("TUN prefix pool is empty")
	}
	return result, nil
}

func isPrivatePrefix(prefix netip.Prefix) bool {
	for _, block := range privateBlocks {
		if block.Addr().BitLen() == prefix.Addr().BitLen() && prefix.Bits() >= block.Bits() && block.Contains(prefix.Addr()) {
			return true
		}
	}
	return false
}

func preferFirst(pool []netip.Prefix, preferred string) []netip.Prefix {
	wanted, err := netip.ParsePrefix(preferred)
	if err != nil {
		return pool
	}
	wanted = wanted.Masked()
	result := make([]netip.Prefix, 0, len(pool))
	for _, candidate := range pool {
		if candidate == wanted {
			result = append(result, candidate)
		}
	}
	for _, candidate := range pool {
		if candidate != wanted {
			result = append(result, candidate)
		}
	}
	return result
}

func ConflictsForPrefix(candidate netip.Prefix, adapters []interfaces.Adapter, routeTable []routes.Route) []Conflict {
	return conflictsFor(candidate, adapters, routeTable)
}

func conflictsFor(candidate netip.Prefix, adapters []interfaces.Adapter, routeTable []routes.Route) []Conflict {
	result := make([]Conflict, 0)
	for _, adapter := range adapters {
		for _, address := range adapter.Addresses {
			ip, err := netip.ParseAddr(address.IP)
			if err != nil || ip.BitLen() != candidate.Addr().BitLen() || int(address.PrefixLength) > ip.BitLen() {
				continue
			}
			existing := netip.PrefixFrom(ip, int(address.PrefixLength)).Masked()
			if overlaps(candidate, existing) {
				ifIndex := adapter.Index
				if candidate.Addr().Is6() && adapter.IPv6Index != 0 {
					ifIndex = adapter.IPv6Index
				}
				result = append(result, Conflict{Candidate: candidate.String(), ExistingPrefix: existing.String(), Source: ConflictInterface, InterfaceIndex: ifIndex, InterfaceName: adapter.FriendlyName})
			}
		}
	}
	for _, route := range routeTable {
		existing, err := netip.ParsePrefix(route.Prefix)
		if err != nil || isSummaryRoute(existing.Masked()) || existing.Addr().BitLen() != candidate.Addr().BitLen() {
			continue
		}
		if overlaps(candidate, existing.Masked()) {
			result = append(result, Conflict{Candidate: candidate.String(), ExistingPrefix: existing.Masked().String(), Source: ConflictRoute, InterfaceIndex: route.InterfaceIndex})
		}
	}
	return result
}

func isSummaryRoute(prefix netip.Prefix) bool {
	if prefix.Bits() == 0 {
		return true
	}
	for _, block := range privateBlocks {
		if prefix == block {
			return true
		}
	}
	return false
}

func overlaps(a, b netip.Prefix) bool {
	return a.Contains(b.Addr()) || b.Contains(a.Addr())
}

func samePrefix(candidate netip.Prefix, preferred string) bool {
	wanted, err := netip.ParsePrefix(preferred)
	return err == nil && candidate == wanted.Masked()
}

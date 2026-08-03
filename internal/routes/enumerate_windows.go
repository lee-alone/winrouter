//go:build windows

package routes

import (
	"fmt"
	"net/netip"
	"sort"
	"unsafe"

	"golang.org/x/sys/windows"
)

func Enumerate() ([]Route, error) {
	var table *windows.MibIpForwardTable2
	if err := windows.GetIpForwardTable2(windows.AF_UNSPEC, &table); err != nil {
		return nil, fmt.Errorf("GetIpForwardTable2: %w", err)
	}
	if table == nil {
		return []Route{}, nil
	}
	defer windows.FreeMibTable(unsafe.Pointer(table))

	result := make([]Route, 0, table.NumEntries)
	for _, row := range table.Rows() {
		prefix, ok := addressPrefix(row.DestinationPrefix)
		if !ok {
			continue
		}
		result = append(result, Route{
			Prefix: prefix.String(), InterfaceLUID: row.InterfaceLuid,
			InterfaceIndex: row.InterfaceIndex, Metric: row.Metric, Protocol: row.Protocol,
		})
	}
	sort.Slice(result, func(i, j int) bool {
		a := netip.MustParsePrefix(result[i].Prefix)
		b := netip.MustParsePrefix(result[j].Prefix)
		if comparison := a.Addr().Compare(b.Addr()); comparison != 0 {
			return comparison < 0
		}
		if a.Bits() != b.Bits() {
			return a.Bits() < b.Bits()
		}
		if result[i].InterfaceIndex != result[j].InterfaceIndex {
			return result[i].InterfaceIndex < result[j].InterfaceIndex
		}
		if result[i].InterfaceLUID != result[j].InterfaceLUID {
			return result[i].InterfaceLUID < result[j].InterfaceLUID
		}
		if result[i].Metric != result[j].Metric {
			return result[i].Metric < result[j].Metric
		}
		return result[i].Protocol < result[j].Protocol
	})
	return result, nil
}

func addressPrefix(value windows.IpAddressPrefix) (netip.Prefix, bool) {
	var address netip.Addr
	switch value.Prefix.Family {
	case windows.AF_INET:
		raw := (*windows.RawSockaddrInet4)(unsafe.Pointer(&value.Prefix))
		address = netip.AddrFrom4(raw.Addr)
	case windows.AF_INET6:
		raw := (*windows.RawSockaddrInet6)(unsafe.Pointer(&value.Prefix))
		address = netip.AddrFrom16(raw.Addr)
	default:
		return netip.Prefix{}, false
	}
	if int(value.PrefixLength) > address.BitLen() {
		return netip.Prefix{}, false
	}
	return netip.PrefixFrom(address, int(value.PrefixLength)).Masked(), true
}

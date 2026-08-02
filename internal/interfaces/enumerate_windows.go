//go:build windows

package interfaces

import (
	"errors"
	"fmt"
	"net"
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

const initialBufferSize = 15 * 1024

func Enumerate() ([]Adapter, error) {
	bufferSize := uint32(initialBufferSize)
	for attempts := 0; attempts < 3; attempts++ {
		buffer := make([]byte, bufferSize)
		first := (*windows.IpAdapterAddresses)(unsafe.Pointer(&buffer[0]))
		err := windows.GetAdaptersAddresses(
			windows.AF_UNSPEC,
			windows.GAA_FLAG_INCLUDE_PREFIX|windows.GAA_FLAG_INCLUDE_GATEWAYS|windows.GAA_FLAG_INCLUDE_ALL_INTERFACES,
			0,
			first,
			&bufferSize,
		)
		if errors.Is(err, syscall.ERROR_BUFFER_OVERFLOW) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("GetAdaptersAddresses: %w", err)
		}

		result := make([]Adapter, 0)
		for current := first; current != nil; current = current.Next {
			result = append(result, mapAdapter(current))
		}
		sort.Slice(result, func(i, j int) bool {
			if result[i].Candidate != result[j].Candidate {
				return result[i].Candidate
			}
			return strings.ToLower(result[i].FriendlyName) < strings.ToLower(result[j].FriendlyName)
		})
		return result, nil
	}
	return nil, fmt.Errorf("GetAdaptersAddresses: adapter data changed repeatedly while sizing buffer")
}

func mapAdapter(source *windows.IpAdapterAddresses) Adapter {
	name := windows.UTF16PtrToString(source.FriendlyName)
	description := windows.UTF16PtrToString(source.Description)
	kind := classify(source.IfType, name, description)
	adapter := Adapter{
		GUID:         normalizeGUID(windows.BytePtrToString(source.AdapterName)),
		LUID:         source.Luid,
		Index:        source.IfIndex,
		IPv6Index:    source.Ipv6IfIndex,
		FriendlyName: name,
		Description:  description,
		Status:       statusName(source.OperStatus),
		Kind:         kind,
		Candidate:    isCandidate(kind),
		MTU:          source.Mtu,
		IPv4Metric:   source.Ipv4Metric,
		IPv6Metric:   source.Ipv6Metric,
		Addresses:    make([]Address, 0),
		Gateways:     make([]string, 0),
		DNSServers:   make([]string, 0),
	}

	if length := min(int(source.PhysicalAddressLength), len(source.PhysicalAddress)); length > 0 {
		adapter.MAC = net.HardwareAddr(source.PhysicalAddress[:length]).String()
	}
	for address := source.FirstUnicastAddress; address != nil; address = address.Next {
		if ip := address.Address.IP(); ip != nil {
			adapter.Addresses = append(adapter.Addresses, Address{IP: ip.String(), PrefixLength: address.OnLinkPrefixLength})
		}
	}
	for gateway := source.FirstGatewayAddress; gateway != nil; gateway = gateway.Next {
		if ip := gateway.Address.IP(); ip != nil {
			adapter.Gateways = append(adapter.Gateways, ip.String())
		}
	}
	for server := source.FirstDnsServerAddress; server != nil; server = server.Next {
		if ip := server.Address.IP(); ip != nil {
			adapter.DNSServers = appendUnique(adapter.DNSServers, ip.String())
		}
	}
	return adapter
}

func appendUnique(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func normalizeGUID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" || strings.HasPrefix(value, "{") {
		return value
	}
	return "{" + value + "}"
}

func statusName(status uint32) string {
	switch status {
	case windows.IfOperStatusUp:
		return "up"
	case windows.IfOperStatusDown:
		return "down"
	case windows.IfOperStatusTesting:
		return "testing"
	case windows.IfOperStatusDormant:
		return "dormant"
	case windows.IfOperStatusNotPresent:
		return "not-present"
	case windows.IfOperStatusLowerLayerDown:
		return "lower-layer-down"
	default:
		return "unknown"
	}
}

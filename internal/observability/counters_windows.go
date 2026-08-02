//go:build windows

package observability

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"winrouter/internal/interfaces"
)

var iphlpapi = windows.NewLazySystemDLL("iphlpapi.dll")
var getIfEntry2 = iphlpapi.NewProc("GetIfEntry2")

type mibIfRow2 struct {
	InterfaceLuid               uint64
	InterfaceIndex              uint32
	InterfaceGuid               windows.GUID
	Alias                       [257]uint16
	Description                 [257]uint16
	PhysicalAddressLength       uint32
	PhysicalAddress             [32]byte
	PermanentPhysicalAddress    [32]byte
	Mtu                         uint32
	Type                        uint32
	TunnelType                  uint32
	MediaType                   uint32
	PhysicalMediumType          uint32
	AccessType                  uint32
	DirectionType               uint32
	InterfaceAndOperStatusFlags byte
	OperStatus                  uint32
	AdminStatus                 uint32
	MediaConnectState           uint32
	NetworkGuid                 windows.GUID
	ConnectionType              uint32
	TransmitLinkSpeed           uint64
	ReceiveLinkSpeed            uint64
	InOctets                    uint64
	InUcastPkts                 uint64
	InNUcastPkts                uint64
	InDiscards                  uint64
	InErrors                    uint64
	InUnknownProtos             uint64
	InUcastOctets               uint64
	InMulticastOctets           uint64
	InBroadcastOctets           uint64
	OutOctets                   uint64
	OutUcastPkts                uint64
	OutNUcastPkts               uint64
	OutDiscards                 uint64
	OutErrors                   uint64
	OutUcastOctets              uint64
	OutMulticastOctets          uint64
	OutBroadcastOctets          uint64
	OutQLen                     uint64
}

func ReadInterfaceCounters(adapters []interfaces.Adapter) ([]InterfaceCounter, error) {
	now := time.Now().UTC()
	result := make([]InterfaceCounter, 0, len(adapters))
	for _, adapter := range adapters {
		row := mibIfRow2{InterfaceLuid: adapter.LUID}
		status, _, _ := getIfEntry2.Call(uintptr(unsafe.Pointer(&row)))
		if status != 0 {
			return nil, fmt.Errorf("GetIfEntry2(%s): Windows error %d", adapter.FriendlyName, status)
		}
		result = append(result, InterfaceCounter{GUID: adapter.GUID, Name: adapter.FriendlyName, Received: row.InOctets, Transmitted: row.OutOctets, SampledAt: now})
	}
	return result, nil
}

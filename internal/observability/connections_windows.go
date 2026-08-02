//go:build windows

package observability

import (
	"fmt"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

var getExtendedTCPTable = iphlpapi.NewProc("GetExtendedTcpTable")
var getExtendedUDPTable = iphlpapi.NewProc("GetExtendedUdpTable")

const (
	afInet              = 2
	tcpTableOwnerPIDAll = 5
	udpTableOwnerPID    = 1
	tcpStateEstablished = 5
	tcpStateListen      = 2
)

func ReadConnectionSummary() (ConnectionSummary, error) {
	tcpRows, err := readIPTable(getExtendedTCPTable, afInet, tcpTableOwnerPIDAll, 24)
	if err != nil {
		return ConnectionSummary{}, fmt.Errorf("read IPv4 TCP table: %w", err)
	}
	udpRows, err := readIPTable(getExtendedUDPTable, afInet, udpTableOwnerPID, 12)
	if err != nil {
		return ConnectionSummary{}, fmt.Errorf("read IPv4 UDP table: %w", err)
	}
	result := ConnectionSummary{ActiveTCP: len(tcpRows), UDPEndpoints: len(udpRows), SampledAt: time.Now().UTC()}
	for _, row := range tcpRows {
		state := *(*uint32)(unsafe.Pointer(&row[0]))
		if state == tcpStateEstablished {
			result.EstablishedTCP++
		}
		if state == tcpStateListen {
			result.ListeningTCP++
		}
	}
	return result, nil
}

func readIPTable(proc *windows.LazyProc, family, class, rowSize uint32) ([][]byte, error) {
	var size uint32
	status, _, _ := proc.Call(0, uintptr(unsafe.Pointer(&size)), 0, uintptr(family), uintptr(class), 0)
	if status != uintptr(windows.ERROR_INSUFFICIENT_BUFFER) {
		return nil, windows.Errno(status)
	}
	buffer := make([]byte, size)
	status, _, _ = proc.Call(uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), 0, uintptr(family), uintptr(class), 0)
	if status != 0 {
		return nil, windows.Errno(status)
	}
	count := *(*uint32)(unsafe.Pointer(&buffer[0]))
	rows := make([][]byte, 0, count)
	for offset, index := uint32(4), uint32(0); index < count && offset+rowSize <= size; offset, index = offset+rowSize, index+1 {
		rows = append(rows, buffer[offset:offset+rowSize])
	}
	if uint32(len(rows)) != count {
		return nil, fmt.Errorf("table declared %d rows but contained %d", count, len(rows))
	}
	return rows, nil
}

//go:build !windows

package observability

import "winrouter/internal/interfaces"

func ReadInterfaceCounters([]interfaces.Adapter) ([]InterfaceCounter, error) {
	return []InterfaceCounter{}, nil
}

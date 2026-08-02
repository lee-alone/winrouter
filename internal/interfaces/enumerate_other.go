//go:build !windows

package interfaces

import "errors"

func Enumerate() ([]Adapter, error) {
	return nil, errors.New("interface enumeration is supported only on Windows")
}

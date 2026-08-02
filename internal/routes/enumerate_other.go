//go:build !windows

package routes

import "errors"

func Enumerate() ([]Route, error) {
	return nil, errors.New("route enumeration is supported only on Windows")
}

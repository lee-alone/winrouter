//go:build !windows

package nodes

import "errors"

type DPAPIProtector struct{}

func (DPAPIProtector) Protect([]byte) ([]byte, error) {
	return nil, errors.New("DPAPI is only available on Windows")
}
func (DPAPIProtector) Unprotect([]byte) ([]byte, error) {
	return nil, errors.New("DPAPI is only available on Windows")
}

//go:build windows

package nodes

import (
	"errors"
	"unsafe"

	"golang.org/x/sys/windows"
)

type DPAPIProtector struct{}

func (DPAPIProtector) Protect(plaintext []byte) ([]byte, error) {
	return cryptProtect(plaintext, true)
}

func (DPAPIProtector) Unprotect(ciphertext []byte) ([]byte, error) {
	return cryptProtect(ciphertext, false)
}

func cryptProtect(input []byte, protect bool) ([]byte, error) {
	if len(input) == 0 {
		return []byte{}, nil
	}
	in := windows.DataBlob{Size: uint32(len(input)), Data: &input[0]}
	var out windows.DataBlob
	var err error
	if protect {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, err
	}
	if out.Data == nil {
		return nil, errors.New("DPAPI returned empty output")
	}
	defer windows.LocalFree(windows.Handle(unsafe.Pointer(out.Data)))
	return append([]byte(nil), unsafe.Slice(out.Data, out.Size)...), nil
}

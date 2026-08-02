//go:build windows

package singleinstance

import (
	"errors"
	"fmt"

	"golang.org/x/sys/windows"
)

const mutexName = `Local\WinRouter.Desktop.v1`

var ErrAlreadyRunning = errors.New("WinRouter is already running")

type Lock struct {
	handle windows.Handle
}

func Acquire() (*Lock, error) {
	name, err := windows.UTF16PtrFromString(mutexName)
	if err != nil {
		return nil, fmt.Errorf("encode single-instance mutex name: %w", err)
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		return nil, ErrAlreadyRunning
	}
	if err != nil {
		return nil, fmt.Errorf("create single-instance mutex: %w", err)
	}
	return &Lock{handle: handle}, nil
}

func (lock *Lock) Close() error {
	if lock == nil || lock.handle == 0 {
		return nil
	}
	err := windows.CloseHandle(lock.handle)
	lock.handle = 0
	return err
}

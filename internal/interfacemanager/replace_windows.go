//go:build windows

package interfacemanager

import (
	"time"

	"golang.org/x/sys/windows"
)

func replaceStateFile(source, destination string) error {
	sourcePtr, err := windows.UTF16PtrFromString(source)
	if err != nil {
		return err
	}
	destinationPtr, err := windows.UTF16PtrFromString(destination)
	if err != nil {
		return err
	}
	var lastErr error
	for attempt := 0; attempt < 5; attempt++ {
		lastErr = windows.MoveFileEx(sourcePtr, destinationPtr, windows.MOVEFILE_REPLACE_EXISTING|windows.MOVEFILE_WRITE_THROUGH)
		if lastErr == nil {
			return nil
		}
		time.Sleep(25 * time.Millisecond)
	}
	return lastErr
}

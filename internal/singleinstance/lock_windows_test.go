//go:build windows

package singleinstance

import (
	"errors"
	"testing"
)

func TestAcquireRejectsSecondInstanceAndReleasesLock(t *testing.T) {
	first, err := Acquire()
	if err != nil {
		t.Fatal(err)
	}
	second, err := Acquire()
	if second != nil || !errors.Is(err, ErrAlreadyRunning) {
		first.Close()
		t.Fatalf("second acquire = %#v, %v", second, err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	third, err := Acquire()
	if err != nil {
		t.Fatalf("acquire after release: %v", err)
	}
	if err := third.Close(); err != nil {
		t.Fatal(err)
	}
}

//go:build windows

package tray

import (
	"testing"
	"time"
)

func TestRegisterLeftTap(t *testing.T) {
	var controller Controller
	start := time.Unix(100, 0)
	interval := 500 * time.Millisecond

	if controller.registerLeftTap(start, interval) {
		t.Fatal("first tap must not open the window")
	}
	if !controller.registerLeftTap(start.Add(300*time.Millisecond), interval) {
		t.Fatal("second tap within the interval must open the window")
	}
	if controller.registerLeftTap(start.Add(600*time.Millisecond), interval) {
		t.Fatal("tap after a completed double click must start a new pair")
	}
	if controller.registerLeftTap(start.Add(1200*time.Millisecond), interval) {
		t.Fatal("tap outside the interval must not open the window")
	}
}

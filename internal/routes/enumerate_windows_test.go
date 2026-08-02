//go:build windows

package routes

import (
	"net/netip"
	"testing"
)

func TestEnumerateReturnsValidPrefixes(t *testing.T) {
	items, err := Enumerate()
	if err != nil {
		t.Fatalf("Enumerate() error: %v", err)
	}
	if len(items) == 0 {
		t.Fatal("Enumerate() returned no routes")
	}
	for _, item := range items {
		if _, err := netip.ParsePrefix(item.Prefix); err != nil {
			t.Errorf("invalid route prefix %q: %v", item.Prefix, err)
		}
		if item.InterfaceIndex == 0 {
			t.Errorf("route %q has zero interface index", item.Prefix)
		}
	}
}

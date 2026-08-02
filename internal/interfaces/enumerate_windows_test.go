//go:build windows

package interfaces

import (
	"strings"
	"testing"
)

func TestEnumerateReturnsStableIdentifiers(t *testing.T) {
	adapters, err := Enumerate()
	if err != nil {
		t.Fatalf("Enumerate() error: %v", err)
	}
	if len(adapters) == 0 {
		t.Fatal("Enumerate() returned no adapters")
	}
	for _, adapter := range adapters {
		if !strings.HasPrefix(adapter.GUID, "{") || !strings.HasSuffix(adapter.GUID, "}") {
			t.Errorf("adapter %q has malformed GUID %q", adapter.FriendlyName, adapter.GUID)
		}
		if adapter.LUID == 0 {
			t.Errorf("adapter %q has zero LUID", adapter.FriendlyName)
		}
		if adapter.Index == 0 {
			t.Errorf("adapter %q has zero interface index", adapter.FriendlyName)
		}
	}
}

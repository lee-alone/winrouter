//go:build windows

package observability

import (
	"testing"

	"winrouter/internal/interfaces"
)

func TestReadInterfaceCountersFromWindows(t *testing.T) {
	adapters, err := interfaces.Enumerate()
	if err != nil {
		t.Fatal(err)
	}
	counters, err := ReadInterfaceCounters(adapters)
	if err != nil {
		t.Fatal(err)
	}
	if len(counters) != len(adapters) {
		t.Fatalf("got %d counters for %d adapters", len(counters), len(adapters))
	}
}

func TestReadConnectionSummaryFromWindows(t *testing.T) {
	summary, err := ReadConnectionSummary()
	if err != nil {
		t.Fatal(err)
	}
	if summary.SampledAt.IsZero() || summary.ActiveTCP < summary.EstablishedTCP+summary.ListeningTCP {
		t.Fatalf("invalid connection summary: %#v", summary)
	}
}

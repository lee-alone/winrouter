//go:build !windows

package observability

import "time"

func ReadConnectionSummary() (ConnectionSummary, error) {
	return ConnectionSummary{SampledAt: time.Now().UTC()}, nil
}

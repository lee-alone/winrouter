//go:build !windows

package core

import (
	"context"
	"errors"
	"time"
)

type TUNExperiment struct{}

func RunTUNExperiment(context.Context, string, []byte, string, string, time.Duration, []string, []string) (TUNExperiment, error) {
	return TUNExperiment{}, errors.New("TUN experiment is supported only on Windows")
}

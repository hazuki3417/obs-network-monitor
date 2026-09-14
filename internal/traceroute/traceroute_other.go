//go:build !windows

package traceroute

import (
	"context"
	"errors"
	"time"
)

type unsupportedHopRunner struct{}

func newSystemHopRunner() HopRunner {
	return unsupportedHopRunner{}
}

func (unsupportedHopRunner) Probe(context.Context, string, int, time.Duration) (HopResponse, error) {
	return HopResponse{}, errors.New("traceroute is only supported on Windows")
}

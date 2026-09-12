//go:build !windows

package probe

import (
	"context"
	"errors"
	"time"
)

type unsupportedICMPProbe struct{}

func newSystemICMPProbe() Runner {
	return unsupportedICMPProbe{}
}

func (unsupportedICMPProbe) Probe(context.Context, string, time.Duration) (time.Duration, error) {
	return 0, errors.New("system ICMP probe is only supported on Windows")
}

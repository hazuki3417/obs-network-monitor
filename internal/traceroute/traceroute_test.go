package traceroute

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

type fakeHopRunner struct {
	responses []HopResponse
	errAt     int
	ttls      []int
}

func (runner *fakeHopRunner) Probe(
	ctx context.Context,
	_ string,
	ttl int,
	_ time.Duration,
) (HopResponse, error) {
	if err := ctx.Err(); err != nil {
		return HopResponse{}, err
	}
	runner.ttls = append(runner.ttls, ttl)
	if runner.errAt == ttl {
		return HopResponse{}, errors.New("probe failed")
	}
	if ttl <= len(runner.responses) {
		return runner.responses[ttl-1], nil
	}
	return HopResponse{}, nil
}

func TestTraceStopsWhenTargetIsReached(t *testing.T) {
	runner := &fakeHopRunner{responses: []HopResponse{
		{Address: "192.0.2.1", Responded: true, RTT: time.Millisecond},
		{},
		{Address: "8.8.8.8", Responded: true, RTT: 10 * time.Millisecond, Target: true},
	}}
	checkedAt := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	engine := newEngine("8.8.8.8", 30, time.Second, runner, func() time.Time { return checkedAt })

	result, err := engine.Trace(context.Background())
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}
	if !result.Complete || len(result.Hops) != 3 || !result.Hops[2].Target {
		t.Fatalf("Trace() = %#v", result)
	}
	if !reflect.DeepEqual(runner.ttls, []int{1, 2, 3}) {
		t.Fatalf("TTLs = %v", runner.ttls)
	}
	if result.CheckedAt != checkedAt {
		t.Fatalf("CheckedAt = %v, want %v", result.CheckedAt, checkedAt)
	}
}

func TestTraceReturnsIncompleteRouteAtMaxHops(t *testing.T) {
	runner := &fakeHopRunner{}
	engine := newEngine("8.8.8.8", 3, time.Second, runner, time.Now)

	result, err := engine.Trace(context.Background())
	if err != nil {
		t.Fatalf("Trace() error = %v", err)
	}
	if result.Complete || len(result.Hops) != 3 {
		t.Fatalf("Trace() = %#v", result)
	}
}

func TestTraceReturnsPartialRouteOnProbeError(t *testing.T) {
	runner := &fakeHopRunner{responses: []HopResponse{{Responded: true}}, errAt: 2}
	engine := newEngine("8.8.8.8", 5, time.Second, runner, time.Now)

	result, err := engine.Trace(context.Background())
	if err == nil || len(result.Hops) != 1 {
		t.Fatalf("Trace() result = %#v, error = %v", result, err)
	}
}

func TestTraceHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &fakeHopRunner{}
	engine := newEngine("8.8.8.8", 5, time.Second, runner, time.Now)

	if _, err := engine.Trace(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("Trace() error = %v, want context canceled", err)
	}
	if len(runner.ttls) != 0 {
		t.Fatalf("TTLs = %v, want no probes", runner.ttls)
	}
}

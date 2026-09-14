package traceroute

import (
	"context"
	"fmt"
	"time"
)

const (
	DefaultMaxHops = 30
	DefaultTimeout = time.Second
)

type Hop struct {
	Number    int
	Address   string
	Responded bool
	RTT       time.Duration
	Target    bool
}

type Result struct {
	Target    string
	Hops      []Hop
	Complete  bool
	CheckedAt time.Time
}

type HopResponse struct {
	Address   string
	Responded bool
	RTT       time.Duration
	Target    bool
}

type HopRunner interface {
	Probe(context.Context, string, int, time.Duration) (HopResponse, error)
}

type Tracer interface {
	Trace(context.Context) (Result, error)
}

type Engine struct {
	target  string
	maxHops int
	timeout time.Duration
	runner  HopRunner
	now     func() time.Time
}

func New(target string) *Engine {
	return newEngine(target, DefaultMaxHops, DefaultTimeout, newSystemHopRunner(), time.Now)
}

func newEngine(target string, maxHops int, timeout time.Duration, runner HopRunner, now func() time.Time) *Engine {
	if maxHops <= 0 {
		maxHops = DefaultMaxHops
	}
	if timeout <= 0 {
		timeout = DefaultTimeout
	}
	return &Engine{target: target, maxHops: maxHops, timeout: timeout, runner: runner, now: now}
}

func (engine *Engine) Trace(ctx context.Context) (Result, error) {
	result := Result{Target: engine.target}
	for ttl := 1; ttl <= engine.maxHops; ttl++ {
		if err := ctx.Err(); err != nil {
			result.CheckedAt = engine.now()
			return result, err
		}

		response, err := engine.runner.Probe(ctx, engine.target, ttl, engine.timeout)
		if err != nil {
			result.CheckedAt = engine.now()
			return result, fmt.Errorf("probe hop %d: %w", ttl, err)
		}
		result.Hops = append(result.Hops, Hop{
			Number:    ttl,
			Address:   response.Address,
			Responded: response.Responded,
			RTT:       response.RTT,
			Target:    response.Target,
		})
		if response.Target {
			result.Complete = true
			break
		}
	}
	result.CheckedAt = engine.now()
	return result, nil
}

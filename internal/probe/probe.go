package probe

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

type Method string

const (
	MethodICMP Method = "icmp"
	MethodHTTP Method = "http"
)

const (
	DefaultTimeout            = time.Second
	DefaultICMPFailureLimit   = 3
	DefaultICMPRecoveryPeriod = 30 * time.Second
)

type Result struct {
	Method    Method    `json:"method"`
	Target    string    `json:"target"`
	Success   bool      `json:"success"`
	RTTMillis int64     `json:"rttMs"`
	CheckedAt time.Time `json:"checkedAt"`
}

type Runner interface {
	Probe(ctx context.Context, target string, timeout time.Duration) (time.Duration, error)
}

type Measurer interface {
	Measure(ctx context.Context) (Result, error)
}

type Options struct {
	ICMPTarget          string
	HTTPTarget          string
	Timeout             time.Duration
	ICMPFailureLimit    int
	ICMPRecoveryPeriod time.Duration
}

type Engine struct {
	mu sync.Mutex

	options Options
	icmp    Runner
	http    Runner
	now     func() time.Time

	method                  Method
	consecutiveICMPFailures int
	nextICMPRecoveryCheck   time.Time
}

func NewEngine(icmpTarget, httpTarget string) *Engine {
	return newEngine(Options{
		ICMPTarget:         icmpTarget,
		HTTPTarget:         httpTarget,
		Timeout:            DefaultTimeout,
		ICMPFailureLimit:   DefaultICMPFailureLimit,
		ICMPRecoveryPeriod: DefaultICMPRecoveryPeriod,
	}, newSystemICMPProbe(), newHTTPProbe(), time.Now)
}

func newEngine(options Options, icmp, http Runner, now func() time.Time) *Engine {
	return &Engine{
		options: options,
		icmp:    icmp,
		http:    http,
		now:     now,
		method:  MethodICMP,
	}
}

func (engine *Engine) Measure(ctx context.Context) (Result, error) {
	engine.mu.Lock()
	defer engine.mu.Unlock()

	if engine.method == MethodHTTP {
		return engine.measureHTTP(ctx)
	}
	return engine.measureICMP(ctx)
}

func (engine *Engine) measureICMP(ctx context.Context) (Result, error) {
	result, icmpErr := engine.run(ctx, MethodICMP, engine.options.ICMPTarget, engine.icmp)
	if icmpErr == nil {
		engine.consecutiveICMPFailures = 0
		return result, nil
	}

	engine.consecutiveICMPFailures++
	if engine.consecutiveICMPFailures < engine.options.ICMPFailureLimit {
		return result, fmt.Errorf("ICMP probe: %w", icmpErr)
	}

	engine.consecutiveICMPFailures = 0
	httpResult, httpErr := engine.run(ctx, MethodHTTP, engine.options.HTTPTarget, engine.http)
	if httpErr != nil {
		return result, errors.Join(
			fmt.Errorf("ICMP probe: %w", icmpErr),
			fmt.Errorf("HTTP fallback probe: %w", httpErr),
		)
	}

	engine.method = MethodHTTP
	engine.nextICMPRecoveryCheck = httpResult.CheckedAt.Add(engine.options.ICMPRecoveryPeriod)
	return httpResult, nil
}

func (engine *Engine) measureHTTP(ctx context.Context) (Result, error) {
	now := engine.now()
	var recoveryErr error
	if !now.Before(engine.nextICMPRecoveryCheck) {
		icmpResult, err := engine.run(ctx, MethodICMP, engine.options.ICMPTarget, engine.icmp)
		if err == nil {
			engine.method = MethodICMP
			engine.consecutiveICMPFailures = 0
			engine.nextICMPRecoveryCheck = time.Time{}
			return icmpResult, nil
		}
		recoveryErr = fmt.Errorf("ICMP recovery probe: %w", err)
		engine.nextICMPRecoveryCheck = engine.now().Add(engine.options.ICMPRecoveryPeriod)
	}

	httpResult, httpErr := engine.run(ctx, MethodHTTP, engine.options.HTTPTarget, engine.http)
	if httpErr != nil {
		return httpResult, errors.Join(recoveryErr, fmt.Errorf("HTTP probe: %w", httpErr))
	}
	return httpResult, nil
}

func (engine *Engine) run(ctx context.Context, method Method, target string, runner Runner) (Result, error) {
	result := Result{
		Method:    method,
		Target:    target,
		CheckedAt: engine.now(),
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}

	rtt, err := runner.Probe(ctx, target, engine.options.Timeout)
	result.CheckedAt = engine.now()
	if err != nil {
		return result, err
	}

	result.Success = true
	result.RTTMillis = rtt.Milliseconds()
	return result, nil
}

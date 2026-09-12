package probe

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

type probeResponse struct {
	rtt time.Duration
	err error
}

type fakeRunner struct {
	responses []probeResponse
	targets   []string
	timeouts  []time.Duration
}

func (runner *fakeRunner) Probe(_ context.Context, target string, timeout time.Duration) (time.Duration, error) {
	runner.targets = append(runner.targets, target)
	runner.timeouts = append(runner.timeouts, timeout)
	if len(runner.responses) == 0 {
		return 0, errors.New("unexpected probe call")
	}
	response := runner.responses[0]
	runner.responses = runner.responses[1:]
	return response.rtt, response.err
}

type manualClock struct {
	now time.Time
}

func (clock *manualClock) Now() time.Time {
	return clock.now
}

func (clock *manualClock) Advance(duration time.Duration) {
	clock.now = clock.now.Add(duration)
}

func testOptions() Options {
	return Options{
		ICMPTarget:         "8.8.8.8",
		HTTPTarget:         "https://www.google.com/generate_204",
		Timeout:            time.Second,
		ICMPFailureLimit:   3,
		ICMPRecoveryPeriod: 30 * time.Second,
	}
}

func TestEngineStartsWithICMP(t *testing.T) {
	icmp := &fakeRunner{responses: []probeResponse{{rtt: 12 * time.Millisecond}}}
	http := &fakeRunner{}
	clock := &manualClock{now: time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)}
	engine := newEngine(testOptions(), icmp, http, clock.Now)

	result, err := engine.Measure(context.Background())
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}
	if result.Method != MethodICMP || result.Target != "8.8.8.8" || !result.Success || result.RTTMillis != 12 {
		t.Fatalf("Measure() = %#v", result)
	}
	if len(http.targets) != 0 {
		t.Fatalf("HTTP calls = %d, want 0", len(http.targets))
	}
}

func TestEngineWaitsForFailureLimitBeforeHTTPConfirmation(t *testing.T) {
	icmp := &fakeRunner{responses: []probeResponse{
		{err: errors.New("timeout 1")},
		{err: errors.New("timeout 2")},
	}}
	http := &fakeRunner{}
	engine := newEngine(testOptions(), icmp, http, time.Now)

	for attempt := 1; attempt <= 2; attempt++ {
		result, err := engine.Measure(context.Background())
		if err == nil || result.Method != MethodICMP || result.Success {
			t.Fatalf("attempt %d: result = %#v, error = %v", attempt, result, err)
		}
	}
	if len(http.targets) != 0 {
		t.Fatalf("HTTP calls = %d, want 0", len(http.targets))
	}
}

func TestEngineSwitchesToHTTPWhenOnlyHTTPSucceeds(t *testing.T) {
	icmp := &fakeRunner{responses: []probeResponse{
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
	}}
	http := &fakeRunner{responses: []probeResponse{{rtt: 45 * time.Millisecond}}}
	clock := &manualClock{now: time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)}
	engine := newEngine(testOptions(), icmp, http, clock.Now)

	for attempt := 1; attempt <= 2; attempt++ {
		_, _ = engine.Measure(context.Background())
	}
	result, err := engine.Measure(context.Background())
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}
	if result.Method != MethodHTTP || !result.Success || result.RTTMillis != 45 {
		t.Fatalf("Measure() = %#v", result)
	}
	if engine.method != MethodHTTP {
		t.Fatalf("method = %q, want %q", engine.method, MethodHTTP)
	}
	if !engine.nextICMPRecoveryCheck.Equal(clock.now.Add(30 * time.Second)) {
		t.Fatalf("next recovery = %v", engine.nextICMPRecoveryCheck)
	}
}

func TestEngineStaysOnICMPWhenBothProbesFail(t *testing.T) {
	icmp := &fakeRunner{responses: []probeResponse{
		{err: errors.New("timeout 1")},
		{err: errors.New("timeout 2")},
		{err: errors.New("timeout 3")},
		{err: errors.New("timeout 4")},
	}}
	http := &fakeRunner{responses: []probeResponse{{err: errors.New("HTTP offline")}}}
	engine := newEngine(testOptions(), icmp, http, time.Now)

	_, _ = engine.Measure(context.Background())
	_, _ = engine.Measure(context.Background())
	result, err := engine.Measure(context.Background())
	if err == nil || !strings.Contains(err.Error(), "HTTP offline") {
		t.Fatalf("Measure() error = %v", err)
	}
	if result.Method != MethodICMP || engine.method != MethodICMP {
		t.Fatalf("result method = %q, engine method = %q", result.Method, engine.method)
	}

	_, _ = engine.Measure(context.Background())
	if len(http.targets) != 1 {
		t.Fatalf("HTTP calls = %d, want 1 after failure counter reset", len(http.targets))
	}
}

func TestEngineKeepsUsingHTTPBeforeRecoveryPeriod(t *testing.T) {
	icmp := &fakeRunner{responses: []probeResponse{
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
	}}
	http := &fakeRunner{responses: []probeResponse{
		{rtt: 40 * time.Millisecond},
		{rtt: 42 * time.Millisecond},
	}}
	clock := &manualClock{now: time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)}
	engine := newEngine(testOptions(), icmp, http, clock.Now)

	_, _ = engine.Measure(context.Background())
	_, _ = engine.Measure(context.Background())
	_, _ = engine.Measure(context.Background())
	clock.Advance(29 * time.Second)
	result, err := engine.Measure(context.Background())
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}
	if result.Method != MethodHTTP || result.RTTMillis != 42 {
		t.Fatalf("Measure() = %#v", result)
	}
	if len(icmp.targets) != 3 {
		t.Fatalf("ICMP calls = %d, want 3", len(icmp.targets))
	}
}

func TestEngineReturnsToICMPAfterRecovery(t *testing.T) {
	icmp := &fakeRunner{responses: []probeResponse{
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
		{rtt: 9 * time.Millisecond},
	}}
	http := &fakeRunner{responses: []probeResponse{{rtt: 40 * time.Millisecond}}}
	clock := &manualClock{now: time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)}
	engine := newEngine(testOptions(), icmp, http, clock.Now)

	_, _ = engine.Measure(context.Background())
	_, _ = engine.Measure(context.Background())
	_, _ = engine.Measure(context.Background())
	clock.Advance(30 * time.Second)
	result, err := engine.Measure(context.Background())
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}
	if result.Method != MethodICMP || result.RTTMillis != 9 || engine.method != MethodICMP {
		t.Fatalf("Measure() = %#v, engine method = %q", result, engine.method)
	}
	if len(http.targets) != 1 {
		t.Fatalf("HTTP calls = %d, want no HTTP call after ICMP recovery", len(http.targets))
	}
}

func TestEngineContinuesHTTPWhenRecoveryFails(t *testing.T) {
	icmp := &fakeRunner{responses: []probeResponse{
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
		{err: errors.New("still blocked")},
	}}
	http := &fakeRunner{responses: []probeResponse{
		{rtt: 40 * time.Millisecond},
		{rtt: 41 * time.Millisecond},
	}}
	clock := &manualClock{now: time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)}
	engine := newEngine(testOptions(), icmp, http, clock.Now)

	_, _ = engine.Measure(context.Background())
	_, _ = engine.Measure(context.Background())
	_, _ = engine.Measure(context.Background())
	clock.Advance(30 * time.Second)
	result, err := engine.Measure(context.Background())
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}
	if result.Method != MethodHTTP || result.RTTMillis != 41 || engine.method != MethodHTTP {
		t.Fatalf("Measure() = %#v, engine method = %q", result, engine.method)
	}
	if !engine.nextICMPRecoveryCheck.Equal(clock.now.Add(30 * time.Second)) {
		t.Fatalf("next recovery = %v", engine.nextICMPRecoveryCheck)
	}
}

func TestEngineReturnsBothErrorsWhenRecoveryAndHTTPFail(t *testing.T) {
	icmp := &fakeRunner{responses: []probeResponse{
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
		{err: errors.New("timeout")},
		{err: errors.New("ICMP blocked")},
	}}
	http := &fakeRunner{responses: []probeResponse{
		{rtt: 40 * time.Millisecond},
		{err: errors.New("HTTP offline")},
	}}
	clock := &manualClock{now: time.Date(2026, 9, 12, 9, 0, 0, 0, time.UTC)}
	engine := newEngine(testOptions(), icmp, http, clock.Now)

	_, _ = engine.Measure(context.Background())
	_, _ = engine.Measure(context.Background())
	_, _ = engine.Measure(context.Background())
	clock.Advance(30 * time.Second)
	result, err := engine.Measure(context.Background())
	if err == nil || !strings.Contains(err.Error(), "ICMP blocked") || !strings.Contains(err.Error(), "HTTP offline") {
		t.Fatalf("Measure() error = %v", err)
	}
	if result.Method != MethodHTTP || result.Success {
		t.Fatalf("Measure() = %#v", result)
	}
}

func TestEngineHonorsCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	engine := newEngine(testOptions(), &fakeRunner{}, &fakeRunner{}, time.Now)

	result, err := engine.Measure(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Measure() error = %v, want context.Canceled", err)
	}
	if result.Method != MethodICMP || result.Success {
		t.Fatalf("Measure() = %#v", result)
	}
}

func TestEnginePassesConfiguredTargetsAndTimeout(t *testing.T) {
	icmp := &fakeRunner{responses: []probeResponse{{rtt: time.Millisecond}}}
	options := testOptions()
	options.ICMPTarget = "1.1.1.1"
	options.Timeout = 750 * time.Millisecond
	engine := newEngine(options, icmp, &fakeRunner{}, time.Now)

	_, err := engine.Measure(context.Background())
	if err != nil {
		t.Fatalf("Measure() error = %v", err)
	}
	if len(icmp.targets) != 1 || icmp.targets[0] != "1.1.1.1" || icmp.timeouts[0] != 750*time.Millisecond {
		t.Fatalf("targets = %v, timeouts = %v", icmp.targets, icmp.timeouts)
	}
}

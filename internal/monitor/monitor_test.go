package monitor

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"obs-network-monitor/internal/adapter"
	"obs-network-monitor/internal/probe"
)

func sample(method probe.Method, success bool, rtt int64) probe.Result {
	return probe.Result{
		Method:    method,
		Target:    map[probe.Method]string{probe.MethodICMP: "8.8.8.8", probe.MethodHTTP: "https://example.test"}[method],
		Success:   success,
		RTTMillis: rtt,
		CheckedAt: time.Date(2026, 9, 12, 10, 0, 0, 0, time.UTC),
	}
}

func TestHistoryWindowKeepsLatestSixtySamples(t *testing.T) {
	window := historyWindow{limit: 60}
	for index := 0; index < 65; index++ {
		window.Add(sample(probe.MethodICMP, true, int64(index)))
	}

	samples := window.Samples()
	if len(samples) != 60 {
		t.Fatalf("sample count = %d, want 60", len(samples))
	}
	if samples[0].RTTMillis != 5 || samples[59].RTTMillis != 64 {
		t.Fatalf("RTT range = %d...%d, want 5...64", samples[0].RTTMillis, samples[59].RTTMillis)
	}
}

func TestStatisticsSkipFailuresBetweenJitterPairs(t *testing.T) {
	window := historyWindow{limit: 60}
	window.Add(sample(probe.MethodICMP, true, 10))
	window.Add(sample(probe.MethodICMP, true, 14))
	window.Add(sample(probe.MethodICMP, false, 0))
	window.Add(sample(probe.MethodICMP, true, 30))
	window.Add(sample(probe.MethodICMP, true, 36))

	statistics := window.Statistics()
	if statistics.LatestLatencyMS == nil || *statistics.LatestLatencyMS != 36 {
		t.Fatalf("latest latency = %v, want 36", statistics.LatestLatencyMS)
	}
	if statistics.JitterMS == nil || *statistics.JitterMS != 5 {
		t.Fatalf("jitter = %v, want 5", statistics.JitterMS)
	}
	if statistics.FailureMetric != FailureMetricPacketLoss || statistics.FailureRatePercent != 20 {
		t.Fatalf("failure metric = %q, rate = %v", statistics.FailureMetric, statistics.FailureRatePercent)
	}
}

func TestStatisticsHaveNoLatencyOrJitterWithoutSuccess(t *testing.T) {
	window := historyWindow{limit: 60}
	window.Add(sample(probe.MethodICMP, false, 0))
	window.Add(sample(probe.MethodICMP, false, 0))

	statistics := window.Statistics()
	if statistics.LatestLatencyMS != nil || statistics.JitterMS != nil {
		t.Fatalf("latency = %v, jitter = %v, want nil", statistics.LatestLatencyMS, statistics.JitterMS)
	}
	if statistics.FailureRatePercent != 100 {
		t.Fatalf("failure rate = %v, want 100", statistics.FailureRatePercent)
	}
}

func TestHistoryWindowResetsWhenMethodChanges(t *testing.T) {
	window := historyWindow{limit: 60}
	window.Add(sample(probe.MethodICMP, true, 10))
	window.Add(sample(probe.MethodICMP, false, 0))
	window.Add(sample(probe.MethodHTTP, true, 40))

	statistics := window.Statistics()
	if len(window.Samples()) != 1 {
		t.Fatalf("sample count = %d, want 1", len(window.Samples()))
	}
	if statistics.Method != probe.MethodHTTP || statistics.FailureMetric != FailureMetricRequestFailure {
		t.Fatalf("statistics = %#v", statistics)
	}
	if statistics.FailureRatePercent != 0 {
		t.Fatalf("failure rate = %v, want 0", statistics.FailureRatePercent)
	}
}

func TestSnapshotJSONContainsNullableStatisticsAndHistory(t *testing.T) {
	snapshot := Snapshot{
		NIC: adapter.Info{Name: "Ethernet", State: adapter.StateConnected},
		Statistics: Statistics{
			Method:             probe.MethodICMP,
			Target:             "8.8.8.8",
			FailureMetric:      FailureMetricPacketLoss,
			FailureRatePercent: 100,
		},
		History:     []probe.Result{sample(probe.MethodICMP, false, 0)},
		GeneratedAt: time.Date(2026, 9, 12, 10, 0, 1, 0, time.UTC),
	}

	payload, err := json.Marshal(snapshot)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	statistics := decoded["statistics"].(map[string]any)
	if statistics["latestLatencyMs"] != nil || statistics["jitterMs"] != nil {
		t.Fatalf("statistics JSON = %s", payload)
	}
	history := decoded["history"].([]any)
	if len(history) != 1 || decoded["nic"] == nil || decoded["generatedAt"] == nil {
		t.Fatalf("snapshot JSON = %s", payload)
	}
}

type fakeProvider struct {
	calls int
	info  adapter.Info
	err   error
}

func (provider *fakeProvider) Inspect(context.Context, string) (adapter.Info, error) {
	provider.calls++
	return provider.info, provider.err
}

type fakeMeasurer struct {
	calls   int
	results []probe.Result
	errors  []error
}

func (measurer *fakeMeasurer) Measure(context.Context) (probe.Result, error) {
	index := measurer.calls
	measurer.calls++
	var err error
	if index < len(measurer.errors) {
		err = measurer.errors[index]
	}
	return measurer.results[index], err
}

func TestSubscribersShareOneCollection(t *testing.T) {
	provider := &fakeProvider{info: adapter.Info{Name: "Ethernet", State: adapter.StateConnected}}
	measurer := &fakeMeasurer{results: []probe.Result{sample(probe.MethodICMP, true, 12)}}
	monitor := newMonitor(provider, measurer, "8.8.8.8", time.Second, 60, nil, time.Now)
	first, cancelFirst := monitor.Subscribe()
	defer cancelFirst()
	second, cancelSecond := monitor.Subscribe()
	defer cancelSecond()

	if !monitor.collect(context.Background()) {
		t.Fatal("collect() = false")
	}
	firstSnapshot := <-first
	secondSnapshot := <-second
	if provider.calls != 1 || measurer.calls != 1 {
		t.Fatalf("provider calls = %d, measurer calls = %d, want 1 each", provider.calls, measurer.calls)
	}
	if firstSnapshot.GeneratedAt != secondSnapshot.GeneratedAt || firstSnapshot.Statistics.Method != probe.MethodICMP {
		t.Fatalf("snapshots differ: %#v %#v", firstSnapshot, secondSnapshot)
	}
}

func TestCanceledSubscriberDoesNotStopCollection(t *testing.T) {
	provider := &fakeProvider{err: errors.New("adapter unavailable"), info: adapter.Info{State: adapter.StateDisconnected}}
	measurer := &fakeMeasurer{results: []probe.Result{
		sample(probe.MethodICMP, false, 0),
		sample(probe.MethodICMP, true, 11),
	}}
	monitor := newMonitor(provider, measurer, "8.8.8.8", time.Second, 60, nil, time.Now)
	_, cancel := monitor.Subscribe()
	cancel()

	if !monitor.collect(context.Background()) || !monitor.collect(context.Background()) {
		t.Fatal("collection stopped after subscriber cancellation")
	}
	latest, ok := monitor.Latest()
	if !ok || len(latest.History) != 2 || measurer.calls != 2 {
		t.Fatalf("latest = %#v, ok = %v, calls = %d", latest, ok, measurer.calls)
	}
}

func TestCollectStopsWhenContextIsCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	provider := &fakeProvider{}
	measurer := &fakeMeasurer{results: []probe.Result{sample(probe.MethodICMP, true, 1)}}
	monitor := newMonitor(provider, measurer, "8.8.8.8", time.Second, 60, nil, time.Now)

	if monitor.collect(ctx) {
		t.Fatal("collect() = true, want false")
	}
	if measurer.calls != 0 {
		t.Fatalf("measurer calls = %d, want 0", measurer.calls)
	}
}

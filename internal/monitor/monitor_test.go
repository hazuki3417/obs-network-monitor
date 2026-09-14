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

func TestStatisticsUseLatestSixtySamplesFromRenderBuffer(t *testing.T) {
	window := historyWindow{limit: DefaultHistoryLimit}
	firstStatisticsIndex := DefaultHistoryLimit - StatisticsHistoryLimit
	for index := 0; index < DefaultHistoryLimit; index++ {
		window.Add(sample(probe.MethodICMP, index >= firstStatisticsIndex, int64(index)))
	}

	if len(window.Samples()) != DefaultHistoryLimit {
		t.Fatalf("sample count = %d, want %d", len(window.Samples()), DefaultHistoryLimit)
	}
	statistics := window.Statistics()
	if statistics.FailureRatePercent != 0 {
		t.Fatalf("failure rate = %v, want 0", statistics.FailureRatePercent)
	}
	if statistics.MinimumLatencyMS == nil || *statistics.MinimumLatencyMS != int64(firstStatisticsIndex) {
		t.Fatalf("minimum latency = %v, want %d", statistics.MinimumLatencyMS, firstStatisticsIndex)
	}
}

func TestHistoryWindowsRetainSixtyFiveSecondsAcrossShortIntervals(t *testing.T) {
	latency := historyWindow{limit: DefaultHistoryLimit, retention: DefaultHistoryRetention}
	traffic := trafficWindow{limit: DefaultHistoryLimit, retention: DefaultHistoryRetention}
	start := time.Date(2026, 9, 14, 12, 0, 0, 0, time.UTC)
	for index := 0; index < 150; index++ {
		checkedAt := start.Add(time.Duration(index) * 500 * time.Millisecond)
		result := sample(probe.MethodICMP, true, int64(index))
		result.CheckedAt = checkedAt
		latency.Add(result)
		traffic.Add(TrafficSample{CheckedAt: checkedAt}, false)
	}

	latencySamples := latency.Samples()
	trafficSamples := traffic.Samples()
	latencySpan := latencySamples[len(latencySamples)-1].CheckedAt.Sub(latencySamples[0].CheckedAt)
	trafficSpan := trafficSamples[len(trafficSamples)-1].CheckedAt.Sub(trafficSamples[0].CheckedAt)
	if latencySpan < DefaultHistoryRetention || trafficSpan < DefaultHistoryRetention {
		t.Fatalf("retained spans = %v and %v, want at least %v", latencySpan, trafficSpan, DefaultHistoryRetention)
	}
	if len(latencySamples) <= 64 || len(trafficSamples) <= 64 {
		t.Fatalf("sample counts = %d and %d, want more than 64", len(latencySamples), len(trafficSamples))
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
	if statistics.AverageLatencyMS == nil || *statistics.AverageLatencyMS != 22.5 {
		t.Fatalf("average latency = %v, want 22.5", statistics.AverageLatencyMS)
	}
	if statistics.MinimumLatencyMS == nil || *statistics.MinimumLatencyMS != 10 {
		t.Fatalf("minimum latency = %v, want 10", statistics.MinimumLatencyMS)
	}
	if statistics.MaximumLatencyMS == nil || *statistics.MaximumLatencyMS != 36 {
		t.Fatalf("maximum latency = %v, want 36", statistics.MaximumLatencyMS)
	}
	if statistics.ConsecutiveFailures != 0 {
		t.Fatalf("consecutive failures = %d, want 0", statistics.ConsecutiveFailures)
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
	if statistics.LatestLatencyMS != nil || statistics.AverageLatencyMS != nil ||
		statistics.MinimumLatencyMS != nil || statistics.MaximumLatencyMS != nil || statistics.JitterMS != nil {
		t.Fatalf("statistics = %#v, want nil latency values", statistics)
	}
	if statistics.FailureRatePercent != 100 {
		t.Fatalf("failure rate = %v, want 100", statistics.FailureRatePercent)
	}
	if statistics.ConsecutiveFailures != 2 {
		t.Fatalf("consecutive failures = %d, want 2", statistics.ConsecutiveFailures)
	}
}

func TestStatisticsCountOnlyTrailingFailures(t *testing.T) {
	window := historyWindow{limit: 60}
	window.Add(sample(probe.MethodICMP, false, 0))
	window.Add(sample(probe.MethodICMP, true, 20))
	window.Add(sample(probe.MethodICMP, false, 0))
	window.Add(sample(probe.MethodICMP, false, 0))

	statistics := window.Statistics()
	if statistics.ConsecutiveFailures != 2 {
		t.Fatalf("consecutive failures = %d, want 2", statistics.ConsecutiveFailures)
	}
}

func TestTrafficTrackerCalculatesRatesFromCounterDeltas(t *testing.T) {
	tracker := trafficTracker{}
	start := time.Date(2026, 9, 13, 3, 0, 0, 0, time.UTC)
	first := adapter.Info{
		InterfaceIndex: 4,
		InterfaceLUID:  40,
		State:          adapter.StateConnected,
		TransmitOctets: 1_000,
		ReceiveOctets:  2_000,
	}
	if traffic, reset := tracker.Sample(first, start, true); traffic.TransmitBPS != nil || traffic.ReceiveBPS != nil || reset {
		t.Fatalf("first sample = %#v, reset = %v; want empty baseline", traffic, reset)
	}

	second := first
	second.TransmitOctets += 250
	second.ReceiveOctets += 500
	traffic, reset := tracker.Sample(second, start.Add(2*time.Second), true)
	if reset || traffic.TransmitBPS == nil || *traffic.TransmitBPS != 1_000 {
		t.Fatalf("transmit traffic = %#v, reset = %v; want 1000 bps", traffic.TransmitBPS, reset)
	}
	if traffic.ReceiveBPS == nil || *traffic.ReceiveBPS != 2_000 {
		t.Fatalf("receive traffic = %#v, want 2000 bps", traffic.ReceiveBPS)
	}
}

func TestTrafficTrackerResetsForAdapterChangeAndCounterRollback(t *testing.T) {
	tracker := trafficTracker{}
	start := time.Date(2026, 9, 13, 3, 0, 0, 0, time.UTC)
	info := adapter.Info{InterfaceIndex: 4, InterfaceLUID: 40, State: adapter.StateConnected, TransmitOctets: 100, ReceiveOctets: 200}
	tracker.Sample(info, start, true)

	changed := info
	changed.InterfaceIndex = 8
	changed.InterfaceLUID = 80
	traffic, reset := tracker.Sample(changed, start.Add(time.Second), true)
	if traffic.TransmitBPS != nil || traffic.ReceiveBPS != nil || !reset {
		t.Fatalf("adapter change = %#v, reset = %v; want empty reset sample", traffic, reset)
	}

	rolledBack := changed
	rolledBack.TransmitOctets = 50
	rolledBack.ReceiveOctets = 50
	traffic, reset = tracker.Sample(rolledBack, start.Add(2*time.Second), true)
	if traffic.TransmitBPS != nil || traffic.ReceiveBPS != nil || reset {
		t.Fatalf("counter rollback = %#v, reset = %v; want empty sample", traffic, reset)
	}
}

func TestTrafficWindowKeepsLatestSamplesAndResets(t *testing.T) {
	window := trafficWindow{limit: 2}
	for index := 0; index < 3; index++ {
		value := float64(index)
		window.Add(TrafficSample{Traffic: Traffic{TransmitBPS: &value}}, false)
	}
	if samples := window.Samples(); len(samples) != 2 || *samples[0].TransmitBPS != 1 || *samples[1].TransmitBPS != 2 {
		t.Fatalf("traffic samples = %#v", samples)
	}
	window.Add(TrafficSample{}, true)
	if samples := window.Samples(); len(samples) != 1 || samples[0].TransmitBPS != nil {
		t.Fatalf("reset traffic samples = %#v", samples)
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
		NIC: adapter.Info{Name: "Ethernet", State: adapter.StateConnected, TransmitOctets: 123, ReceiveOctets: 456},
		Traffic: Traffic{},
		Statistics: Statistics{
			Method:             probe.MethodICMP,
			Target:             "8.8.8.8",
			FailureMetric:      FailureMetricPacketLoss,
			FailureRatePercent: 100,
		},
		History:        []probe.Result{sample(probe.MethodICMP, false, 0)},
		TrafficHistory: []TrafficSample{{CheckedAt: time.Date(2026, 9, 12, 10, 0, 1, 0, time.UTC)}},
		GeneratedAt:    time.Date(2026, 9, 12, 10, 0, 1, 0, time.UTC),
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
	if statistics["latestLatencyMs"] != nil || statistics["averageLatencyMs"] != nil ||
		statistics["minimumLatencyMs"] != nil || statistics["maximumLatencyMs"] != nil || statistics["jitterMs"] != nil {
		t.Fatalf("statistics JSON = %s", payload)
	}
	nic := decoded["nic"].(map[string]any)
	for _, key := range []string{"TransmitOctets", "ReceiveOctets", "transmitOctets", "receiveOctets"} {
		if _, exists := nic[key]; exists {
			t.Fatalf("private NIC counter %q leaked in JSON = %s", key, payload)
		}
	}
	traffic := decoded["traffic"].(map[string]any)
	if traffic["transmitBps"] != nil || traffic["receiveBps"] != nil {
		t.Fatalf("traffic JSON = %s", payload)
	}
	history := decoded["history"].([]any)
	trafficHistory := decoded["trafficHistory"].([]any)
	if len(history) != 1 || len(trafficHistory) != 1 || decoded["nic"] == nil || decoded["generatedAt"] == nil {
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

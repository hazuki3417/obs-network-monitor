package monitor

import (
	"context"
	"sync"
	"time"

	"obs-network-monitor/internal/adapter"
	"obs-network-monitor/internal/probe"
)

const (
	DefaultHistoryLimit = 60
	DefaultInterval     = time.Second
)

type FailureMetric string

const (
	FailureMetricPacketLoss     FailureMetric = "packetLoss"
	FailureMetricRequestFailure FailureMetric = "requestFailure"
)

type Statistics struct {
	Method             probe.Method  `json:"method"`
	Target             string        `json:"target"`
	LatestLatencyMS    *int64        `json:"latestLatencyMs"`
	JitterMS           *float64      `json:"jitterMs"`
	FailureMetric      FailureMetric `json:"failureMetric"`
	FailureRatePercent float64       `json:"failureRatePercent"`
}

type Snapshot struct {
	NIC         adapter.Info   `json:"nic"`
	Statistics  Statistics     `json:"statistics"`
	History     []probe.Result `json:"history"`
	GeneratedAt time.Time      `json:"generatedAt"`
}

type Logger interface {
	Printf(format string, values ...any)
}

type Monitor struct {
	provider      adapter.Provider
	measurer      probe.Measurer
	adapterTarget string
	interval      time.Duration
	logger        Logger
	now           func() time.Time

	mu          sync.Mutex
	history     historyWindow
	latest      Snapshot
	hasLatest   bool
	subscribers map[chan Snapshot]struct{}
}

func New(provider adapter.Provider, measurer probe.Measurer, adapterTarget string, logger Logger) *Monitor {
	return newMonitor(provider, measurer, adapterTarget, DefaultInterval, DefaultHistoryLimit, logger, time.Now)
}

func newMonitor(
	provider adapter.Provider,
	measurer probe.Measurer,
	adapterTarget string,
	interval time.Duration,
	historyLimit int,
	logger Logger,
	now func() time.Time,
) *Monitor {
	if interval <= 0 {
		interval = DefaultInterval
	}
	if historyLimit <= 0 {
		historyLimit = DefaultHistoryLimit
	}
	return &Monitor{
		provider:      provider,
		measurer:      measurer,
		adapterTarget: adapterTarget,
		interval:      interval,
		logger:        logger,
		now:           now,
		history:       historyWindow{limit: historyLimit},
		subscribers:   make(map[chan Snapshot]struct{}),
	}
}

func (monitor *Monitor) Run(ctx context.Context) {
	if !monitor.collect(ctx) {
		return
	}

	ticker := time.NewTicker(monitor.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !monitor.collect(ctx) {
				return
			}
		}
	}
}

func (monitor *Monitor) Subscribe() (<-chan Snapshot, func()) {
	updates := make(chan Snapshot, 1)

	monitor.mu.Lock()
	monitor.subscribers[updates] = struct{}{}
	if monitor.hasLatest {
		updates <- cloneSnapshot(monitor.latest)
	}
	monitor.mu.Unlock()

	var once sync.Once
	cancel := func() {
		once.Do(func() {
			monitor.mu.Lock()
			if _, exists := monitor.subscribers[updates]; exists {
				delete(monitor.subscribers, updates)
				close(updates)
			}
			monitor.mu.Unlock()
		})
	}
	return updates, cancel
}

func (monitor *Monitor) Latest() (Snapshot, bool) {
	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	return cloneSnapshot(monitor.latest), monitor.hasLatest
}

func (monitor *Monitor) collect(ctx context.Context) bool {
	nic, nicErr := monitor.provider.Inspect(ctx, monitor.adapterTarget)
	if ctx.Err() != nil {
		return false
	}
	if nicErr != nil && monitor.logger != nil {
		monitor.logger.Printf("inspect network adapter: %v", nicErr)
	}

	result, probeErr := monitor.measurer.Measure(ctx)
	if ctx.Err() != nil {
		return false
	}
	if probeErr != nil && monitor.logger != nil {
		monitor.logger.Printf("measure network: %v", probeErr)
	}

	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	monitor.history.Add(result)
	monitor.latest = Snapshot{
		NIC:         nic,
		Statistics:  monitor.history.Statistics(),
		History:     monitor.history.Samples(),
		GeneratedAt: monitor.now(),
	}
	monitor.hasLatest = true
	monitor.publishLocked(monitor.latest)
	return true
}

func (monitor *Monitor) publishLocked(snapshot Snapshot) {
	for updates := range monitor.subscribers {
		value := cloneSnapshot(snapshot)
		select {
		case updates <- value:
		default:
			select {
			case <-updates:
			default:
			}
			select {
			case updates <- value:
			default:
			}
		}
	}
}

func cloneSnapshot(snapshot Snapshot) Snapshot {
	result := snapshot
	result.History = append([]probe.Result(nil), snapshot.History...)
	return result
}

type historyWindow struct {
	limit   int
	samples []probe.Result
}

func (window *historyWindow) Add(result probe.Result) {
	if len(window.samples) > 0 && window.samples[len(window.samples)-1].Method != result.Method {
		window.samples = window.samples[:0]
	}

	if len(window.samples) < window.limit {
		window.samples = append(window.samples, result)
		return
	}
	copy(window.samples, window.samples[1:])
	window.samples[len(window.samples)-1] = result
}

func (window *historyWindow) Samples() []probe.Result {
	return append([]probe.Result(nil), window.samples...)
}

func (window *historyWindow) Statistics() Statistics {
	if len(window.samples) == 0 {
		return Statistics{}
	}

	last := window.samples[len(window.samples)-1]
	statistics := Statistics{Method: last.Method, Target: last.Target}
	if last.Method == probe.MethodHTTP {
		statistics.FailureMetric = FailureMetricRequestFailure
	} else {
		statistics.FailureMetric = FailureMetricPacketLoss
	}

	failed := 0
	jitterTotal := int64(0)
	jitterPairs := 0
	for index, sample := range window.samples {
		if !sample.Success {
			failed++
			continue
		}
		latency := sample.RTTMillis
		statistics.LatestLatencyMS = &latency
		if index == 0 || !window.samples[index-1].Success {
			continue
		}
		difference := sample.RTTMillis - window.samples[index-1].RTTMillis
		if difference < 0 {
			difference = -difference
		}
		jitterTotal += difference
		jitterPairs++
	}

	statistics.FailureRatePercent = float64(failed) / float64(len(window.samples)) * 100
	if jitterPairs > 0 {
		jitter := float64(jitterTotal) / float64(jitterPairs)
		statistics.JitterMS = &jitter
	}
	return statistics
}

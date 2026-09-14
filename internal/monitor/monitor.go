package monitor

import (
	"context"
	"sync"
	"time"

	"obs-network-monitor/internal/adapter"
	"obs-network-monitor/internal/probe"
	"obs-network-monitor/internal/traceroute"
)

const (
	DefaultHistoryLimit     = 256
	DefaultHistoryRetention = 65 * time.Second
	StatisticsHistoryLimit  = 60
	DefaultInterval         = time.Second
)

type FailureMetric string

const (
	FailureMetricPacketLoss     FailureMetric = "packetLoss"
	FailureMetricRequestFailure FailureMetric = "requestFailure"
)

type Statistics struct {
	Method              probe.Method  `json:"method"`
	Target              string        `json:"target"`
	LatestLatencyMS     *int64        `json:"latestLatencyMs"`
	AverageLatencyMS    *float64      `json:"averageLatencyMs"`
	MinimumLatencyMS    *int64        `json:"minimumLatencyMs"`
	MaximumLatencyMS    *int64        `json:"maximumLatencyMs"`
	JitterMS            *float64      `json:"jitterMs"`
	FailureMetric       FailureMetric `json:"failureMetric"`
	FailureRatePercent  float64       `json:"failureRatePercent"`
	ConsecutiveFailures int          `json:"consecutiveFailures"`
}

type Traffic struct {
	TransmitBPS *float64 `json:"transmitBps"`
	ReceiveBPS  *float64 `json:"receiveBps"`
}

type TrafficSample struct {
	Traffic
	CheckedAt time.Time `json:"checkedAt"`
}

type RouteStatus string

const (
	RouteStatusMeasuring   RouteStatus = "measuring"
	RouteStatusComplete    RouteStatus = "complete"
	RouteStatusIncomplete  RouteStatus = "incomplete"
	RouteStatusUnavailable RouteStatus = "unavailable"
)

type RouteHop struct {
	Number     int     `json:"number"`
	Responded  bool    `json:"responded"`
	RTTMillis  *int64  `json:"rttMs"`
	DeltaRTTMS *int64  `json:"deltaRttMs"`
	Target     bool    `json:"target"`
}

type Route struct {
	Status    RouteStatus `json:"status"`
	CheckedAt time.Time   `json:"checkedAt"`
	HopCount  int         `json:"hopCount"`
	MaxNodes  int         `json:"maxNodes"`
	Hops      []RouteHop  `json:"hops"`
}

type Snapshot struct {
	NIC            adapter.Info    `json:"nic"`
	Traffic        Traffic         `json:"traffic"`
	Statistics     Statistics      `json:"statistics"`
	History        []probe.Result  `json:"history"`
	TrafficHistory []TrafficSample `json:"trafficHistory"`
	Route          Route           `json:"route"`
	GeneratedAt    time.Time       `json:"generatedAt"`
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

	mu            sync.Mutex
	history       historyWindow
	traffic       trafficWindow
	tracker       trafficTracker
	racer         traceroute.Tracer
	racerInterval time.Duration
	tracerMaxNodes int
	routeResult   traceroute.Result
	route         Route
	latest        Snapshot
	hasLatest     bool
	subscribers   map[chan Snapshot]struct{}
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
		history:       historyWindow{limit: historyLimit, retention: DefaultHistoryRetention},
		traffic:       trafficWindow{limit: historyLimit, retention: DefaultHistoryRetention},
		subscribers:   make(map[chan Snapshot]struct{}),
	}
}

func (monitor *Monitor) ConfigureTraceroute(tracer traceroute.Tracer, interval time.Duration, maxNodes int) {
	if interval <= 0 {
		interval = time.Minute
	}
	if maxNodes < 3 {
		maxNodes = 3
	}
	monitor.tracer = tracer
	monitor.tracerInterval = interval
	monitor.tracerMaxNodes = maxNodes
	monitor.route = Route{Status: RouteStatusMeasuring, MaxNodes: maxNodes}
}

func (monitor *Monitor) Run(ctx context.Context) {
	if monitor.tracer != nil {
		go monitor.runTraceroute(ctx)
	}
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

	generatedAt := monitor.now()

	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	monitor.history.Add(result)
	traffic, resetTraffic := monitor.tracker.Sample(nic, generatedAt, nicErr == nil)
	monitor.traffic.Add(TrafficSample{Traffic: traffic, CheckedAt: generatedAt}, resetTraffic)
	monitor.latest = Snapshot{
		NIC:            nic,
		Traffic:        traffic,
		Statistics:     monitor.history.Statistics(),
		History:        monitor.history.Samples(),
		TrafficHistory: monitor.traffic.Samples(),
		Route:          monitor.route,
		GeneratedAt:    generatedAt,
	}
	monitor.hasLatest = true
	monitor.publishLocked(monitor.latest)
	return true
}

func (monitor *Monitor) runTraceroute(ctx context.Context) {
	for {
		if !monitor.collectRoute(ctx) {
			return
		}
		timer := time.NewTimer(monitor.tracerInterval)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
	}
}

func (monitor *Monitor) collectRoute(ctx context.Context) bool {
	result, traceErr := monitor.tracer.Trace(ctx)
	if ctx.Err() != nil {
		return false
	}
	if traceErr != nil && monitor.logger != nil {
		monitor.logger.Printf("trace route: %v", traceErr)
	}
	checkedAt := result.CheckedAt
	if checkedAt.IsZero() {
		checkedAt = monitor.now()
	}
	route := sanitizedRoute(result, traceErr, monitor.tracerMaxNodes, checkedAt)

	monitor.mu.Lock()
	defer monitor.mu.Unlock()
	monitor.routeResult = result
	monitor.route = route
	if monitor.hasLatest {
		monitor.latest.Route = route
		monitor.publishLocked(monitor.latest)
	}
	return true
}

func sanitizedRoute(result traceroute.Result, traceErr error, maxNodes int, checkedAt time.Time) Route {
	status := RouteStatusIncomplete
	if result.Complete {
		status = RouteStatusComplete
	} else if traceErr != nil && len(result.Hops) == 0 {
		status = RouteStatusUnavailable
	}
	route := Route{
		Status:    status,
		CheckedAt: checkedAt,
		HopCount:  len(result.Hops),
		MaxNodes:  maxNodes,
		Hops:      make([]RouteHop, 0, len(result.Hops)),
	}
	var previousRTT *int64
	for _, hop := range result.Hops {
		item := RouteHop{Number: hop.Number, Responded: hop.Responded, Target: hop.Target}
		if hop.Responded {
			rtt := hop.RTT.Milliseconds()
			item.RTTMillis = &rtt
			if previousRTT != nil {
				delta := rtt - *previousRTT
				item.DeltaRTTMS = &delta
			}
			previousRTT = &rtt
		} else {
			previousRTT = nil
		}
		route.Hops = append(route.Hops, item)
	}
	return route
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
	result.TrafficHistory = append([]TrafficSample(nil), snapshot.TrafficHistory...)
	result.Route.Hops = append([]RouteHop(nil), snapshot.Route.Hops...)
	return result
}

type trafficTracker struct {
	valid          bool
	interfaceIndex uint32
	interfaceLUID  uint64
	transmitOctets uint64
	receiveOctets  uint64
	checkedAt      time.Time
}

func (tracker *trafficTracker) Sample(info adapter.Info, checkedAt time.Time, valid bool) (Traffic, bool) {
	if !valid || info.State != adapter.StateConnected {
		wasValid := tracker.valid
		tracker.valid = false
		return Traffic{}, wasValid
	}

	sameAdapter := tracker.valid && tracker.interfaceIndex == info.InterfaceIndex && tracker.interfaceLUID == info.InterfaceLUID
	reset := tracker.valid && !sameAdapter
	if !sameAdapter || !checkedAt.After(tracker.checkedAt) ||
		info.TransmitOctets < tracker.transmitOctets || info.ReceiveOctets < tracker.receiveOctets {
		tracker.setBaseline(info, checkedAt)
		return Traffic{}, reset
	}

	seconds := checkedAt.Sub(tracker.checkedAt).Seconds()
	transmit := float64(info.TransmitOctets-tracker.transmitOctets) * 8 / seconds
	receive := float64(info.ReceiveOctets-tracker.receiveOctets) * 8 / seconds
	tracker.setBaseline(info, checkedAt)
	return Traffic{TransmitBPS: &transmit, ReceiveBPS: &receive}, false
}

func (tracker *trafficTracker) setBaseline(info adapter.Info, checkedAt time.Time) {
	tracker.valid = true
	tracker.interfaceIndex = info.InterfaceIndex
	tracker.interfaceLUID = info.InterfaceLUID
	tracker.transmitOctets = info.TransmitOctets
	tracker.receiveOctets = info.ReceiveOctets
	tracker.checkedAt = checkedAt
}

type trafficWindow struct {
	limit     int
	retention time.Duration
	samples   []TrafficSample
}

func (window *trafficWindow) Add(sample TrafficSample, reset bool) {
	if reset {
		window.samples = window.samples[:0]
	}
	window.samples = append(window.samples, sample)
	if window.retention > 0 && !sample.CheckedAt.IsZero() {
		cutoff := sample.CheckedAt.Add(-window.retention)
		first := 0
		for first+1 < len(window.samples) && window.samples[first+1].CheckedAt.Before(cutoff) {
			first++
		}
		if first > 0 {
			copy(window.samples, window.samples[first:])
			window.samples = window.samples[:len(window.samples)-first]
		}
	}
	if window.limit > 0 && len(window.samples) > window.limit {
		overflow := len(window.samples) - window.limit
		copy(window.samples, window.samples[overflow:])
		window.samples = window.samples[:window.limit]
	}
}

func (window *trafficWindow) Samples() []TrafficSample {
	return append([]TrafficSample(nil), window.samples...)
}

type historyWindow struct {
	limit     int
	retention time.Duration
	samples   []probe.Result
}

func (window *historyWindow) Add(result probe.Result) {
	if len(window.samples) > 0 && window.samples[len(window.samples)-1].Method != result.Method {
		window.samples = window.samples[:0]
	}

	window.samples = append(window.samples, result)
	if window.retention > 0 && !result.CheckedAt.IsZero() {
		cutoff := result.CheckedAt.Add(-window.retention)
		first := 0
		for first+1 < len(window.samples) && window.samples[first+1].CheckedAt.Before(cutoff) {
			first++
		}
		if first > 0 {
			copy(window.samples, window.samples[first:])
			window.samples = window.samples[:len(window.samples)-first]
		}
	}
	if window.limit > 0 && len(window.samples) > window.limit {
		overflow := len(window.samples) - window.limit
		copy(window.samples, window.samples[overflow:])
		window.samples = window.samples[:window.limit]
	}
}

func (window *historyWindow) Samples() []probe.Result {
	return append([]probe.Result(nil), window.samples...)
}

func (window *historyWindow) Statistics() Statistics {
	if len(window.samples) == 0 {
		return Statistics{}
	}

	samples := window.samples
	if len(samples) > StatisticsHistoryLimit {
		samples = samples[len(samples)-StatisticsHistoryLimit:]
	}
	last := samples[len(samples)-1]
	statistics := Statistics{Method: last.Method, Target: last.Target}
	if last.Method == probe.MethodHTTP {
		statistics.FailureMetric = FailureMetricRequestFailure
	} else {
		statistics.FailureMetric = FailureMetricPacketLoss
	}

	failed := 0
	successful := 0
	latencyTotal := int64(0)
	jitterTotal := int64(0)
	jitterPairs := 0
	for index, sample := range samples {
		if !sample.Success {
			failed++
			continue
		}
		latency := sample.RTTMillis
		successful++
		latencyTotal += latency
		statistics.LatestLatencyMS = &latency
		if statistics.MinimumLatencyMS == nil || latency < *statistics.MinimumLatencyMS {
			minimum := latency
			statistics.MinimumLatencyMS = &minimum
		}
		if statistics.MaximumLatencyMS == nil || latency > *statistics.MaximumLatencyMS {
			maximum := latency
			statistics.MaximumLatencyMS = &maximum
		}
		if index == 0 || !samples[index-1].Success {
			continue
		}
		difference := sample.RTTMillis - window.samples[index-1].RTTMillis
		if difference < 0 {
			difference = -difference
		}
		jitterTotal += difference
		jitterPairs++
	}

	statistics.FailureRatePercent = float64(failed) / float64(len(samples)) * 100
	if successful > 0 {
		average := float64(latencyTotal) / float64(successful)
		statistics.AverageLatencyMS = &average
	}
	for index := len(samples) - 1; index >= 0 && !samples[index].Success; index-- {
		statistics.ConsecutiveFailures++
	}
	if jitterPairs > 0 {
		jitter := float64(jitterTotal) / float64(jitterPairs)
		statistics.JitterMS = &jitter
	}
	return statistics
}

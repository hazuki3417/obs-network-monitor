(function initializeLatencyView(namespace) {
  const chart = {width: 480, height: 112, capacity: 60};

  function formatValue(value, digits = 0) {
    return Number.isFinite(value) ? value.toFixed(digits) : '--';
  }

  namespace.createLatencyView = function createLatencyView(root = document) {
    const elements = {
      latency: root.querySelector('#latency'),
      averageLatency: root.querySelector('#average-latency'),
      minimumLatency: root.querySelector('#minimum-latency'),
      maximumLatency: root.querySelector('#maximum-latency'),
      jitter: root.querySelector('#jitter'),
      failureLabel: root.querySelector('#failure-label'),
      failureRate: root.querySelector('#failure-rate'),
      consecutiveFailures: root.querySelector('#consecutive-failures'),
      chartPath: root.querySelector('#latency-path'),
      axisMaximum: root.querySelector('#latency-axis-maximum'),
      axisMiddle: root.querySelector('#latency-axis-middle'),
      overlayMaximum: root.querySelector('#latency-overlay-maximum'),
    };

    function renderChart(rawHistory) {
      const {compactNumber, niceMaximum, segmentedSmoothPath} = namespace.charts;
      const history = rawHistory.slice(-chart.capacity);
      const successes = history.filter((entry) => entry.success && Number.isFinite(entry.rttMs));
      const maximum = niceMaximum(Math.max(0, ...successes.map((entry) => entry.rttMs)));
      const firstSlot = chart.capacity - history.length;
      const path = segmentedSmoothPath(history, (entry, index) => {
        if (!entry.success || !Number.isFinite(entry.rttMs)) return null;
        const x = ((firstSlot + index) / (chart.capacity - 1)) * chart.width;
        const y = chart.height - (Math.max(0, entry.rttMs) / maximum) * chart.height;
        return {x, y};
      });

      elements.chartPath.setAttribute('d', path);
      elements.axisMaximum.textContent = successes.length ? compactNumber(maximum) : '--';
      elements.axisMiddle.textContent = successes.length ? compactNumber(maximum / 2) : '--';
      elements.overlayMaximum.textContent = successes.length ? `${maximum} ms` : '-- ms';
    }

    return function renderLatency(snapshot) {
      const statistics = snapshot.statistics || {};
      const history = Array.isArray(snapshot.history) ? snapshot.history : [];

      elements.latency.textContent = formatValue(statistics.latestLatencyMs);
      elements.averageLatency.textContent = formatValue(statistics.averageLatencyMs, 1);
      elements.minimumLatency.textContent = formatValue(statistics.minimumLatencyMs);
      elements.maximumLatency.textContent = formatValue(statistics.maximumLatencyMs);
      elements.jitter.textContent = formatValue(statistics.jitterMs, 1);
      elements.failureLabel.textContent = statistics.failureMetric === 'requestFailure'
        ? 'REQUEST FAILURE'
        : 'PACKET LOSS';
      elements.failureRate.textContent = formatValue(statistics.failureRatePercent, 1);
      elements.consecutiveFailures.textContent = Number.isInteger(statistics.consecutiveFailures)
        ? statistics.consecutiveFailures
        : '--';
      renderChart(history);
    };
  };
}(window.NetworkMonitor = window.NetworkMonitor || {}));

(function initializeLatencyView(namespace) {
  function formatValue(value, digits = 0) {
    return Number.isFinite(value) ? value.toFixed(digits) : '--';
  }

  namespace.createLatencyView = function createLatencyView(root = document) {
    const showValues = document.body.dataset.parts !== 'graph';
    const elements = {
      latency: root.querySelector('#latency'),
      averageLatency: root.querySelector('#average-latency'),
      minimumLatency: root.querySelector('#minimum-latency'),
      maximumLatency: root.querySelector('#maximum-latency'),
      jitter: root.querySelector('#jitter'),
      failureLabel: root.querySelector('#failure-label'),
      failureRate: root.querySelector('#failure-rate'),
      consecutiveFailures: root.querySelector('#consecutive-failures'),
      chart: root.querySelector('#latency-chart-canvas'),
    };
    const chart = elements.chart && document.body.dataset.parts !== 'values'
      ? namespace.charts.createCanvasChart(elements.chart, {
        width: 516,
        height: 112,
        minimum: 10,
        emptyLabel: '-- ms',
        scaleLabel: (maximum) => `${namespace.charts.compactNumber(maximum)} ms`,
        series: [{key: 'latency', color: 'latency'}],
        value: (entry) => (entry.success && Number.isFinite(entry.rttMs) ? entry.rttMs : null),
      })
      : null;

    return function renderLatency(snapshot) {
      const statistics = snapshot.statistics || {};
      const history = Array.isArray(snapshot.history) ? snapshot.history : [];

      if (showValues) {
        const {updateText} = namespace.dom;
        updateText(elements.latency, formatValue(statistics.latestLatencyMs));
        updateText(elements.averageLatency, formatValue(statistics.averageLatencyMs, 1));
        updateText(elements.minimumLatency, formatValue(statistics.minimumLatencyMs));
        updateText(elements.maximumLatency, formatValue(statistics.maximumLatencyMs));
        updateText(elements.jitter, formatValue(statistics.jitterMs, 1));
        updateText(elements.failureLabel, statistics.failureMetric === 'requestFailure'
          ? 'REQUEST FAILURE'
          : 'PACKET LOSS');
        updateText(elements.failureRate, formatValue(statistics.failureRatePercent, 1));
        updateText(elements.consecutiveFailures, Number.isInteger(statistics.consecutiveFailures)
          ? statistics.consecutiveFailures
          : '--');
      }
      chart?.update(history, snapshot.generatedAt);
    };
  };
}(window.NetworkMonitor = window.NetworkMonitor || {}));

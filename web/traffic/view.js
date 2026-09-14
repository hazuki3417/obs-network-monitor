(function initializeTrafficView(namespace) {
  function formatSpeed(bitsPerSecond) {
    if (!Number.isFinite(bitsPerSecond) || bitsPerSecond < 0) return '--';
    const units = ['bps', 'Kbps', 'Mbps', 'Gbps', 'Tbps'];
    let value = bitsPerSecond;
    let unit = 0;
    while (value >= 1000 && unit < units.length - 1) {
      value /= 1000;
      unit += 1;
    }
    const digits = value >= 100 || unit === 0 ? 0 : 1;
    return `${value.toFixed(digits)} ${units[unit]}`;
  }

  function speedScale(bitsPerSecond) {
    const {compactNumber} = namespace.charts;
    const units = ['bps', 'Kbps', 'Mbps', 'Gbps', 'Tbps'];
    let value = bitsPerSecond;
    let unit = 0;
    while (value >= 1000 && unit < units.length - 1) {
      value /= 1000;
      unit += 1;
    }
    return {
      maximum: compactNumber(value),
      middle: compactNumber(value / 2),
      label: `${compactNumber(value)} ${units[unit]}`,
    };
  }

  namespace.createTrafficView = function createTrafficView(root = document) {
    const showValues = document.body.dataset.parts !== 'graph';
    const elements = {
      transmit: root.querySelector('#tx-traffic'),
      receive: root.querySelector('#rx-traffic'),
      chart: root.querySelector('#traffic-chart-canvas'),
    };
    const chart = elements.chart && document.body.dataset.parts !== 'values'
      ? namespace.charts.createCanvasChart(elements.chart, {
        width: 516,
        height: 96,
        minimum: 1000,
        emptyLabel: '--',
        scaleLabel: (maximum) => speedScale(maximum).label,
        series: [
          {key: 'transmitBps', color: 'transmit'},
          {key: 'receiveBps', color: 'receive'},
        ],
        value: (entry, key) => entry[key],
      })
      : null;

    return function renderTraffic(snapshot) {
      const traffic = snapshot.traffic || {};
      const history = Array.isArray(snapshot.trafficHistory) ? snapshot.trafficHistory : [];
      if (showValues) {
        namespace.dom.updateText(elements.transmit, formatSpeed(traffic.transmitBps));
        namespace.dom.updateText(elements.receive, formatSpeed(traffic.receiveBps));
      }
      chart?.update(history, snapshot.generatedAt);
    };
  };
}(window.NetworkMonitor = window.NetworkMonitor || {}));

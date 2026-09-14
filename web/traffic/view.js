(function initializeTrafficView(namespace) {
  const chart = {width: 480, height: 96, capacity: 60};

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
    const elements = {
      transmit: root.querySelector('#tx-traffic'),
      receive: root.querySelector('#rx-traffic'),
      transmitPath: root.querySelector('#traffic-transmit-path'),
      receivePath: root.querySelector('#traffic-receive-path'),
      axisMaximum: root.querySelector('#traffic-axis-maximum'),
      axisMiddle: root.querySelector('#traffic-axis-middle'),
      overlayMaximum: root.querySelector('#traffic-overlay-maximum'),
    };

    function path(history, key, maximum) {
      const firstSlot = chart.capacity - history.length;
      return namespace.charts.segmentedSmoothPath(history, (entry, index) => {
        const value = entry[key];
        if (!Number.isFinite(value)) return null;
        const x = ((firstSlot + index) / (chart.capacity - 1)) * chart.width;
        const y = chart.height - (Math.max(0, value) / maximum) * chart.height;
        return {x, y};
      });
    }

    function renderChart(rawHistory) {
      const history = rawHistory.slice(-chart.capacity);
      const values = history.flatMap((entry) => [entry.transmitBps, entry.receiveBps])
        .filter(Number.isFinite);
      const maximum = namespace.charts.niceMaximum(Math.max(0, ...values), 1000);

      elements.transmitPath.setAttribute('d', path(history, 'transmitBps', maximum));
      elements.receivePath.setAttribute('d', path(history, 'receiveBps', maximum));
      const scale = speedScale(maximum);
      elements.axisMaximum.textContent = values.length ? scale.maximum : '--';
      elements.axisMiddle.textContent = values.length ? scale.middle : '--';
      elements.overlayMaximum.textContent = values.length ? scale.label : '--';
    }

    return function renderTraffic(snapshot) {
      const traffic = snapshot.traffic || {};
      const history = Array.isArray(snapshot.trafficHistory) ? snapshot.trafficHistory : [];
      elements.transmit.textContent = formatSpeed(traffic.transmitBps);
      elements.receive.textContent = formatSpeed(traffic.receiveBps);
      renderChart(history);
    };
  };
}(window.NetworkMonitor = window.NetworkMonitor || {}));

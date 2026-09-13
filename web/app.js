const elements = {
  txTraffic: document.querySelector('#tx-traffic'),
  rxTraffic: document.querySelector('#rx-traffic'),
  latency: document.querySelector('#latency'),
  averageLatency: document.querySelector('#average-latency'),
  minimumLatency: document.querySelector('#minimum-latency'),
  maximumLatency: document.querySelector('#maximum-latency'),
  jitter: document.querySelector('#jitter'),
  failureLabel: document.querySelector('#failure-label'),
  failureRate: document.querySelector('#failure-rate'),
  consecutiveFailures: document.querySelector('#consecutive-failures'),
  chartPath: document.querySelector('#latency-path'),
  trafficTransmitPath: document.querySelector('#traffic-transmit-path'),
  trafficReceivePath: document.querySelector('#traffic-receive-path'),
  trafficAxisMaximum: document.querySelector('#traffic-axis-maximum'),
  trafficAxisMiddle: document.querySelector('#traffic-axis-middle'),
  trafficOverlayMaximum: document.querySelector('#traffic-overlay-maximum'),
  latencyAxisMaximum: document.querySelector('#latency-axis-maximum'),
  latencyAxisMiddle: document.querySelector('#latency-axis-middle'),
  latencyOverlayMaximum: document.querySelector('#latency-overlay-maximum'),
};

const chart = {width: 480, height: 112, capacity: 60};
const trafficChart = {width: 480, height: 96, capacity: 60};
const reconnectDelayMS = 2000;
let reconnectTimer;

function applyViewOptions() {
  const query = new URLSearchParams(location.search);
  const requestedSections = (query.get('sections') || '')
    .split(',')
    .map((section) => section.trim().toLowerCase())
    .filter((section) => section === 'latency' || section === 'traffic');
  const sections = new Set(requestedSections);
  const sectionSelection = sections.size === 1 ? [...sections][0] : 'all';
  const requestedParts = (query.get('parts') || '')
    .split(',')
    .map((part) => part.trim().toLowerCase())
    .filter((part) => part === 'values' || part === 'graph');
  const parts = new Set(requestedParts);
  const partSelection = parts.size === 1 ? [...parts][0] : 'all';

  document.body.dataset.sections = sectionSelection;
  document.body.dataset.parts = partSelection;
}

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

function formatValue(value, digits = 0) {
  return Number.isFinite(value) ? value.toFixed(digits) : '--';
}

function niceMaximum(value, minimum = 10) {
  if (!Number.isFinite(value) || value <= 0) return minimum;
  const padded = value * 1.15;
  const magnitude = 10 ** Math.floor(Math.log10(padded));
  const normalized = padded / magnitude;
  const step = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10;
  return Math.max(minimum, step * magnitude);
}

function compactNumber(value) {
  if (!Number.isFinite(value)) return '--';
  if (Number.isInteger(value)) return value.toFixed(0);
  return value.toFixed(1).replace(/\.0$/, '');
}

function speedScale(bitsPerSecond) {
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

function monotonePath(points) {
  if (points.length === 0) return '';
  if (points.length === 1) {
    return `M ${points[0].x.toFixed(2)} ${points[0].y.toFixed(2)}`;
  }

  const slopes = points.slice(0, -1).map((point, index) => {
    const next = points[index + 1];
    return (next.y - point.y) / (next.x - point.x);
  });
  const tangents = new Array(points.length);
  tangents[0] = slopes[0];
  tangents[tangents.length - 1] = slopes[slopes.length - 1];

  for (let index = 1; index < tangents.length - 1; index += 1) {
    const previous = slopes[index - 1];
    const next = slopes[index];
    tangents[index] = previous * next <= 0
      ? 0
      : (2 * previous * next) / (previous + next);
  }

  let path = `M ${points[0].x.toFixed(2)} ${points[0].y.toFixed(2)}`;
  for (let index = 0; index < points.length - 1; index += 1) {
    const point = points[index];
    const next = points[index + 1];
    const width = next.x - point.x;
    const firstControlX = point.x + width / 3;
    const firstControlY = point.y + (tangents[index] * width) / 3;
    const secondControlX = next.x - width / 3;
    const secondControlY = next.y - (tangents[index + 1] * width) / 3;
    path += ` C ${firstControlX.toFixed(2)} ${firstControlY.toFixed(2)}`
      + ` ${secondControlX.toFixed(2)} ${secondControlY.toFixed(2)}`
      + ` ${next.x.toFixed(2)} ${next.y.toFixed(2)}`;
  }
  return path;
}

function segmentedSmoothPath(history, pointForEntry) {
  const segments = [];
  let segment = [];
  history.forEach((entry, index) => {
    const point = pointForEntry(entry, index);
    if (point) {
      segment.push(point);
      return;
    }
    if (segment.length) segments.push(segment);
    segment = [];
  });
  if (segment.length) segments.push(segment);
  return segments.map(monotonePath).join(' ');
}

function trafficPath(history, key, maximum) {
  const firstSlot = trafficChart.capacity - history.length;
  return segmentedSmoothPath(history, (entry, index) => {
    const value = entry[key];
    if (!Number.isFinite(value)) return null;
    const x = ((firstSlot + index) / (trafficChart.capacity - 1)) * trafficChart.width;
    const y = trafficChart.height - (Math.max(0, value) / maximum) * trafficChart.height;
    return {x, y};
  });
}

function renderTrafficChart(rawHistory) {
  const history = rawHistory.slice(-trafficChart.capacity);
  const values = history.flatMap((entry) => [entry.transmitBps, entry.receiveBps])
    .filter(Number.isFinite);
  const maximum = niceMaximum(Math.max(0, ...values), 1000);

  elements.trafficTransmitPath.setAttribute('d', trafficPath(history, 'transmitBps', maximum));
  elements.trafficReceivePath.setAttribute('d', trafficPath(history, 'receiveBps', maximum));
  const scale = speedScale(maximum);
  elements.trafficAxisMaximum.textContent = values.length ? scale.maximum : '--';
  elements.trafficAxisMiddle.textContent = values.length ? scale.middle : '--';
  elements.trafficOverlayMaximum.textContent = values.length ? scale.label : '--';
}

function renderChart(rawHistory) {
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
  elements.latencyAxisMaximum.textContent = successes.length ? compactNumber(maximum) : '--';
  elements.latencyAxisMiddle.textContent = successes.length ? compactNumber(maximum / 2) : '--';
  elements.latencyOverlayMaximum.textContent = successes.length ? `${maximum} ms` : '-- ms';
}

function renderSnapshot(snapshot) {
  const traffic = snapshot.traffic || {};
  const statistics = snapshot.statistics || {};
  const history = Array.isArray(snapshot.history) ? snapshot.history : [];
  const trafficHistory = Array.isArray(snapshot.trafficHistory) ? snapshot.trafficHistory : [];

  elements.txTraffic.textContent = formatSpeed(traffic.transmitBps);
  elements.rxTraffic.textContent = formatSpeed(traffic.receiveBps);
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
  renderTrafficChart(trafficHistory);
}

function scheduleReconnect() {
  window.clearTimeout(reconnectTimer);
  reconnectTimer = window.setTimeout(connect, reconnectDelayMS);
}

function connect() {
  const protocol = location.protocol === 'https:' ? 'wss' : 'ws';
  const socket = new WebSocket(`${protocol}://${location.host}/ws`);

  socket.onmessage = ({data}) => {
    try {
      renderSnapshot(JSON.parse(data));
    } catch (error) {
      console.error('Invalid monitor snapshot', error);
    }
  };

  socket.onerror = () => socket.close();
  socket.onclose = () => {
    scheduleReconnect();
  };
}

applyViewOptions();
connect();

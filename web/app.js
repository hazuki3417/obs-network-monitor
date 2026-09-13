const elements = {
  streamState: document.querySelector('#stream-state'),
  streamLabel: document.querySelector('#stream-label'),
  nicName: document.querySelector('#nic-name'),
  nicDescription: document.querySelector('#nic-description'),
  nicState: document.querySelector('#nic-state'),
  txLinkSpeed: document.querySelector('#tx-link-speed'),
  rxLinkSpeed: document.querySelector('#rx-link-speed'),
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
  probeMethod: document.querySelector('#probe-method'),
  probeTarget: document.querySelector('#probe-target'),
  chartMax: document.querySelector('#chart-max'),
  chartPath: document.querySelector('#latency-path'),
  chartPoints: document.querySelector('#latency-points'),
  chartEmpty: document.querySelector('#chart-empty'),
  sampleCount: document.querySelector('#sample-count'),
  trafficChartMax: document.querySelector('#traffic-chart-max'),
  trafficTransmitPath: document.querySelector('#traffic-transmit-path'),
  trafficReceivePath: document.querySelector('#traffic-receive-path'),
  trafficChartEmpty: document.querySelector('#traffic-chart-empty'),
  trafficSampleCount: document.querySelector('#traffic-sample-count'),
  updateNote: document.querySelector('#update-note'),
  updated: document.querySelector('#updated'),
};

const chart = {width: 480, height: 112, capacity: 60};
const trafficChart = {width: 480, height: 96, capacity: 60};
const reconnectDelayMS = 2000;
let reconnectTimer;

function setStreamState(state, label) {
  elements.streamState.dataset.state = state;
  elements.streamLabel.textContent = label;
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

function formatLinkSpeed(bitsPerSecond) {
  return bitsPerSecond > 0 ? formatSpeed(bitsPerSecond) : '--';
}

function formatValue(value, digits = 0) {
  return Number.isFinite(value) ? value.toFixed(digits) : '--';
}

function nicStateLabel(state) {
  return {
    connected: '接続済み',
    disconnected: '未接続',
    unknown: '状態不明',
  }[state] || '状態不明';
}

function niceMaximum(value, minimum = 10) {
  if (!Number.isFinite(value) || value <= 0) return minimum;
  const padded = value * 1.15;
  const magnitude = 10 ** Math.floor(Math.log10(padded));
  const normalized = padded / magnitude;
  const step = normalized <= 1 ? 1 : normalized <= 2 ? 2 : normalized <= 5 ? 5 : 10;
  return Math.max(minimum, step * magnitude);
}

function trafficPath(history, key, maximum) {
  const firstSlot = trafficChart.capacity - history.length;
  let path = '';
  let drawing = false;
  history.forEach((entry, index) => {
    const value = entry[key];
    if (!Number.isFinite(value)) {
      drawing = false;
      return;
    }
    const x = ((firstSlot + index) / (trafficChart.capacity - 1)) * trafficChart.width;
    const y = trafficChart.height - (Math.max(0, value) / maximum) * trafficChart.height;
    path += `${drawing ? ' L' : ' M'} ${x.toFixed(2)} ${y.toFixed(2)}`;
    drawing = true;
  });
  return path.trim();
}

function renderTrafficChart(rawHistory) {
  const history = rawHistory.slice(-trafficChart.capacity);
  const values = history.flatMap((entry) => [entry.transmitBps, entry.receiveBps])
    .filter(Number.isFinite);
  const maximum = niceMaximum(Math.max(0, ...values), 1000);

  elements.trafficTransmitPath.setAttribute('d', trafficPath(history, 'transmitBps', maximum));
  elements.trafficReceivePath.setAttribute('d', trafficPath(history, 'receiveBps', maximum));
  elements.trafficChartMax.textContent = values.length ? formatSpeed(maximum) : '--';
  elements.trafficChartEmpty.hidden = values.length > 0;
  elements.trafficSampleCount.textContent = `${history.length} / ${trafficChart.capacity}`;
}

function renderChart(rawHistory) {
  const history = rawHistory.slice(-chart.capacity);
  const successes = history.filter((entry) => entry.success && Number.isFinite(entry.rttMs));
  const maximum = niceMaximum(Math.max(0, ...successes.map((entry) => entry.rttMs)));
  const firstSlot = chart.capacity - history.length;
  let path = '';
  let drawing = false;

  elements.chartPoints.replaceChildren();
  history.forEach((entry, index) => {
    if (!entry.success || !Number.isFinite(entry.rttMs)) {
      drawing = false;
      return;
    }
    const x = ((firstSlot + index) / (chart.capacity - 1)) * chart.width;
    const y = chart.height - (Math.max(0, entry.rttMs) / maximum) * chart.height;
    path += `${drawing ? ' L' : ' M'} ${x.toFixed(2)} ${y.toFixed(2)}`;
    drawing = true;

    const point = document.createElementNS('http://www.w3.org/2000/svg', 'circle');
    point.setAttribute('cx', x.toFixed(2));
    point.setAttribute('cy', y.toFixed(2));
    point.setAttribute('r', index === history.length - 1 ? '2.5' : '1.4');
    elements.chartPoints.append(point);
  });

  elements.chartPath.setAttribute('d', path.trim());
  elements.chartMax.textContent = successes.length ? `${maximum} ms` : '-- ms';
  elements.chartEmpty.hidden = successes.length > 0;
  elements.sampleCount.textContent = `${history.length} / ${chart.capacity}`;
}

function renderSnapshot(snapshot) {
  const nic = snapshot.nic || {};
  const traffic = snapshot.traffic || {};
  const statistics = snapshot.statistics || {};
  const history = Array.isArray(snapshot.history) ? snapshot.history : [];
  const trafficHistory = Array.isArray(snapshot.trafficHistory) ? snapshot.trafficHistory : [];

  elements.nicName.textContent = nic.name || '--';
  elements.nicDescription.textContent = nic.description || 'アダプター情報なし';
  elements.nicState.textContent = nicStateLabel(nic.state);
  elements.txLinkSpeed.textContent = formatLinkSpeed(nic.transmitLinkSpeedBps);
  elements.rxLinkSpeed.textContent = formatLinkSpeed(nic.receiveLinkSpeedBps);
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
  elements.probeMethod.textContent = statistics.method ? statistics.method.toUpperCase() : '--';
  elements.probeTarget.textContent = statistics.target || '測定先なし';
  renderChart(history);
  renderTrafficChart(trafficHistory);

  const generatedAt = new Date(snapshot.generatedAt);
  elements.updated.textContent = Number.isNaN(generatedAt.getTime())
    ? '--:--:--'
    : generatedAt.toLocaleTimeString('ja-JP', {hour12: false});
  elements.updateNote.textContent = '最終更新';
  setStreamState('live', '受信中');
}

function scheduleReconnect() {
  window.clearTimeout(reconnectTimer);
  reconnectTimer = window.setTimeout(connect, reconnectDelayMS);
}

function connect() {
  setStreamState('connecting', '接続中');
  const protocol = location.protocol === 'https:' ? 'wss' : 'ws';
  const socket = new WebSocket(`${protocol}://${location.host}/ws`);

  socket.onopen = () => {
    setStreamState('connecting', 'データ待機');
  };

  socket.onmessage = ({data}) => {
    try {
      renderSnapshot(JSON.parse(data));
    } catch (error) {
      console.error('Invalid monitor snapshot', error);
      elements.updateNote.textContent = 'データ形式エラー';
    }
  };

  socket.onerror = () => socket.close();
  socket.onclose = () => {
    setStreamState('disconnected', '再接続中');
    elements.updateNote.textContent = '更新停止 · 最終受信';
    scheduleReconnect();
  };
}

connect();

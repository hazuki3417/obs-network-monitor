const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

class FakeElement {
  constructor() {
    this.attributes = {};
    this.dataset = {};
    this.textContent = '';
  }

  setAttribute(name, value) {
    this.attributes[name] = value;
  }
}

const pages = {
  combined: {
    html: fs.readFileSync('web/index.html', 'utf8'),
    scripts: [
      'web/shared/options.js',
      'web/shared/chart.js',
      'web/shared/websocket.js',
      'web/latency/view.js',
      'web/traffic/view.js',
      'web/app.js',
    ],
  },
  latency: {
    html: fs.readFileSync('web/latency/index.html', 'utf8'),
    scripts: [
      'web/shared/options.js',
      'web/shared/chart.js',
      'web/shared/websocket.js',
      'web/latency/view.js',
      'web/latency/app.js',
    ],
  },
  traffic: {
    html: fs.readFileSync('web/traffic/index.html', 'utf8'),
    scripts: [
      'web/shared/options.js',
      'web/shared/chart.js',
      'web/shared/websocket.js',
      'web/traffic/view.js',
      'web/traffic/app.js',
    ],
  },
};
const css = fs.readFileSync('web/shared/base.css', 'utf8');

for (const [name, page] of Object.entries(pages)) {
  const ids = [...page.html.matchAll(/id="([^"]+)"/g)].map((match) => match[1]);
  assert.equal(new Set(ids).size, ids.length, `${name} HTML element IDs must be unique`);
  assert.match(page.html, /href="\/shared\/base\.css"/, `${name} must use shared CSS`);
  const scriptSources = [...page.html.matchAll(/<script src="([^"]+)"/g)].map((match) => match[1]);
  assert.deepEqual(scriptSources, page.scripts.map((script) => script.replace(/^web/, '')));
  assert.doesNotMatch(
    page.html,
    /成功した測定を待っています|通信量を測定しています/,
    `${name} charts must not contain waiting text`,
  );
}
assert.match(pages.combined.html, /id="latency-chart-svg"/);
assert.match(pages.combined.html, /id="traffic-chart-svg"/);
assert.match(pages.latency.html, /id="latency-chart-svg"/);
assert.doesNotMatch(pages.latency.html, /id="traffic-chart-svg"/);
assert.match(pages.traffic.html, /id="traffic-chart-svg"/);
assert.doesNotMatch(pages.traffic.html, /id="latency-chart-svg"/);
assert.match(
  pages.latency.html,
  /<span class="label">LATENCY<\/span>\s*<strong id="latency">--<\/strong>\s*<span class="unit">ms<\/span>/,
  'label, value, and unit must be independent grid items',
);
assert.match(css, /width: min\(464px, calc\(100vw - 16px\)\)/, 'overlay must fit a 480 px source');
assert.match(css, /grid-template-columns: minmax\(0, 1fr\) 58px 20px/, 'metric columns must remain aligned');
assert.match(css, /data-parts="values"/, 'values-only styles must exist');
assert.match(css, /data-parts="graph"/, 'graph-only styles must exist');

function loadPage(page, search = '') {
  const ids = [...page.html.matchAll(/id="([^"]+)"/g)].map((match) => match[1]);
  const elements = new Map(ids.map((id) => [id, new FakeElement()]));
  const body = new FakeElement();
  const sockets = [];
  class FakeWebSocket {
    constructor(url) {
      this.url = url;
      sockets.push(this);
    }

    close() {}
  }
  const window = {clearTimeout() {}, setTimeout() {}};
  const context = {
    console,
    document: {
      body,
      querySelector(selector) {
        const element = elements.get(selector.slice(1));
        assert.ok(element, `missing HTML element for ${selector}`);
        return element;
      },
    },
    location: {host: '127.0.0.1:8080', protocol: 'http:', search},
    URLSearchParams,
    WebSocket: FakeWebSocket,
    window,
  };
  for (const script of page.scripts) {
    vm.runInNewContext(fs.readFileSync(script, 'utf8'), context, {filename: script});
  }
  return {body, elements, namespace: window.NetworkMonitor, sockets};
}

const snapshot = {
  traffic: {transmitBps: 0, receiveBps: 1_500_000},
  statistics: {
    latestLatencyMs: 20,
    averageLatencyMs: 15.5,
    minimumLatencyMs: 10,
    maximumLatencyMs: 20,
    jitterMs: 5,
    failureMetric: 'packetLoss',
    failureRatePercent: 0,
    consecutiveFailures: 0,
  },
  history: [
    {success: true, rttMs: 10},
    {success: true, rttMs: 15},
    {success: false, rttMs: 0},
    {success: true, rttMs: 20},
    {success: true, rttMs: 25},
  ],
  trafficHistory: [
    {transmitBps: null, receiveBps: null},
    {transmitBps: 0, receiveBps: 1_000_000},
    {transmitBps: 500_000, receiveBps: 1_500_000},
  ],
};

const combined = loadPage(pages.combined);
assert.equal(combined.sockets.length, 1, 'combined page must use one WebSocket');
assert.equal(combined.sockets[0].url, 'ws://127.0.0.1:8080/ws');
assert.equal(combined.body.dataset.sections, 'all');
assert.equal(combined.body.dataset.parts, 'all');
assert.equal(
  combined.namespace.charts.monotonePath([{x: 0, y: 10}, {x: 1, y: 0}, {x: 2, y: 10}]),
  'M 0.00 10.00 C 0.33 6.67 0.67 0.00 1.00 0.00 C 1.33 0.00 1.67 6.67 2.00 10.00',
);
combined.sockets[0].onmessage({data: JSON.stringify(snapshot)});
assert.equal(combined.elements.get('latency').textContent, '20');
assert.equal(combined.elements.get('average-latency').textContent, '15.5');
assert.equal(combined.elements.get('tx-traffic').textContent, '0 bps');
assert.equal(combined.elements.get('rx-traffic').textContent, '1.5 Mbps');
assert.match(combined.elements.get('latency-path').attributes.d, /^M .* C .* M .* C /);
assert.match(combined.elements.get('traffic-transmit-path').attributes.d, / C /);
assert.equal(combined.elements.get('latency-axis-maximum').textContent, '50');
assert.equal(combined.elements.get('traffic-overlay-maximum').textContent, '2 Mbps');

const legacyLatency = loadPage(pages.combined, '?sections=latency&parts=graph');
assert.equal(legacyLatency.body.dataset.sections, 'latency');
assert.equal(legacyLatency.body.dataset.parts, 'graph');

const latency = loadPage(pages.latency, '?parts=values');
assert.equal(latency.sockets.length, 1, 'latency page must use one WebSocket');
assert.equal(latency.body.dataset.parts, 'values');
latency.sockets[0].onmessage({data: JSON.stringify(snapshot)});
assert.equal(latency.elements.get('latency').textContent, '20');
assert.equal(latency.elements.has('tx-traffic'), false);

const traffic = loadPage(pages.traffic, '?parts=graph');
assert.equal(traffic.sockets.length, 1, 'traffic page must use one WebSocket');
assert.equal(traffic.body.dataset.parts, 'graph');
traffic.sockets[0].onmessage({data: JSON.stringify(snapshot)});
assert.equal(traffic.elements.get('rx-traffic').textContent, '1.5 Mbps');
assert.equal(traffic.elements.has('latency'), false);

const allParts = loadPage(pages.traffic, '?parts=values,graph');
assert.equal(allParts.body.dataset.parts, 'all');
const invalidParts = loadPage(pages.latency, '?parts=unknown');
assert.equal(invalidParts.body.dataset.parts, 'all');

console.log('Web UI tests passed.');

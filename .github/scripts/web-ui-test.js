const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

class FakeElement {
  constructor() {
    this.attributes = {};
    this.children = [];
    this.dataset = {};
    this.hidden = false;
    this.textContent = '';
  }

  append(child) {
    this.children.push(child);
  }

  replaceChildren() {
    this.children = [];
  }

  setAttribute(name, value) {
    this.attributes[name] = value;
  }
}

const html = fs.readFileSync('web/index.html', 'utf8');
const css = fs.readFileSync('web/style.css', 'utf8');
const app = fs.readFileSync('web/app.js', 'utf8');
const ids = [...html.matchAll(/id="([^"]+)"/g)].map((match) => match[1]);
assert.equal(new Set(ids).size, ids.length, 'HTML element IDs must be unique');
assert.doesNotMatch(html, /id="nic-name"/, 'legacy NIC dashboard must not remain');
assert.doesNotMatch(html, /OBS NETWORK MONITOR/, 'legacy product heading must not remain');
assert.doesNotMatch(html, /成功した測定を待っています|通信量を測定しています/, 'charts must not contain waiting text');
assert.match(
  html,
  /<span class="label">LATENCY<\/span>\s*<strong id="latency">--<\/strong>\s*<span class="unit">ms<\/span>/,
  'label, value, and unit must be independent grid items',
);
assert.match(html, /id="consecutive-failures">0<\/strong>\s*<span class="unit" aria-hidden="true"><\/span>/);
assert.match(css, /width: min\(464px, calc\(100vw - 16px\)\)/, 'overlay must fit a 480 px source');
assert.match(css, /grid-template-columns: minmax\(0, 1fr\) 58px 20px/, 'metric columns must have fixed value and unit widths');
assert.match(css, /data-sections="latency"/, 'latency-only styles must exist');
assert.match(css, /data-sections="traffic"/, 'traffic-only styles must exist');
assert.match(css, /data-parts="values"/, 'values-only styles must exist');
assert.match(css, /data-parts="graph"/, 'graph-only styles must exist');
assert.equal((app.match(/new WebSocket/g) || []).length, 1, 'UI parts must share one WebSocket');

function loadUI(search = '', pathname = '/') {
  const elements = new Map(ids.map((id) => [id, new FakeElement()]));
  const body = new FakeElement();
  const context = {
    console,
    document: {
      body,
      querySelector(selector) {
        const element = elements.get(selector.slice(1));
        assert.ok(element, `missing HTML element for ${selector}`);
        return element;
      },
      createElementNS() {
        return new FakeElement();
      },
    },
    location: {host: '127.0.0.1:8080', pathname, protocol: 'http:', search},
    URLSearchParams,
    WebSocket: class {},
    window: {clearTimeout() {}, setTimeout() {}},
  };
  vm.runInNewContext(app, context);
  return {body, context, elements};
}

const {body, context, elements} = loadUI();
assert.equal(
  context.monotonePath([{x: 0, y: 10}, {x: 1, y: 0}, {x: 2, y: 10}]),
  'M 0.00 10.00 C 0.33 6.67 0.67 0.00 1.00 0.00 C 1.33 0.00 1.67 6.67 2.00 10.00',
  'smooth path controls must stay within the adjacent sample range',
);
context.renderSnapshot({
  nic: {
    name: 'Ethernet',
    description: 'Test adapter',
    state: 'connected',
    transmitLinkSpeedBps: 1_000_000_000,
    receiveLinkSpeedBps: 1_000_000_000,
  },
  traffic: {transmitBps: 0, receiveBps: 1_500_000},
  statistics: {
    method: 'icmp',
    target: '8.8.8.8',
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
  generatedAt: '2026-09-13T03:00:00Z',
});

assert.equal(elements.get('tx-traffic').textContent, '0 bps');
assert.equal(elements.get('rx-traffic').textContent, '1.5 Mbps');
assert.equal(elements.get('average-latency').textContent, '15.5');
assert.equal(elements.get('minimum-latency').textContent, '10');
assert.equal(elements.get('maximum-latency').textContent, '20');
assert.equal(elements.get('consecutive-failures').textContent, 0);
assert.match(elements.get('latency-path').attributes.d, /^M .* C .* M .* C /, 'failed sample must split smooth curves');
assert.match(elements.get('traffic-transmit-path').attributes.d, / C /, 'traffic path must be smooth');
assert.match(elements.get('traffic-receive-path').attributes.d, / C /, 'traffic path must be smooth');
assert.equal(elements.get('latency-axis-maximum').textContent, '50');
assert.equal(elements.get('latency-axis-middle').textContent, '25');
assert.equal(elements.get('latency-overlay-maximum').textContent, '50 ms');
assert.equal(elements.get('traffic-axis-maximum').textContent, '2');
assert.equal(elements.get('traffic-axis-middle').textContent, '1');
assert.equal(elements.get('traffic-overlay-maximum').textContent, '2 Mbps');
assert.equal(body.dataset.sections, 'all');
assert.equal(body.dataset.parts, 'all');
assert.match(html, /id="latency-chart-svg" viewBox="-36 0 516 112"/);
assert.match(html, /id="traffic-chart-svg" viewBox="-36 0 516 96"/);

const latencyOnly = loadUI('?sections=latency');
assert.equal(latencyOnly.body.dataset.sections, 'latency');

const trafficOnly = loadUI('?sections=traffic');
assert.equal(trafficOnly.body.dataset.sections, 'traffic');

const invalidSections = loadUI('?sections=unknown');
assert.equal(invalidSections.body.dataset.sections, 'all');

const legacyOverlayURL = loadUI('?view=overlay');
assert.equal(legacyOverlayURL.body.dataset.sections, 'all');

const valuesOnly = loadUI('?parts=values');
assert.equal(valuesOnly.body.dataset.parts, 'values');

const graphOnly = loadUI('?parts=graph');
assert.equal(graphOnly.body.dataset.parts, 'graph');

const allParts = loadUI('?parts=values,graph');
assert.equal(allParts.body.dataset.parts, 'all');

const invalidParts = loadUI('?parts=unknown');
assert.equal(invalidParts.body.dataset.parts, 'all');

const combinedOptions = loadUI('?sections=traffic&parts=graph');
assert.equal(combinedOptions.body.dataset.sections, 'traffic');
assert.equal(combinedOptions.body.dataset.parts, 'graph');

const latencyPage = loadUI('?parts=values', '/latency');
assert.equal(latencyPage.body.dataset.sections, 'latency');
assert.equal(latencyPage.body.dataset.parts, 'values');

const trafficPage = loadUI('?parts=graph', '/traffic');
assert.equal(trafficPage.body.dataset.sections, 'traffic');
assert.equal(trafficPage.body.dataset.parts, 'graph');

const pathOverridesLegacyQuery = loadUI('?sections=traffic', '/latency');
assert.equal(pathOverridesLegacyQuery.body.dataset.sections, 'latency');

console.log('Web UI tests passed.');

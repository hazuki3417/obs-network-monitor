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
const ids = [...html.matchAll(/id="([^"]+)"/g)].map((match) => match[1]);
assert.equal(new Set(ids).size, ids.length, 'HTML element IDs must be unique');

const elements = new Map(ids.map((id) => [id, new FakeElement()]));
const context = {
  console,
  document: {
    querySelector(selector) {
      const element = elements.get(selector.slice(1));
      assert.ok(element, `missing HTML element for ${selector}`);
      return element;
    },
    createElementNS() {
      return new FakeElement();
    },
  },
  location: {host: '127.0.0.1:8080', protocol: 'http:'},
  WebSocket: class {},
  window: {clearTimeout() {}, setTimeout() {}},
};

vm.runInNewContext(fs.readFileSync('web/app.js', 'utf8'), context);
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
    {success: false, rttMs: 0},
    {success: true, rttMs: 20},
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
assert.match(elements.get('latency-path').attributes.d, /^M .* M /, 'failed latency sample must split the line');
assert.ok(elements.get('traffic-transmit-path').attributes.d);
assert.ok(elements.get('traffic-receive-path').attributes.d);
assert.equal(elements.get('traffic-sample-count').textContent, '3 / 60');

console.log('Web UI tests passed.');

const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

class FakeContext {
  constructor() {
    this.operations = [];
  }

  record(name, ...args) { this.operations.push({name, args}); }
  setTransform(...args) { this.record('setTransform', ...args); }
  clearRect(...args) { this.record('clearRect', ...args); }
  beginPath() { this.record('beginPath'); }
  moveTo(...args) { this.record('moveTo', ...args); }
  lineTo(...args) { this.record('lineTo', ...args); }
  bezierCurveTo(...args) { this.record('bezierCurveTo', ...args); }
  stroke() { this.record('stroke'); }
  fillText(...args) { this.record('fillText', ...args); }
  save() { this.record('save'); }
  restore() { this.record('restore'); }
  rect(...args) { this.record('rect', ...args); }
  clip() { this.record('clip'); }
}

class FakeElement {
  constructor(id = '') {
    this.id = id;
    this.attributes = {};
    this.dataset = {};
    this.textContent = '';
    this.animations = [];
    this.context = id.endsWith('-canvas') ? new FakeContext() : null;
  }

  setAttribute(name, value) { this.attributes[name] = value; }
  getContext(kind) {
    assert.equal(kind, '2d');
    return this.context;
  }
  getBoundingClientRect() {
    return {width: 464, height: this.id.startsWith('traffic') ? 96 : 104};
  }
  getAnimations() { return this.animations; }
  animate(keyframes, options) {
    const animation = {keyframes, options, cancel() {}};
    this.animations.push(animation);
    return animation;
  }
}

const sharedScripts = [
  'web/shared/options.js',
  'web/shared/animation.js',
  'web/shared/dom.js',
  'web/shared/chart.js',
  'web/shared/websocket.js',
];
const pages = {
  combined: {
    html: fs.readFileSync('web/index.html', 'utf8'),
    scripts: [...sharedScripts, 'web/latency/view.js', 'web/traffic/view.js', 'web/app.js'],
  },
  latency: {
    html: fs.readFileSync('web/latency/index.html', 'utf8'),
    scripts: [...sharedScripts, 'web/latency/view.js', 'web/latency/app.js'],
  },
  traffic: {
    html: fs.readFileSync('web/traffic/index.html', 'utf8'),
    scripts: [...sharedScripts, 'web/traffic/view.js', 'web/traffic/app.js'],
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
assert.match(pages.combined.html, /id="latency-chart-canvas"/);
assert.match(pages.combined.html, /id="traffic-chart-canvas"/);
assert.match(pages.latency.html, /id="latency-chart-canvas"/);
assert.doesNotMatch(pages.latency.html, /id="traffic-chart-canvas"/);
assert.match(pages.traffic.html, /id="traffic-chart-canvas"/);
assert.doesNotMatch(pages.traffic.html, /id="latency-chart-canvas"/);
assert.doesNotMatch(pages.combined.html, /<svg|<path/);
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
  const elements = new Map(ids.map((id) => [id, new FakeElement(id)]));
  const body = new FakeElement('body');
  const sockets = [];
  const frames = [];
  let frameId = 0;
  let clock = 0;
  class FakeWebSocket {
    constructor(url) {
      this.url = url;
      sockets.push(this);
    }

    close() {}
  }
  const window = {
    clearTimeout() {},
    setTimeout() {},
    devicePixelRatio: 2,
    performance: {now: () => clock},
    requestAnimationFrame(callback) {
      frameId += 1;
      frames.push({id: frameId, callback});
      return frameId;
    },
    cancelAnimationFrame(id) {
      const index = frames.findIndex((frame) => frame.id === id);
      if (index >= 0) frames.splice(index, 1);
    },
  };
  const context = {
    console,
    Date,
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
  function runFrame(time) {
    clock = time;
    const pending = frames.shift();
    assert.ok(pending, 'an animation frame must be scheduled');
    pending.callback(time);
  }
  return {body, elements, frames, namespace: window.NetworkMonitor, runFrame, sockets};
}

const snapshot = {
  generatedAt: '2026-09-14T12:00:05.000Z',
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
    {checkedAt: '2026-09-14T12:00:01.000Z', success: true, rttMs: 10},
    {checkedAt: '2026-09-14T12:00:02.000Z', success: true, rttMs: 15},
    {checkedAt: '2026-09-14T12:00:03.000Z', success: false, rttMs: 0},
    {checkedAt: '2026-09-14T12:00:04.000Z', success: true, rttMs: 20},
    {checkedAt: '2026-09-14T12:00:05.000Z', success: true, rttMs: 25},
  ],
  trafficHistory: [
    {checkedAt: '2026-09-14T12:00:03.000Z', transmitBps: null, receiveBps: null},
    {checkedAt: '2026-09-14T12:00:04.000Z', transmitBps: 0, receiveBps: 1_000_000},
    {checkedAt: '2026-09-14T12:00:05.000Z', transmitBps: 500_000, receiveBps: 1_500_000},
  ],
};

const combined = loadPage(pages.combined);
assert.equal(combined.sockets.length, 1, 'combined page must use one WebSocket');
assert.equal(combined.sockets[0].url, 'ws://127.0.0.1:8080/ws');
assert.equal(combined.body.dataset.sections, 'all');
assert.equal(combined.body.dataset.parts, 'all');
assert.equal(combined.frames.length, 1, 'all charts must share one requestAnimationFrame loop');
combined.sockets[0].onmessage({data: JSON.stringify(snapshot)});
assert.equal(combined.elements.get('latency').textContent, '20');
assert.equal(combined.elements.get('average-latency').textContent, '15.5');
assert.equal(combined.elements.get('tx-traffic').textContent, '0 bps');
assert.equal(combined.elements.get('rx-traffic').textContent, '1.5 Mbps');
assert.equal(combined.elements.get('latency').animations.length, 1, 'changed values must fade once');
combined.runFrame(17);
assert.equal(combined.frames.length, 1, 'the shared loop must schedule only one next frame');
const latencyContext = combined.elements.get('latency-chart-canvas').context;
const trafficContext = combined.elements.get('traffic-chart-canvas').context;
assert.ok(latencyContext.operations.some((operation) => operation.name === 'bezierCurveTo'));
assert.ok(trafficContext.operations.some((operation) => operation.name === 'bezierCurveTo'));
assert.ok(
  latencyContext.operations.filter((operation) => operation.name === 'moveTo').length >= 5,
  'a failed latency sample must split the curve into separate segments',
);
assert.ok(latencyContext.operations.some(
  (operation) => operation.name === 'fillText' && operation.args[0] === '50 ms',
));
const firstCurveEnd = latencyContext.operations.filter(
  (operation) => operation.name === 'bezierCurveTo',
).at(-1).args[4];
const operationCount = latencyContext.operations.length;
combined.runFrame(25);
assert.equal(
  latencyContext.operations.length,
  operationCount,
  'rendering must not exceed 60fps on high-refresh displays',
);
combined.runFrame(34);
const secondCurveEnd = latencyContext.operations.filter(
  (operation) => operation.name === 'bezierCurveTo',
).at(-1).args[4];
assert.ok(secondCurveEnd < firstCurveEnd, 'timestamp-based graph must move left between samples');
assert.equal(combined.elements.get('latency').animations.length, 1, 'animation frames must not update values');
assert.equal(combined.elements.get('latency-chart-canvas').width, 928, 'canvas must use high-DPI width');

const legacyLatency = loadPage(pages.combined, '?sections=latency&parts=graph');
assert.equal(legacyLatency.body.dataset.sections, 'latency');
assert.equal(legacyLatency.body.dataset.parts, 'graph');
assert.equal(legacyLatency.frames.length, 1, 'hidden traffic chart must not register another renderer');
legacyLatency.sockets[0].onmessage({data: JSON.stringify(snapshot)});
assert.equal(legacyLatency.elements.get('latency').textContent, '', 'graph-only view must skip value DOM updates');

const latency = loadPage(pages.latency, '?parts=values');
assert.equal(latency.sockets.length, 1, 'latency page must use one WebSocket');
assert.equal(latency.body.dataset.parts, 'values');
assert.equal(latency.frames.length, 0, 'values-only view must not start canvas rendering');
latency.sockets[0].onmessage({data: JSON.stringify(snapshot)});
assert.equal(latency.elements.get('latency').textContent, '20');
assert.equal(latency.elements.has('tx-traffic'), false);

const traffic = loadPage(pages.traffic, '?parts=graph');
assert.equal(traffic.sockets.length, 1, 'traffic page must use one WebSocket');
assert.equal(traffic.body.dataset.parts, 'graph');
assert.equal(traffic.frames.length, 1);
traffic.sockets[0].onmessage({data: JSON.stringify(snapshot)});
assert.equal(traffic.elements.get('rx-traffic').textContent, '', 'graph-only view must skip value DOM updates');
traffic.runFrame(17);
assert.ok(traffic.elements.get('traffic-chart-canvas').context.operations.some(
  (operation) => operation.name === 'fillText' && operation.args[0] === '2 Mbps',
));

const allParts = loadPage(pages.traffic, '?parts=values,graph');
assert.equal(allParts.body.dataset.parts, 'all');
const invalidParts = loadPage(pages.latency, '?parts=unknown');
assert.equal(invalidParts.body.dataset.parts, 'all');

console.log('Web UI tests passed.');

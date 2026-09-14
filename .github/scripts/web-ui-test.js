const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');

class FakeContext {
  constructor() { this.operations = []; }
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
    this.className = '';
    this.dataset = {};
    this.textContent = '';
    this.lastChild = {textContent: ''};
    this.animations = [];
    this.context = id.endsWith('-canvas') ? new FakeContext() : null;
  }

  setAttribute(name, value) { this.attributes[name] = value; }
  getContext(kind) { assert.equal(kind, '2d'); return this.context; }
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

const overlaySharedScripts = [
  'web/shared/options.js',
  'web/shared/animation.js',
  'web/shared/dom.js',
  'web/shared/chart.js',
  'web/shared/websocket.js',
];
const pages = {
  home: {
    html: fs.readFileSync('web/index.html', 'utf8'),
    scripts: ['web/shared/websocket.js', 'web/app.js'],
  },
  latency: {
    html: fs.readFileSync('web/latency/index.html', 'utf8'),
    scripts: [...overlaySharedScripts, 'web/latency/view.js', 'web/latency/app.js'],
  },
  traffic: {
    html: fs.readFileSync('web/traffic/index.html', 'utf8'),
    scripts: [...overlaySharedScripts, 'web/traffic/view.js', 'web/traffic/app.js'],
  },
};
const overlayCSS = fs.readFileSync('web/shared/base.css', 'utf8');
const homeCSS = fs.readFileSync('web/home.css', 'utf8');

for (const [name, page] of Object.entries(pages)) {
  const ids = [...page.html.matchAll(/id="([^"]+)"/g)].map((match) => match[1]);
  assert.equal(new Set(ids).size, ids.length, `${name} HTML element IDs must be unique`);
  const scriptSources = [...page.html.matchAll(/<script src="([^"]+)"/g)].map((match) => match[1]);
  assert.deepEqual(scriptSources, page.scripts.map((script) => script.replace(/^web/, '')));
}

assert.match(pages.home.html, /id="service-status"/);
assert.match(pages.home.html, /id="websocket-status"/);
assert.match(pages.home.html, /id="stream-status"/);
assert.match(pages.home.html, /href="\/latency"/);
assert.match(pages.home.html, /href="\/traffic"/);
assert.match(pages.home.html, /github\.com\/hazuki3417\/obs-network-monitor/);
assert.match(pages.home.html, /MIT License/);
assert.match(pages.home.html, /© 2026/);
assert.match(pages.home.html, /<ul class="links">/);
assert.match(pages.home.html, /href="\/latency">http:\/\/127\.0\.0\.1:8080\/latency<\/a>/);
assert.match(pages.home.html, /<footer>[\s\S]*MIT License[\s\S]*<\/footer>/);
assert.doesNotMatch(pages.home.html, /latency-chart-canvas|traffic-chart-canvas/);
assert.match(pages.latency.html, /id="latency-chart-canvas"/);
assert.doesNotMatch(pages.latency.html, /id="traffic-chart-canvas"/);
assert.match(pages.traffic.html, /id="traffic-chart-canvas"/);
assert.doesNotMatch(pages.traffic.html, /id="latency-chart-canvas"/);
assert.match(overlayCSS, /data-parts="values"/);
assert.match(overlayCSS, /data-parts="graph"/);
assert.match(homeCSS, /grid-template-columns: minmax\(0, 1fr\) 160px/);

function loadPage(page, search = '') {
  const ids = [...page.html.matchAll(/id="([^"]+)"/g)].map((match) => match[1]);
  const elements = new Map(ids.map((id) => [id, new FakeElement(id)]));
  const body = new FakeElement('body');
  const sockets = [];
  const frames = [];
  let frameId = 0;
  let clock = 0;
  class FakeWebSocket {
    constructor(url) { this.url = url; sockets.push(this); }
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
  return {body, elements, frames, runFrame, sockets};
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
  ],
  trafficHistory: [
    {checkedAt: '2026-09-14T12:00:04.000Z', transmitBps: 0, receiveBps: 1_000_000},
    {checkedAt: '2026-09-14T12:00:05.000Z', transmitBps: 500_000, receiveBps: 1_500_000},
  ],
};

const home = loadPage(pages.home);
assert.equal(home.sockets.length, 1, 'home health check must connect to WebSocket');
assert.equal(home.sockets[0].url, 'ws://127.0.0.1:8080/ws');
assert.equal(home.elements.get('websocket-status').lastChild.textContent, 'Connecting');
home.sockets[0].onopen();
assert.equal(home.elements.get('websocket-status').lastChild.textContent, 'Connected');
assert.equal(home.elements.get('websocket-status').className, 'status is-healthy');
home.sockets[0].onmessage({data: JSON.stringify(snapshot)});
assert.equal(home.elements.get('stream-status').lastChild.textContent, 'Receiving data');
assert.notEqual(home.elements.get('last-update').textContent, '--');
home.sockets[0].onclose();
assert.equal(home.elements.get('websocket-status').lastChild.textContent, 'Reconnecting');
assert.equal(home.elements.get('stream-status').lastChild.textContent, 'Waiting for data');

const latency = loadPage(pages.latency, '?parts=values');
assert.equal(latency.sockets.length, 1);
assert.equal(latency.body.dataset.parts, 'values');
assert.equal(latency.frames.length, 0, 'values-only view must not render Canvas');
latency.sockets[0].onmessage({data: JSON.stringify(snapshot)});
assert.equal(latency.elements.get('latency').textContent, '20');
assert.equal(
  latency.elements.get('latency').animations.length,
  0,
  'value updates must not use a flashing animation',
);

const traffic = loadPage(pages.traffic, '?parts=graph');
assert.equal(traffic.sockets.length, 1);
assert.equal(traffic.body.dataset.parts, 'graph');
assert.equal(traffic.frames.length, 1);
traffic.sockets[0].onmessage({data: JSON.stringify({
  ...snapshot,
  trafficHistory: Array.from({length: 5}, (_, index) => ({
    checkedAt: `2026-09-14T12:00:0${index + 1}.000Z`,
    transmitBps: index * 250_000,
    receiveBps: (index + 1) * 300_000,
  })),
})});
assert.equal(traffic.elements.get('rx-traffic').textContent, '');
traffic.runFrame(17);
const trafficOperations = traffic.elements.get('traffic-chart-canvas').context.operations;
assert.ok(trafficOperations.some(
  (operation) => operation.name === 'bezierCurveTo',
));
assert.ok(trafficOperations.some(
  (operation) => operation.name === 'bezierCurveTo'
    && operation.args[4] > 516,
), 'received samples must be rendered in the hidden right overscan area');
assert.equal(trafficOperations.some(
  (operation) => operation.name === 'bezierCurveTo'
    && operation.args[4] === 516,
), false, 'the visible right edge must not use a synthetic horizontal extension');
assert.ok(trafficOperations.some(
  (operation) => operation.name === 'rect'
    && operation.args[0] === 28
    && operation.args[1] === 1
    && operation.args[2] === 488
    && operation.args[3] === 94,
), 'graph lines must be clipped at a fixed left boundary');
assert.equal(
  trafficOperations.filter((operation) => operation.name === 'lineTo'
    && operation.args[0] === 28).length,
  0,
  'the fixed clipping boundary must remain invisible',
);

const trafficWithRenderBuffer = loadPage(pages.traffic, '?parts=graph');
const bufferedAt = Date.parse('2026-09-14T12:01:03.000Z');
trafficWithRenderBuffer.sockets[0].onmessage({data: JSON.stringify({
  ...snapshot,
  generatedAt: new Date(bufferedAt).toISOString(),
  trafficHistory: Array.from({length: 74}, (_, index) => ({
    checkedAt: new Date(bufferedAt - (73 - index) * 900).toISOString(),
    transmitBps: index * 1000,
    receiveBps: index * 2000,
  })),
})});
trafficWithRenderBuffer.runFrame(17);
assert.ok(
  trafficWithRenderBuffer.elements.get('traffic-chart-canvas').context.operations.some(
    (operation) => operation.name === 'moveTo' && operation.args[0] < 28,
  ),
  'render history must retain more than 60 seconds when intervals are short',
);

const trafficWithStableClock = loadPage(pages.traffic, '?parts=graph');
const stableTrafficHistory = Array.from({length: 5}, (_, index) => ({
  checkedAt: `2026-09-14T12:00:0${index + 1}.000Z`,
  transmitBps: index * 250_000,
  receiveBps: (index + 1) * 300_000,
}));
trafficWithStableClock.sockets[0].onmessage({data: JSON.stringify({
  ...snapshot,
  trafficHistory: stableTrafficHistory,
})});
trafficWithStableClock.runFrame(1000);
const firstFrameOperations = trafficWithStableClock.elements
  .get('traffic-chart-canvas').context.operations;
const firstFrameSeriesX = firstFrameOperations.filter(
  (operation) => operation.name === 'moveTo' && operation.args[0] !== 28,
).at(-1).args[0];
const operationsBeforeUpdate = firstFrameOperations.length;
trafficWithStableClock.sockets[0].onmessage({data: JSON.stringify({
  ...snapshot,
  generatedAt: '2026-09-14T12:00:08.000Z',
  trafficHistory: [
    stableTrafficHistory[0],
    stableTrafficHistory[stableTrafficHistory.length - 1],
    {checkedAt: '2026-09-14T12:00:08.000Z', transmitBps: 750_000, receiveBps: 1_750_000},
  ],
})});
trafficWithStableClock.runFrame(1017);
const secondFrameSeriesX = firstFrameOperations.slice(operationsBeforeUpdate).filter(
  (operation) => operation.name === 'moveTo' && operation.args[0] !== 28,
).at(-1).args[0];
assert.ok(
  firstFrameSeriesX - secondFrameSeriesX > 0
    && firstFrameSeriesX - secondFrameSeriesX < 0.2,
  'pruning overlapping history must not reset or move the rendering clock',
);

const allParts = loadPage(pages.traffic, '?parts=values,graph');
assert.equal(allParts.body.dataset.parts, 'all');
const invalidParts = loadPage(pages.latency, '?parts=unknown');
assert.equal(invalidParts.body.dataset.parts, 'all');

console.log('Web UI tests passed.');

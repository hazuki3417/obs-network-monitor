const {expect, test} = require('@playwright/test');

const generatedAt = Date.parse('2026-09-19T12:00:00.000Z');

function sampleTime(secondsAgo) {
  return new Date(generatedAt - secondsAgo * 1000).toISOString();
}

const history = Array.from({length: 70}, (_, index) => {
  const secondsAgo = 69 - index;
  const failed = index === 24 || index === 25;
  return {
    checkedAt: sampleTime(secondsAgo),
    method: 'icmp',
    success: !failed,
    rttMs: failed ? null : Math.round(27 + Math.sin(index / 5) * 8 + (index % 17 === 0 ? 25 : 0)),
  };
});

const trafficHistory = Array.from({length: 70}, (_, index) => {
  const secondsAgo = 69 - index;
  return {
    checkedAt: sampleTime(secondsAgo),
    transmitBps: Math.round(700_000 + Math.sin(index / 6) * 280_000 + (index % 19 === 0 ? 500_000 : 0)),
    receiveBps: Math.round(4_600_000 + Math.cos(index / 7) * 1_400_000 + (index % 23 === 0 ? 2_000_000 : 0)),
  };
});

const snapshot = {
  generatedAt: new Date(generatedAt).toISOString(),
  traffic: {transmitBps: 824_000, receiveBps: 5_780_000},
  statistics: {
    latestLatencyMs: 27,
    averageLatencyMs: 29.8,
    minimumLatencyMs: 18,
    maximumLatencyMs: 61,
    jitterMs: 4.6,
    failureMetric: 'packetLoss',
    failureRatePercent: 1.7,
    consecutiveFailures: 0,
  },
  history,
  trafficHistory,
  route: {
    status: 'complete',
    checkedAt: sampleTime(8),
    hopCount: 10,
    maxNodes: 6,
    hops: [
      {number: 1, responded: true, rttMs: 2, deltaRttMs: null, target: false},
      {number: 2, responded: true, rttMs: 4, deltaRttMs: 2, target: false},
      {number: 3, responded: true, rttMs: 7, deltaRttMs: 3, target: false},
      {number: 4, responded: true, rttMs: 31, deltaRttMs: 24, target: false},
      {number: 5, responded: true, rttMs: 35, deltaRttMs: 4, target: false},
      {number: 6, responded: false, rttMs: null, deltaRttMs: null, target: false},
      {number: 7, responded: true, rttMs: 42, deltaRttMs: null, target: false},
      {number: 8, responded: true, rttMs: 68, deltaRttMs: 26, target: false},
      {number: 9, responded: true, rttMs: 71, deltaRttMs: 3, target: false},
      {number: 10, responded: true, rttMs: 73, deltaRttMs: 2, target: true},
    ],
  },
};

async function openOverlay(page, route, readySelector, readyText) {
  await page.addInitScript((fixedSnapshot) => {
    const frames = [];
    Object.defineProperty(window.performance, 'now', {value: () => 0});
    window.requestAnimationFrame = (callback) => {
      frames.push(callback);
      return frames.length;
    };
    window.cancelAnimationFrame = () => {};
    window.__runVisualFrame = () => {
      const pending = frames.splice(0);
      pending.forEach((callback) => callback(0));
    };

    class MockWebSocket {
      constructor(url) {
        this.url = url;
        this.readyState = 0;
        window.setTimeout(() => {
          this.readyState = 1;
          this.onopen?.({});
          this.onmessage?.({data: JSON.stringify(fixedSnapshot)});
        }, 0);
      }

      close() {
        this.readyState = 3;
      }
    }

    window.WebSocket = MockWebSocket;
  }, snapshot);
  await page.goto(route);
  await page.addStyleTag({content: 'html, body { background: #111827 !important; }'});
  await expect(page.locator(readySelector)).toContainText(readyText);
  await page.evaluate(() => window.__runVisualFrame());
}

const cases = [
  {name: 'latency', route: '/latency', viewport: {width: 480, height: 270}, ready: '#latency', readyText: '27'},
  {name: 'traffic', route: '/traffic', viewport: {width: 480, height: 210}, ready: '#tx-traffic', readyText: '824'},
  {name: 'route', route: '/route', viewport: {width: 480, height: 340}, ready: '#route-nodes', readyText: 'TARGET'},
];

for (const example of cases) {
  test(`${example.name} overlay`, async ({page}) => {
    await page.setViewportSize(example.viewport);
    await openOverlay(page, example.route, example.ready, example.readyText);
    await expect(page).toHaveScreenshot(`${example.name}.png`, {
      animations: 'disabled',
      threshold: 0.2,
      maxDiffPixelRatio: 0.02,
    });
  });
}

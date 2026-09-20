const {defineConfig} = require('@playwright/test');

module.exports = defineConfig({
  testDir: './tests/visual',
  fullyParallel: false,
  retries: process.env.CI ? 1 : 0,
  reporter: 'line',
  snapshotPathTemplate: 'docs/images/{arg}{ext}',
  use: {
    baseURL: 'http://127.0.0.1:4173',
    browserName: 'chromium',
    colorScheme: 'dark',
    deviceScaleFactor: 1,
    locale: 'ja-JP',
    timezoneId: 'UTC',
    trace: 'retain-on-failure',
    ...(process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE
      ? {launchOptions: {executablePath: process.env.PLAYWRIGHT_CHROMIUM_EXECUTABLE}}
      : {}),
  },
  webServer: {
    command: 'node .github/scripts/serve-web.js',
    url: 'http://127.0.0.1:4173/latency',
    reuseExistingServer: !process.env.CI,
    timeout: 10_000,
  },
});

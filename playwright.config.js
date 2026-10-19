const { defineConfig, devices } = require('@playwright/test');

module.exports = defineConfig({
  testDir: './e2e',
  fullyParallel: false,
  workers: 1,
  retries: 0,
  reporter: [['list'], ['html', { outputFolder: 'playwright-report', open: 'never' }]],
  use: {
    baseURL: 'http://127.0.0.1:8088',
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    ...devices['Desktop Chrome'],
  },
  webServer: [{
    command: 'node ./e2e/start-server.js',
    url: 'http://127.0.0.1:8088/api/session',
    reuseExistingServer: false,
    timeout: 120_000,
  }, {
    command: 'node ./e2e/start-server.js --exploration',
    url: 'http://127.0.0.1:8089/api/session',
    reuseExistingServer: false,
    timeout: 120_000,
  }, {
    command: 'node ./e2e/start-server.js --workspace',
    url: 'http://127.0.0.1:8090/api/session',
    reuseExistingServer: false,
    timeout: 120_000,
  }],
});

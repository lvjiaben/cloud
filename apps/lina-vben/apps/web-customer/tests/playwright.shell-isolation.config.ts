import { defineConfig } from '@playwright/test';

const customerBaseURL = 'http://127.0.0.1:6032';
const adminBaseURL = 'http://127.0.0.1:6031/admin/';

export default defineConfig({
  expect: { timeout: 10_000 },
  fullyParallel: false,
  projects: [
    {
      name: process.env.E2E_BROWSER_CHANNEL?.trim() || 'chrome',
      use: {
        browserName: 'chromium',
        channel: process.env.E2E_BROWSER_CHANNEL?.trim() || 'chrome',
      },
    },
  ],
  reporter: 'list',
  testDir: './e2e',
  testMatch: /TC028-shell-isolation\.ts$/u,
  timeout: 120_000,
  use: {
    baseURL: customerBaseURL,
    headless: true,
    locale: 'zh-CN',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  webServer: [
    {
      command: 'pnpm dev --host 127.0.0.1',
      cwd: '..',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      url: customerBaseURL,
    },
    {
      command: 'pnpm dev --host 127.0.0.1',
      cwd: '../../web-antd',
      reuseExistingServer: !process.env.CI,
      timeout: 120_000,
      url: adminBaseURL,
    },
  ],
  workers: 1,
});

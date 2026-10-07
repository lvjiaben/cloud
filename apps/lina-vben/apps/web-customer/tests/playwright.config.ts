import { defineConfig } from '@playwright/test';

const baseURL =
  process.env.E2E_CUSTOMER_BASE_URL?.trim() || 'http://127.0.0.1:6032';

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
  // TC028 deliberately starts both Vben applications and has its own
  // two-server config so the ordinary customer suite stays single-process.
  testIgnore: /TC028-shell-isolation\.ts$/u,
  testMatch: /TC\d{3}-[^.]+\.ts$/u,
  timeout: 120_000,
  use: {
    baseURL,
    headless: true,
    locale: 'zh-CN',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
  },
  webServer: {
    command: 'pnpm dev --host 127.0.0.1',
    cwd: '..',
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    url: baseURL,
  },
  workers: 1,
});

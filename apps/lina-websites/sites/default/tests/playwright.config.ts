import { defineConfig } from '@playwright/test';

import { testRuntime } from './support/runtime-config';

const browserChannel = process.env.E2E_BROWSER_CHANNEL?.trim() || 'chrome';
const serverURL = new URL(testRuntime.baseURL);

export default defineConfig({
  expect: { timeout: 10_000 },
  fullyParallel: false,
  outputDir: './test-results',
  projects: [
    {
      name: browserChannel,
      use: {
        browserName: 'chromium',
        channel: browserChannel,
      },
    },
  ],
  reporter: [
    ['list'],
    ['html', { open: 'never', outputFolder: './playwright-report' }],
  ],
  retries: Number.parseInt(process.env.E2E_RETRIES ?? (process.env.CI ? '1' : '0'), 10),
  testDir: './e2e',
  testMatch: /TC\d{3}-[^.]+\.ts$/u,
  timeout: 180_000,
  tsconfig: './tsconfig.json',
  use: {
    baseURL: testRuntime.baseURL,
    headless: true,
    locale: 'zh-CN',
    screenshot: 'only-on-failure',
    trace: 'retain-on-failure',
    video: 'retain-on-failure',
  },
  webServer: {
    command: 'node ../.output/server/index.mjs',
    env: {
      NITRO_HOST: serverURL.hostname,
      NITRO_PORT: serverURL.port || '6680',
      NUXT_API_ORIGIN:
        process.env.NUXT_API_ORIGIN?.trim() || 'http://127.0.0.1:9999',
      NUXT_PUBLIC_CUSTOMER_URL: testRuntime.customerURL.replace(/\/$/u, ''),
      NUXT_PUBLIC_I18N_BASE_URL: testRuntime.baseURL.replace(/\/$/u, ''),
      NUXT_PUBLIC_SITE_URL: testRuntime.baseURL.replace(/\/$/u, ''),
    },
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    url: testRuntime.baseURL,
  },
  workers: Number.parseInt(process.env.E2E_WORKERS ?? '1', 10),
});

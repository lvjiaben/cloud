import { defineConfig } from "@playwright/test";
import { resolve } from "node:path";
const evidence = resolve(
  process.cwd(),
  "../../../temp/20261023",
  `${Date.now()}-customer-tdesign-e2e`,
);
export default defineConfig({
  testDir: "e2e",
  testMatch: /TC\d{3}.*\.ts$/,
  fullyParallel: true,
  workers: 2,
  timeout: 30000,
  outputDir: evidence,
  reporter: [["list"]],
  use: {
    baseURL: process.env.CUSTOMER_PREVIEW_URL || "http://127.0.0.1:16742",
    viewport: { width: 1440, height: 960 },
    locale: "zh-CN",
    timezoneId: "Asia/Shanghai",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: process.env.CUSTOMER_PREVIEW_URL
    ? undefined
    : {
        command: "pnpm run dev",
        url: "http://127.0.0.1:16742",
        reuseExistingServer: true,
        timeout: 60000,
      },
});

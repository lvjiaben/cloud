// Isolated website tests run without repository authentication or source imports.
import { defineConfig } from "@playwright/test";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { evidenceDirectory } from "./support/output";
const root = fileURLToPath(new URL("../..", import.meta.url));
export default defineConfig({
  testDir: "./e2e",
  testMatch: "TC*.ts",
  workers: 2,
  timeout: 30000,
  outputDir: resolve(evidenceDirectory, "website-qycn-results"),
  reporter: "list",
  use: {
    baseURL: "http://127.0.0.1:16881",
    viewport: { width: 1440, height: 900 },
    reducedMotion: "reduce",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "node scripts/preview.mjs",
    cwd: root,
    url: "http://127.0.0.1:16881",
    reuseExistingServer: false,
    env: { PORT: "16881", WEBSITE_CUSTOMER_URL: "http://127.0.0.1:6032" },
  },
});

// This helper stores public website visual evidence in the dated temp directory.
import { mkdir } from "node:fs/promises";
import path from "node:path";
import { fileURLToPath } from "node:url";

import type { Page } from "@playwright/test";

const repositoryRoot = fileURLToPath(
  new URL("../../../../../../", import.meta.url),
);

function timestampParts(now: Date) {
  const iso = now.toISOString();
  return {
    date: iso.slice(0, 10).replaceAll("-", ""),
    time: iso.slice(11, 19).replaceAll(":", ""),
  };
}

export async function capturePublicScreenshot(page: Page, description: string) {
  const { date, time } = timestampParts(new Date());
  const directory = path.join(repositoryRoot, "temp", date);
  const screenshotPath = path.join(directory, `${time}-${description}.png`);
  await mkdir(directory, { recursive: true });
  await page.screenshot({ fullPage: false, path: screenshotPath });
  return screenshotPath;
}

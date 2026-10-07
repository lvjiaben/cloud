import type { Page } from '@playwright/test';

import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const repositoryRoot = fileURLToPath(
  new URL('../../../../../../', import.meta.url),
);

function timestampParts(now: Date) {
  const iso = now.toISOString();
  return {
    date: iso.slice(0, 10).replaceAll('-', ''),
    time: iso.slice(11, 19).replaceAll(':', ''),
  };
}

export async function captureCustomerScreenshot(
  page: Page,
  description: string,
) {
  await page
    .locator('#__app-loading__')
    .waitFor({ state: 'hidden', timeout: 10_000 });
  await page
    .locator('.bg-overlay-content:visible')
    .waitFor({ state: 'hidden', timeout: 10_000 });
  const { date, time } = timestampParts(new Date());
  const directory = path.join(repositoryRoot, 'temp', date);
  const screenshotPath = path.join(directory, `${time}-${description}.png`);
  await mkdir(directory, { recursive: true });
  await page.screenshot({
    animations: 'disabled',
    fullPage: false,
    path: screenshotPath,
  });
  return screenshotPath;
}

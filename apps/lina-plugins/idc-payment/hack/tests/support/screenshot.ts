import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

import type { Page } from '@host-tests/support/playwright';

const repositoryRoot = fileURLToPath(new URL('../../../../../../', import.meta.url));

export async function capturePaymentScreenshot(page: Page, description: string) {
  await page
    .locator('#__app-loading__')
    .waitFor({ state: 'hidden', timeout: 10_000 })
    .catch(() => undefined);
  await page
    .locator('.bg-overlay-content')
    .waitFor({ state: 'hidden', timeout: 10_000 })
    .catch(() => undefined);
  const iso = new Date().toISOString();
  const date = iso.slice(0, 10).replaceAll('-', '');
  const time = iso.slice(11, 19).replaceAll(':', '');
  const directory = path.join(repositoryRoot, 'temp', date);
  await mkdir(directory, { recursive: true });
  const target = path.join(directory, `${time}-${description}.png`);
  await page.screenshot({ animations: 'disabled', fullPage: false, path: target });
  return target;
}

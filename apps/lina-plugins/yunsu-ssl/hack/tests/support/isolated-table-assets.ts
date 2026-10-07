import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
import type { Page } from '@playwright/test';

/** Serve the real built official Vben runtime, never a table mock. */
export async function installIsolatedTableAssets(page: Page, options: { failFirstRuntime?: boolean; beforeFailure?: () => Promise<void> } = {}) {
  const root = resolve(process.cwd(), '../../apps/lina-plugins/vben-adapter/frontend/dist/isolated');
  let runtimeRequests = 0;
  await page.route('**/x-assets/vben-adapter/v0.1.0/isolated/**', async route => {
    const name = new URL(route.request().url()).pathname.split('/isolated/')[1] ?? '';
    if (name === 'runtime.js') {
      runtimeRequests += 1;
      if (options.failFirstRuntime && runtimeRequests === 1) {
        await options.beforeFailure?.();
        await route.abort('failed');
        return;
      }
    }
    if (!/^[\w.-]+\.(?:css|js)$/.test(name)) { await route.abort(); return; }
    await route.fulfill({body:await readFile(resolve(root,name)),contentType:name.endsWith('.css')?'text/css; charset=utf-8':'text/javascript; charset=utf-8',headers:{'Access-Control-Allow-Origin':'*','Cache-Control':'public, max-age=31536000, immutable'}});
  });
  return { runtimeRequests: () => runtimeRequests };
}

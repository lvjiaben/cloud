import type { Page } from "@playwright/test";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";

// Clear previous runtime bundles so translation assertions use the live server.
export async function installWebsiteMessages(page: Page) {
  await page.addInitScript(() => {
    for (const key of Object.keys(localStorage)) {
      if (key.startsWith("linapro:i18n:runtime:")) localStorage.removeItem(key);
    }
  });
  // Use the current plugin bundle while keeping unrelated host translations unchanged.
  await page.route('**/api/v1/i18n/runtime/messages*', async route => {
    const headers = {...route.request().headers()};
    delete headers['if-none-match'];
    delete headers['if-modified-since'];
    const response = await route.fetch({headers});
    const payload = await response.json();
    const locale = new URL(route.request().url()).searchParams.get('lang') === 'en-US' ? 'en-US' : 'zh-CN';
    const messages = payload.data?.messages ?? {};
    const source = JSON.parse(await readFile(resolve(__dirname, `../../../manifest/i18n/${locale}/plugin.json`), 'utf8'));
    for (const [key,value] of Object.entries(source)) {
      const parts=key.split('.');let target=messages;
      for (const part of parts.slice(0,-1)) {target[part]??={};target=target[part];}
      target[parts.at(-1)!]=value;
    }
    await route.fulfill({json:{...payload,data:{...payload.data,messages}}});
  });
}

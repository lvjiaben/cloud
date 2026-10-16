import type { Page } from "@playwright/test";

// Clear previous runtime bundles so translation assertions use the live server.
export async function installWebsiteMessages(page: Page) {
  await page.addInitScript(() => {
    for (const key of Object.keys(localStorage)) {
      if (key.startsWith("linapro:i18n:runtime:")) localStorage.removeItem(key);
    }
  });
}

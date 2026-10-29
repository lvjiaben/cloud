// Authenticate an isolated merchant context without rewriting host-owned
// storage-state files or relying on a token left by a previous test run.
import {resolve} from 'node:path';
import { test as hostTest, expect } from '@host-tests/fixtures/auth';
import { config } from '@host-tests/fixtures/config';
import { LoginPage } from '@host-tests/pages/LoginPage';

export const test = hostTest.extend({
  adminContext: async ({ browser }, use) => {
    const context = await browser.newContext({ baseURL: config.baseURL });
    const isolatedEntry=process.env.IDC_ISOLATED_ADMIN_ENTRY;
    if(isolatedEntry) {
      const path=resolve(isolatedEntry);
      if(!path.startsWith(resolve('temp')+'/') || ['6030','6031','6032','6680'].includes(new URL(config.baseURL).port)) throw new Error('Admin entry override requires isolated runtime');
      await context.route(new URL('/',config.baseURL).href,route=>route.fulfill({path,contentType:'text/html'}));
    }
    const page = await context.newPage();
    try {
      const login = new LoginPage(page);
      await login.goto();
      const authenticated = page.waitForResponse(response => new URL(response.url()).pathname === '/api/v1/auth/login' && response.request().method() === 'POST');
      await login.login(config.adminUser, config.adminPass);
      expect((await (await authenticated).json()).code).toBe(0);
      await page.waitForURL(url => url.hash.startsWith('#/') ? !url.hash.includes('/auth/') : !url.pathname.includes('/auth/'));
      await page.close();
      await use(context);
    } finally { await context.close(); }
  },
});
export { expect };

// Mount the shipped customer page in a browser without requiring a live account.
// The real API adapter and translations remain in use; only transport and layout
// are isolated, so retired writes cannot be hidden by a server rejection.
import { test, expect } from '@host-tests/fixtures/auth';
import { createRequire } from 'node:module';
import { readFile, mkdir } from 'node:fs/promises';
import { fileURLToPath, pathToFileURL } from 'node:url';
import path from 'node:path';

const pluginRoot = fileURLToPath(new URL('../../..', import.meta.url));
const appRoot = path.join(pluginRoot, 'frontend/customer');
const repoRoot = path.resolve(pluginRoot, '../../..');
let server: { close(): Promise<void> };
let origin = '';

// The app's locked workspace supplies its compiler; no external network or host
// frontend source is needed to execute this template-owned regression.
test.beforeAll(async () => {
  const appRequire = createRequire(path.join(appRoot, 'package.json'));
  const compilerRequire = createRequire(path.join(appRoot, 'vendor/vben/internal/vite-config/package.json'));
  const { createServer } = await import(pathToFileURL(appRequire.resolve('vite')).href);
  const { default: vue } = await import(pathToFileURL(compilerRequire.resolve('@vitejs/plugin-vue')).href);
  const locales = Object.fromEntries(await Promise.all(['zh-CN', 'en-US'].map(async locale => [
    locale, JSON.parse(await readFile(path.join(appRoot, 'src/locales/langs', locale, 'salesChannel.json'), 'utf8')),
  ])));
  const instance = await createServer({
    root: appRoot,
    configFile: false,
    logLevel: 'error',
    optimizeDeps: { noDiscovery: true, include: ['vue', 'vue-router', 'ant-design-vue'] },
    plugins: [vue(), {
      name: 'commission-browser-fixture',
      enforce: 'pre',
      resolveId(id: string) {
        if (['virtual:commission-entry', '#/api/request', '#/locales', '@vben/common-ui'].includes(id)) return '\0' + id.replace('#/api/request', 'fixture-request').replace('#/locales', 'fixture-locales');
        return null;
      },
      load(id: string) {
        if (id === '\0virtual:commission-entry') return `
          import { createApp } from 'vue';
          import { createRouter, createWebHistory } from 'vue-router';
          import Page from '/src/views/account/sales-channel/index.vue';
          import 'ant-design-vue/dist/reset.css';
          const router = createRouter({history:createWebHistory(),routes:[{path:'/:pathMatch(.*)*',name:'CustomerCommissions',component:Page}]});
          createApp(Page).use(router).mount('#app');`;
        if (id === '\0@vben/common-ui') return `import {h} from 'vue'; export const Page={setup:(_, {slots})=>()=>h('main',slots.default?.())};`;
        if (id === '\0fixture-request') return `
          export const pluginApiPath=(plugin,path)=>'/x/'+plugin+'/api/v1/'+path;
          export const requestClient={get:async path=>{const response=await fetch(path);if(!response.ok)throw new Error('request failed');return (await response.json()).data;}};`;
        if (id === '\0fixture-locales') return `
          const messages=${JSON.stringify(locales)};
          export const $t=key=>key.split('.').slice(1).reduce((value,part)=>value?.[part],messages[new URL(location.href).searchParams.get('lang')||'zh-CN'])||key;`;
        return null;
      },
      configureServer(vite: { middlewares: { use(handler: unknown): void } }) {
        vite.middlewares.use((req: { url?: string }, res: { setHeader(key: string, value: string): void; end(value: string): void }, next: () => void) => {
          if (!req.url?.startsWith('/commission-check')) return next();
          res.setHeader('Content-Type', 'text/html');
          res.end('<div id="app"></div><script type="module" src="/@id/__x00__virtual:commission-entry"></script>');
        });
      },
    }],
    server: { host: '127.0.0.1', port: 0 },
  });
  await instance.listen();
  server = instance;
  const address = instance.httpServer.address();
  origin = `http://127.0.0.1:${address.port}`;
});
test.afterAll(async () => { await server?.close(); });

test.describe('TC-1 Customer commission records', () => {
  for (const locale of ['zh-CN', 'en-US']) {
    test(`TC-1 ${locale}: offline settlements stay readable without a request form`, async ({ page }) => {
      const writes: string[] = [];
      const pageErrors: string[] = [];
      page.on('pageerror', error => pageErrors.push(error.message));
      const evidence = path.join(repoRoot, 'temp/20261016');
      await mkdir(evidence, { recursive: true });
      let failed = true;
      await page.route('**/x/idc-core/api/v1/account/sales-channel/**', async route => {
        if (route.request().method() !== 'GET') writes.push(route.request().url());
        if (failed) return route.fulfill({ status: 503, json: {} });
        const endpoint = new URL(route.request().url()).pathname;
        const data = endpoint.endsWith('/referral') ? { referralCode: 'REF_DEMO', referralUrl: 'https://example.test/ref/DEMO' }
          : endpoint.endsWith('/overview') ? { accounts: [{ currencyCode: 'CNY', availableAmountMinor: '12500', pendingAmountMinor: '0', maturedAmountMinor: '12500', frozenAmountMinor: '0' }], entries: [{ entryNo: 'COM_DEMO', currencyCode: 'CNY', amountMinor: '12500', statusCode: 'matured', createdAt: 1792108800000 }] }
          : { items: [{ settlementNo: 'SETTLED_DEMO', currencyCode: 'CNY', amountMinor: '5000', statusCode: 'paid', submittedAt: 1792108800000 }] };
        return route.fulfill({ json: { code: 0, data } });
      });
      await page.goto(`${origin}/commission-check?lang=${locale}`);
      await expect(page.getByRole('alert')).toContainText(locale === 'zh-CN' ? '推荐或佣金数据暂时无法加载' : 'Referral or commission data is temporarily unavailable');
      await page.screenshot({ path: path.join(evidence, `140000-commission-error-${locale}.png`), fullPage: true });
      failed = false;
      await page.getByTestId('customer-sales-refresh').click();
      await expect(page.getByTestId('customer-commission-CNY')).toContainText('125.00');
      await expect(page.locator('main')).toContainText('COM_DEMO');
      await expect(page.locator('main')).toContainText('SETTLED_DEMO');
      await expect(page.locator('main')).toContainText(locale === 'zh-CN' ? '结算记录' : 'Settlement records');
      await expect(page.locator('main')).toContainText(locale === 'zh-CN' ? '管理员线下打款' : 'Administrators pay available commissions offline');
      await expect(page.getByTestId('customer-commission-settlement')).toHaveCount(0);
      await expect(page.getByRole('combobox')).toHaveCount(0);
      await expect(page.locator('input')).toHaveCount(0);
      expect(writes).toEqual([]);
      expect(pageErrors).toEqual([]);
      await page.screenshot({ path: path.join(evidence, `140000-commission-records-${locale}.png`), fullPage: true });
    });
  }
});

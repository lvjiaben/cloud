import { test, expect } from '@host-tests/fixtures/auth';
import { createServer } from 'node:http';
import { spawn, type ChildProcess } from 'node:child_process';
import { once } from 'node:events';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';
import { AnnouncementWebsitePage } from '../pages/AnnouncementWebsitePage';
const templateID = process.env.E2E_WEBSITE_TEMPLATE || 'website-default';
if (!['website-default', 'website-minimal'].includes(templateID)) throw new Error('Unsupported template');
const pluginRoot = resolve(fileURLToPath(new URL('../../../../', import.meta.url)), templateID);
let origin = '', renderer: ChildProcess;
let announcementRequests = 0, retiredRequests = 0;
const item = { id: 321, title: '维护公告', createdAt: 1792640000000, updatedAt: 1792640000000, sortOrder: 0, content: '<p>今晚进行服务维护。</p><img src="https://example.test/announcement.png" alt="公告配图" onerror="window.announcementInjected=true"><script>window.announcementInjected=true</script>' };
const api = createServer((req, res) => {
  const url = new URL(req.url!, 'http://local');
  let data: unknown;
  if (url.pathname.endsWith('/public/brand')) data = { brand: { siteName: { zhCN: '云服务平台', enUS: 'Cloud services' }, footer: { zhCN: '服务支持', enUS: 'Support' }, navigation: [], footerNavigation: [] } };
  else if (url.pathname === '/x/content-notice/api/v1/public/announcements') {
    announcementRequests++;
    const page = Number(url.searchParams.get('pageNum') || 1);
    const pageSize = Number(url.searchParams.get('pageSize') || 20);
    data = { list: [{ ...item, id: page === 1 ? 321 : 322, title: page === 1 ? '维护公告' : '后续公告' }], total: pageSize === 100 ? 1 : 21 };
  } else if (url.pathname === '/x/content-notice/api/v1/public/announcements/321') { announcementRequests++; data = item; }
  else if (url.pathname === '/x/content-notice/api/v1/public/announcements/999') { res.end(JSON.stringify({ code: 65, data: null, errorCode: 'CONTENT_NOTICE_NOT_FOUND', message: 'Announcement not found' })); return; }
  else if (url.pathname.endsWith('/content/items')) { if (url.searchParams.get('type') === 'announcement') retiredRequests++; data = { items: [], nextCursor: '' }; }
  else if (url.pathname.endsWith('/campaigns')) data = { campaigns: [] };
  else { res.statusCode = 404; res.end(JSON.stringify({ code: 404 })); return; }
  res.setHeader('content-type', 'application/json'); res.end(JSON.stringify({ code: 0, data }));
});

test.describe('TC-4 官网统一公告', () => {
  test.beforeAll(async () => {
    api.listen(0, '127.0.0.1'); await once(api, 'listening');
    const apiOrigin = `http://127.0.0.1:${(api.address() as any).port}`;
    const probe = createServer(); probe.listen(0, '127.0.0.1'); await once(probe, 'listening'); const port = (probe.address() as any).port; await new Promise<void>(done => probe.close(() => done()));
    origin = `http://127.0.0.1:${port}`;
    renderer = spawn(process.execPath, [resolve(pluginRoot, 'frontend/site/.output/server/index.mjs')], { env: { ...process.env, PORT: String(port), HOST: '127.0.0.1', NUXT_API_ORIGIN: apiOrigin }, stdio: 'ignore' });
    await expect.poll(async () => { try { return (await fetch(origin + '/__site/health')).status; } catch { return 0; } }, { timeout: 20000 }).toBe(200);
  });
  test.afterAll(async () => { renderer?.kill(); await new Promise<void>(done => api.close(() => done())); });
  test('TC-4a: SSR与客户端分页、图片详情和sitemap都来自统一公告', async ({ page }, testInfo) => {
    await page.route('https://example.test/announcement.png', r => r.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="120"><rect width="400" height="120" fill="#edf4ff"/><text x="24" y="68" font-size="24" fill="#1677ff">Service maintenance</text></svg>' }));
    const html = await (await fetch(origin + '/pages/announcements/321')).text();
    expect(html).toContain('今晚进行服务维护。');
    expect(html).toContain('<img src="https://example.test/announcement.png" alt="公告配图"');
    expect(html).not.toContain('onerror="window.announcementInjected');
    const view = new AnnouncementWebsitePage(page, origin);
    await view.open();
    await expect(page.getByTestId('public-content-list')).toContainText('维护公告');
    await view.next();
    await expect(page.getByTestId('public-content-list')).toContainText('后续公告');
    await view.open();
    await view.select('维护公告');
    await expect(page.getByTestId('public-content-detail')).toContainText('今晚进行服务维护。');
    await expect(page.getByRole('img', { name: '公告配图' })).toBeVisible();
    expect(await page.evaluate(() => (window as any).announcementInjected)).toBeUndefined();
    await page.screenshot({ path: testInfo.outputPath(`${Date.now()}-${templateID}-announcement-detail.png`), animations: 'disabled' });
    const sitemap = await (await fetch(origin + '/sitemap.xml')).text();
    expect(sitemap).toContain('/pages/announcements/321');
    expect(announcementRequests).toBeGreaterThan(0);
    expect(retiredRequests).toBe(0);
    expect((await fetch(origin + '/pages/announcements/999')).status).toBe(404);
  });
  test('TC-4b: 真实source插件命名空间返回公开数据与登录保护，宿主旧路径不可访问', async () => {
    test.skip(!process.env.ANNOUNCEMENT_REAL_API_ORIGIN, 'Set ANNOUNCEMENT_REAL_API_ORIGIN to validate a running backend.');
    const realOrigin = process.env.ANNOUNCEMENT_REAL_API_ORIGIN!;
    const publicResponse = await fetch(`${realOrigin}/x/content-notice/api/v1/public/announcements?pageNum=1&pageSize=1`);
    expect(publicResponse.status).toBe(200);
    const payload = await publicResponse.json();
    expect(payload.code).toBe(0);
    expect(Array.isArray(payload.data.list)).toBe(true);
    expect(typeof payload.data.total).toBe('number');
    expect((await fetch(`${realOrigin}/api/v1/public/announcements?pageNum=1&pageSize=1`)).status).toBe(404);
    expect((await fetch(`${realOrigin}/x/content-notice/api/v1/notice/announcements?pageNum=1&pageSize=1`)).status).toBe(401);
  });
});

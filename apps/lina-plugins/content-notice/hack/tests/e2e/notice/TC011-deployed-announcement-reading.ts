import { test, expect } from '@host-tests/fixtures/auth';
import { request as playwrightRequest } from '@host-tests/support/playwright';
import { createAdminApiContext, expectSuccess } from '@host-tests/support/api/job';
import { pluginApiPath, config } from '@host-tests/fixtures/config';
import { readFile, mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { CustomerPage } from '../../../../../customer-tdesign/hack/tests/pages/CustomerPage';

// This integration test deliberately uses real API routes and a dedicated local customer session.
test.describe('TC-11 已部署公告真实图片与阅读', () => {
  test('TC-11a: 真实上传图片后客户可读，阅读刷新后持久化', async ({ browser, request }) => {
    test.skip(!process.env.LINA_CUSTOMER_SESSION_FILE, '需要本地专用验收客户会话');
    const session = JSON.parse(await readFile(process.env.LINA_CUSTOMER_SESSION_FILE!, 'utf8'));
    const admin = await createAdminApiContext();
    const reader = await playwrightRequest.newContext({ baseURL: config.backendBaseURL, extraHTTPHeaders: { Authorization: `Bearer ${session.accessToken}` } });
    const context = await browser.newContext({ baseURL: process.env.E2E_CUSTOMER_BASE_URL || 'http://127.0.0.1:6032', viewport: {width:1440,height:960} });
    await context.addInitScript(value => sessionStorage.setItem('customer-tdesign.session', JSON.stringify(value)), session);
    const page = await context.newPage();
    const view = new CustomerPage(page);
    const screenshots = fileURLToPath(new URL('../../../../../../../temp/20261028/', import.meta.url));
    const calls: string[] = [];
    page.on('request', req => { if (req.url().includes('/announcements')) calls.push(new URL(req.url()).pathname); });
    let fileId = 0, noticeId = 0;
    const title = `真实图片公告_${Date.now()}`;
    try {
      const image = Buffer.concat([await readFile(fileURLToPath(new URL('../../../../../../../apps/lina-vben/apps/web-antd/public/logo.png', import.meta.url))), Buffer.from(String(Date.now()))]);
      const upload = await expectSuccess<{id:number;url:string}>(await admin.post('file/upload', {multipart:{scene:'notice_image',file:{name:`announcement-${Date.now()}.png`,mimeType:'image/png',buffer:image}}}));
      fileId = upload.id;
      const publicImage = await request.get(new URL(upload.url, config.backendBaseURL).href);
      expect(publicImage.ok()).toBe(true);
      expect(publicImage.headers()['content-type']).toContain('image');
      noticeId = (await expectSuccess<{id:number}>(await admin.post(pluginApiPath('content-notice','notice'),{data:{title,content:`<p>真实公告正文</p><img src="${upload.url}" alt="真实上传公告图片">`,status:1,sortOrder:0}}))).id;
      const detailPath = pluginApiPath('content-notice',`notice/announcements/${noticeId}`);
      expect((await expectSuccess<{read:boolean}>(await reader.get(detailPath))).read).toBe(false);
      await view.open('/account/ssh-keys');
      await expect(page.getByRole('heading', {name:'登录密钥',exact:true})).toBeVisible();
      await view.openAnnouncements();
      await expect(view.announcements.getByRole('button').filter({hasText:title})).toContainText('未读');
      await view.openAnnouncement(title);
      await expect(view.announcementDetail).toContainText('真实公告正文');
      const uploadedImage = view.announcementDetail.getByAltText('真实上传公告图片');
      await expect(uploadedImage).toBeVisible();
      await expect.poll(() => uploadedImage.evaluate(image => image instanceof HTMLImageElement && image.complete && image.naturalWidth > 0)).toBe(true);
      await expect.poll(async () => (await expectSuccess<{read:boolean}>(await reader.get(detailPath))).read).toBe(true);
      await mkdir(screenshots,{recursive:true});
      await page.screenshot({path:`${screenshots}/${Date.now()}-announcement-live-image.png`,fullPage:true,animations:'disabled'});
      await view.reload();
      await expect(page.getByRole('heading', {name:'登录密钥',exact:true})).toBeVisible();
      await view.openAnnouncements();
      await expect(view.announcements.getByRole('button').filter({hasText:title})).toContainText('已读');
      expect(calls.length).toBeGreaterThan(0);
      expect(calls.every(path => path.startsWith('/x/content-notice/api/v1/notice/announcements'))).toBe(true);
      await page.screenshot({path:`${screenshots}/${Date.now()}-announcement-live-read.png`,fullPage:true,animations:'disabled'});
    } finally {
      await context.close();
      if(noticeId) await expectSuccess(await admin.delete(pluginApiPath('content-notice',`notice?ids[]=${noticeId}`)));
      if(fileId) await expectSuccess(await admin.delete(`file?ids[]=${fileId}`));
      await reader.dispose();
      await admin.dispose();
    }
  });
  test('TC-11b: 运行官网网关和两套SSR读取同一公告且删除后404', async ({ request }) => {
    test.skip(process.env.LINA_DEPLOYED_WEBSITE_TESTS !== '1', '需要本地运行的官网网关和两套SSR');
    const admin = await createAdminApiContext();
    let id = 0;
    const title = `官网真实公告_${Date.now()}`;
    try {
      id = (await expectSuccess<{id:number}>(await admin.post(pluginApiPath('content-notice','notice'),{data:{title,content:'<p>官网真实公告正文</p>',status:1}}))).id;
      const gateway = process.env.E2E_WEBSITE_BASE_URL || 'http://127.0.0.1:6680';
      const list = await expectSuccess<{list:Array<{id:number}>}>(await request.get(`${gateway}${pluginApiPath('content-notice','public/announcements')}`));
      expect(list.list.some(item => item.id === id)).toBe(true);
      const renderers = (process.env.E2E_WEBSITE_SSR_ORIGINS || 'http://127.0.0.1:6681,http://127.0.0.1:6682').split(',');
      for (const origin of renderers) {
        const response = await request.get(`${origin}/pages/announcements/${id}`);
        expect(response.status()).toBe(200);
        const html = await response.text();
        expect(html).toContain(title);
        expect(html).toContain('官网真实公告正文');
      }
      await expectSuccess(await admin.delete(pluginApiPath('content-notice',`notice?ids[]=${id}`)));
      for (const origin of renderers) expect((await request.get(`${origin}/pages/announcements/${id}`)).status()).toBe(404);
      id = 0;
    } finally {
      if(id) await expectSuccess(await admin.delete(pluginApiPath('content-notice',`notice?ids[]=${id}`)));
      await admin.dispose();
    }
  });

});

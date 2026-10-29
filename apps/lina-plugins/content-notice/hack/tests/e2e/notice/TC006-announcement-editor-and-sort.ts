import { test, expect } from '@host-tests/fixtures/auth';
import { pluginApiPath } from '@host-tests/fixtures/config';
import { createAdminApiContext, expectSuccess } from '@host-tests/support/api/job';
import { NoticePage } from '../../pages/NoticePage';
import { mkdir } from 'node:fs/promises';
import { resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const screenshots = fileURLToPath(new URL('../../../../../../../temp/20261028/', import.meta.url));
async function screenshot(page: import('@host-tests/support/playwright').Page, name: string) {
  await mkdir(screenshots, { recursive: true });
  await page.locator('.notice-editor-drawer:visible').evaluateAll(async nodes => {
    await Promise.all(nodes.flatMap(node => node.getAnimations().map(animation => animation.finished.catch(() => {}))));
  });
  await page.screenshot({ path: resolve(screenshots, `${Date.now()}-notice-${name}.png`), fullPage: true, animations: 'disabled' });
}

test.describe('TC-6 公告编辑与直接排序', () => {
  test('TC-6a: 中文表单无类型附件范围，显示遮罩阴影动画和正文提示', async ({ adminPage }) => {
    const notice = new NoticePage(adminPage);
    await notice.goto();
    expect(await adminPage.getByTestId('notice-management-page').evaluate(el => getComputedStyle(el).animationName)).toContain('notice-page-in');
    await screenshot(adminPage, 'list');
    await notice.add();
    const drawer = notice.modal;
    await expect(drawer.getByText('公告类型', { exact: true })).toHaveCount(0);
    await expect(drawer.getByText('附件', { exact: true })).toHaveCount(0);
    await expect(drawer.getByText('可见范围', { exact: true })).toHaveCount(0);
    await expect(drawer.locator('[data-placeholder="请输入公告内容"]')).toBeVisible();
    await expect(drawer.getByText('排序', { exact: true })).toBeVisible();
    await expect(adminPage.locator('[data-dismissable-drawer].bg-overlay').first()).toBeVisible();
    const styles = await drawer.evaluate(el => ({ shadow: getComputedStyle(el).boxShadow, animation: getComputedStyle(el).animationName }));
    expect(styles.shadow).not.toBe('none');
    expect(styles.animation).toBe('notice-drawer-in');
    await screenshot(adminPage, 'drawer-zh');
  });

  test('TC-6b: 英文正文和排序提示已翻译', async ({ adminPage, mainLayout }) => {
    await mainLayout.switchLanguage('English');
    const notice = new NoticePage(adminPage);
    await notice.goto();
    await notice.add();
    await expect(notice.modal.locator('[data-placeholder="Please enter the announcement content"]')).toBeVisible();
    await expect(notice.modal.getByText('Sort order', { exact: true })).toBeVisible();
    await expect(notice.modal.getByText('Notice Type', { exact: true })).toHaveCount(0);
    await expect(notice.modal.getByText('Attachments', { exact: true })).toHaveCount(0);
    await screenshot(adminPage, 'drawer-en');
  });

  test('TC-6c: 表格输入排序持久化，失败恢复原值', async ({ adminPage }) => {
    const api = await createAdminApiContext();
    const title = `排序测试_${Date.now()}`;
    let id = 0;
    try {
      id = (await expectSuccess<{ id: number }>(await api.post(pluginApiPath('content-notice', 'notice'), { data: { title, content: '<p>排序正文</p>', status: 1, sortOrder: 40 } }))).id;
      const notice = new NoticePage(adminPage);
      await notice.goto();
      await notice.fillSearchField('公告标题', title);
      await notice.clickSearch();
      await expect(notice.sortInput(title)).toHaveValue('40');
      await screenshot(adminPage, 'filter');
      await notice.setSort(title, '3');
      await expect.poll(async () => (await expectSuccess<{ sortOrder: number }>(await api.get(pluginApiPath('content-notice', `notice/${id}`)))).sortOrder).toBe(3);
      await adminPage.reload();
      await expect(notice.sortInput(title)).toHaveValue('3');
      await screenshot(adminPage, 'sort-saved');
      const update = `**/x/content-notice/api/v1/notice/${id}`;
      await adminPage.route(update, async route => route.request().method() === 'PUT' ? route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ code: 50, message: 'Unable to save' }) }) : route.continue());
      await notice.setSort(title, '8');
      await expect(adminPage.getByText('排序保存失败，请重试', { exact: true })).toBeVisible();
      await expect(notice.sortInput(title)).toHaveValue('3');
      expect((await expectSuccess<{ sortOrder: number }>(await api.get(pluginApiPath('content-notice', `notice/${id}`)))).sortOrder).toBe(3);
      await screenshot(adminPage, 'sort-failed');
      await adminPage.unroute(update);
      const list = '**/x/content-notice/api/v1/notice?*';
      await adminPage.route(list, route => route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ code: 50, message: 'Unable to refresh' }) }));
      await notice.setSort(title, '5');
      await expect.poll(async () => (await expectSuccess<{ sortOrder: number }>(await api.get(pluginApiPath('content-notice', `notice/${id}`)))).sortOrder).toBe(5);
      await expect(notice.sortInput(title)).toHaveValue('5');
      await notice.setSort(title, '3');
      await expect.poll(async () => (await expectSuccess<{ sortOrder: number }>(await api.get(pluginApiPath('content-notice', `notice/${id}`)))).sortOrder).toBe(3);
      await adminPage.unroute(list);
    } finally {
      if (id) await expectSuccess(await api.delete(pluginApiPath('content-notice', `notice?ids[]=${id}`)));
      await api.dispose();
    }
  });
  test('TC-6d: 编辑读取失败不能变成新增或修改', async ({ adminPage }) => {
    const api = await createAdminApiContext();
    const title = `编辑失败_${Date.now()}`;
    let id = 0;
    const writes: string[] = [];
    try {
      id = (await expectSuccess<{ id: number }>(await api.post(pluginApiPath('content-notice', 'notice'), { data: { title, content: '<p>原正文</p>', status: 1 } }))).id;
      const notice = new NoticePage(adminPage);
      await notice.goto();
      adminPage.on('request', request => {
        if (['POST', 'PUT'].includes(request.method()) && /\/content-notice\/api\/v1\/notice(?:\/\d+)?$/.test(new URL(request.url()).pathname)) writes.push(request.method());
      });
      await adminPage.route(`**/x/content-notice/api/v1/notice/${id}`, route => route.fulfill({ status: 500, contentType: 'application/json', body: JSON.stringify({ code: 50, message: 'Unable to load' }) }));
      await notice.openEdit(title);
      await expect(adminPage.getByText('公告加载失败，请重试', { exact: true })).toBeVisible();
      await expect(notice.modal).not.toBeVisible();
      expect(writes).toEqual([]);
      const record = await expectSuccess<{ title: string; content: string }>(await api.get(pluginApiPath('content-notice', `notice/${id}`)));
      expect(record.title).toBe(title);
      expect(record.content).toBe('<p>原正文</p>');
      await screenshot(adminPage, 'edit-load-failed');
    } finally {
      if (id) await expectSuccess(await api.delete(pluginApiPath('content-notice', `notice?ids[]=${id}`)));
      await api.dispose();
    }
  });

});

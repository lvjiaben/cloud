import { expect, test } from '@host-tests/fixtures/auth';
import { AdminExperiencePage } from '../pages/AdminExperiencePage';
import { installSupportDeskMock } from '../support/support-desk-mock';
import { installContentManagementMock } from '../support/content-management-mock';

test.describe('TC-34 后台常用操作与布局', () => {
  test('TC-34d: 系统配置在原页面打开对应抽屉，首次配置不白屏', async ({ adminPage, mainLayout }) => {
    test.setTimeout(120_000);
    await mainLayout.switchLanguage('简体中文');
    const view = new AdminExperiencePage(adminPage);
    const pageErrors: string[] = [];
    adminPage.on('pageerror', error => pageErrors.push(error.message));
    await view.goto('/system/configuration-center');
    const root = adminPage.getByTestId('system-configuration-center');
    const entries = [
      ['业务设置', '工单设置', '/idc/support/settings', '部门与分流'],
      ['业务设置', 'IP 更换策略', '/idc/resource/ip-change-policy', '按最近多少天计算'],
      ['认证与合同', '实名认证服务', '/idc/compliance/providers/aliyun-face', '未测试'],
      ['短信与通知', '短信与通知渠道', '/idc/notifications/channels', '电子邮件'],
      ['短信与通知', '通知模板', '/idc/notifications/templates', '发送规则'],
      ['支付与货币', '支付渠道', '/idc/payment-channels', '新增支付方式'],
    ];
    for (const [tab, title, path, expected] of entries) {
      await root.getByRole('tab', { name: tab, exact: true }).click();
      await root.locator('.configuration-item').filter({ hasText: title }).getByRole('button', { name: '进入配置' }).click();
      const dialog = adminPage.getByRole('dialog');
      await expect(dialog.locator('.configuration-surface')).toHaveAttribute('data-surface-path', path);
      await expect(dialog).toContainText(expected);
      await expect(adminPage).toHaveURL(/\/system\/configuration-center$/);
      await expect(dialog).not.toContainText('plugin.idc-');
      if (path === '/idc/resource/ip-change-policy') {
        await expect(dialog).toContainText('最小单位，如分');
        await expect(dialog).not.toContainText('当前版本');
        await expect(dialog.getByRole('button', { name: '保存设置', exact: true })).toBeVisible();
        for (const field of await dialog.locator('.policy-card .field').all()) {
          const label = await field.locator('span').boundingBox();
          const input = await field.locator('input').boundingBox();
          expect(input!.y).toBeGreaterThanOrEqual(label!.y + label!.height);
          expect(input!.width).toBeGreaterThan(150);
        }
      }
      await view.screenshot(`config-${path.split('/').at(-1)}`);
      await dialog.locator('button').filter({ has: adminPage.locator('svg.lucide-x') }).first().click();
      await expect(dialog).toBeHidden();
    }
    expect(pageErrors).toEqual([]);
  });
  test('TC-34a: 工单简明筛选、展开、重置和失败恢复使用原接口', async ({ adminPage }) => {
    await installSupportDeskMock(adminPage);
    const view = new AdminExperiencePage(adminPage);
    await view.goto('/idc/support');
    const filters = view.root.locator('.filter-panel');
    await expect(filters.getByLabel('处理进度', { exact: true })).toHaveValue('');
    await expect(filters.getByLabel('优先级', { exact: true })).toHaveCount(0);
    await expect(view.root.locator('.pagination')).toHaveCount(0);
    await view.fitsViewport();
    await view.screenshot('support-initial');
    await filters.getByRole('button', { name: '更多筛选', exact: true }).click();
    await filters.getByLabel('工单号或主题', { exact: true }).fill('网络');
    await filters.getByLabel('处理进度', { exact: true }).selectOption('in_progress');
    await filters.getByLabel('优先级', { exact: true }).selectOption('high');
    const filtered = adminPage.waitForRequest(r => r.url().includes('/admin/support/tickets?') && r.url().includes('priority=high'));
    await filters.getByRole('button', { name: '搜索', exact: true }).click();
    const query = new URL((await filtered).url()).searchParams;
    expect(query.get('query')).toBe('网络'); expect(query.get('stage')).toBe('in_progress');
    await expect(view.root).toContainText('跨队列重复网络问题');
    await view.screenshot('support-filtered');
    const reset = adminPage.waitForRequest(r => r.url().includes('/admin/support/tickets?') && !new URL(r.url()).searchParams.get('query'));
    await filters.getByRole('button', { name: '重置', exact: true }).click();
    expect(new URL((await reset).url()).searchParams.get('priority')).toBeNull();
    await expect(filters.getByLabel('优先级', { exact: true })).toHaveValue('');
    let fail = true;
    await adminPage.route('**/admin/support/tickets?*', async route => {
      if (fail) { fail = false; await route.fulfill({ status: 503, json: { code: 503, message: 'unavailable', data: {} } }); }
      else await route.fallback();
    });
    await filters.getByRole('button', { name: '搜索', exact: true }).click();
    await expect(view.root.getByRole('alert')).toContainText('工单数据加载失败');
    await view.screenshot('support-error');
    await filters.getByRole('button', { name: '搜索', exact: true }).click();
    await expect(view.root.getByRole('alert')).toHaveCount(0);
    await expect(view.root).toContainText('跨队列重复网络问题');
    await adminPage.setViewportSize({ width: 768, height: 1000 });
    await view.fitsViewport();
    await view.screenshot('support-narrow');
  });
  test('TC-34b: 内容常用筛选和可见范围可展开、重置，中英文说明可读', async ({ adminPage, mainLayout }) => {
    await installContentManagementMock(adminPage);
    const view = new AdminExperiencePage(adminPage);
    await view.goto('/idc/content');
    const filters = view.root.locator('.filter-panel');
    await expect(view.root).toContainText('选择一条内容查看或编辑。');
    await expect(filters.getByLabel('可见范围', { exact: true })).toHaveCount(0);
    await filters.getByRole('button', { name: '更多筛选', exact: true }).click();
    await filters.getByLabel('可见范围', { exact: true }).selectOption('public');
    const read = adminPage.waitForRequest(r => r.url().includes('/admin/content/items?') && r.url().includes('visibility=public'));
    await filters.getByRole('button', { name: '搜索', exact: true }).click();
    expect(new URL((await read).url()).searchParams.get('visibility')).toBe('public');
    await expect(view.root).toContainText('安全下载测试内容');
    const reset = adminPage.waitForRequest(r => r.url().includes('/admin/content/items?') && !new URL(r.url()).searchParams.get('visibility'));
    await filters.getByRole('button', { name: '重置', exact: true }).click();
    await reset;
    await expect(filters.getByLabel('可见范围', { exact: true })).toHaveValue('');
    await view.screenshot('content-filtered');
    await mainLayout.switchLanguage('English');
    await expect(filters.getByRole('button', { name: 'Search', exact: true })).toBeVisible();
    await expect(view.root).toContainText('Select an item to view or edit it.');
    await filters.getByRole('button', { name: 'More filters', exact: true }).click();
    await expect(filters.getByLabel('Visible to', { exact: true })).toBeVisible();
    await view.screenshot('content-english-expanded');
    await filters.getByRole('button', { name: 'Fewer filters', exact: true }).click();
    await adminPage.setViewportSize({ width: 768, height: 1000 });
    await view.fitsViewport();
    await view.screenshot('content-english-narrow');
  });
  test('TC-34c: 财务账单优先显示，低频工具按需加载且不执行写操作', async ({ adminPage }) => {
    const view = new AdminExperiencePage(adminPage);
    const writes: string[] = [];
    adminPage.on('request', r => { if (r.url().includes('/admin/billing/') && r.method() !== 'GET') writes.push(r.url()); });
    await view.goto('/idc/finance');
    const tools = view.root.locator('.finance-tools');
    await expect(view.root.getByTestId('financial-operations-panel')).toHaveCount(0);
    await expect(view.root.locator('.finance-tabs .vxe-grid').first()).toBeVisible();
    await tools.getByRole('button').click();
    await expect(view.root.getByTestId('financial-operations-panel')).toBeVisible();
    await expect(view.root).toContainText('内部备注');
    expect(writes).toEqual([]);
    await view.screenshot('finance-tools');
    await tools.getByRole('button').first().click();
    await expect(view.root.getByTestId('financial-operations-panel')).not.toBeVisible();
    await expect(view.root.locator('.finance-tabs .vxe-grid').first()).toBeVisible();
    await view.fitsViewport();
    await view.screenshot('finance-daily');
  });
  test('TC-34e: 常用后台表格没有宽度收缩、异常增高或隐藏分页', async ({ adminPage, mainLayout }) => {
    test.setTimeout(120_000);
    await mainLayout.switchLanguage('简体中文');
    const view = new AdminExperiencePage(adminPage);
    for (const path of ['/idc/customers', '/idc/catalog', '/idc/catalog/automation-interfaces', '/idc/member-navigation', '/idc/product-fulfillment/tasks', '/idc/channel', '/idc/payment-channels', '/idc/sales-channel']) {
      await view.goto(path);
      // Categories are a bounded hierarchy, with paging intentionally disabled.
      await view.expectVisibleGridsFit(path !== '/idc/catalog/categories');
      await view.screenshot(`aligned-${path.split('/').at(-1)}`);
    }
    await mainLayout.switchLanguage('English');
    await view.goto('/idc/channel');
    await expect(view.root).toContainText('Customer access');
    await adminPage.setViewportSize({ width: 768, height: 1000 });
    await view.expectVisibleGridsFit();
    await view.screenshot('openapi-english-narrow-aligned');
  });

  test('TC-34f: 常用业务列表去掉重复介绍，表格伸展且分页可达', async ({ adminPage }) => {
    test.setTimeout(180_000);
    await adminPage.setViewportSize({ width: 1440, height: 1000 });
    const view = new AdminExperiencePage(adminPage);
    const errors: string[] = [];
    adminPage.on('pageerror', error => errors.push(error.message));
    for (const path of ['/idc/orders', '/idc/finance', '/idc/payment-channels', '/idc/compliance/enterprise-verifications', '/idc/compliance/tax-invoices', '/idc/resource/overview', '/idc/resource/server-accessories']) {
      await view.goto(path);
      await expect(view.root.locator('.page-heading h1,.page-heading h2,.enterprise-header,.finance-heading h2,.overview-heading h2')).toHaveCount(0);
      await view.fitsViewport();
      await view.expectVisibleGridsFit();
      await expect.poll(() => view.root.evaluate(root => {
        const grids = [...root.querySelectorAll('.vxe-grid')].filter(grid => grid.getBoundingClientRect().width > 0);
        return Math.max(...grids.map(grid => grid.getBoundingClientRect().bottom)) >= innerHeight - 90;
      })).toBe(true);
      await view.screenshot(`standard-${path.split('/').at(-1)}`);
    }
    expect(errors).toEqual([]);
  });
});

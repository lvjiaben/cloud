// Verify the delivered backend and real menu, without source-route injection or lifecycle writes.
import { test, expect } from '../support/test';
import { LoginPage } from '../../../../../../hack/tests/pages/LoginPage';
import { config } from '../../../../../../hack/tests/fixtures/config';
import { PluginManagerPage } from '../pages/PluginManagerPage';

test.describe('TC-4 插件管理运行时装配', () => {
  test('TC-4a: 实际后端包含内置管理插件并替换旧入口，刷新后仍读取真实目录', async ({ page }, info) => {
    const login = new LoginPage(page);
    await login.goto();
    const signed = page.waitForResponse(response =>
      response.request().method() === 'POST' && new URL(response.url()).pathname.endsWith('/auth/login'));
    await login.loginAndWaitForRedirect(config.adminUser, config.adminPass);
    const accessToken = (await (await signed).json()).data.accessToken;
    const headers = { Authorization: `Bearer ${accessToken}` };
    const categoriesResponse = await page.request.get(`${config.backendBaseURL}/x/plugin-manager/api/v1/categories`, { headers });
    expect(categoriesResponse.status(), '实际交付的后端必须编译并注册plugin-manager，不能只保留数据库启用记录').toBe(200);
    const categories = await categoriesResponse.json();
    expect(categories.code).toBe(0);
    expect(categories.data.list.length).toBeGreaterThanOrEqual(7);
    const menusResponse = await page.request.get(`${config.backendBaseURL}/api/v1/menus/all`, { headers });
    expect(menusResponse.ok()).toBe(true);
    const menuPayload = await menusResponse.json();
    expect(menuPayload.code).toBe(0);
    const flatten = (items: any[]): any[] => items.flatMap(item => [item, ...flatten(item.children || [])]);
    const menus = flatten(menuPayload.data.list);
    const current = menus.filter(item => item.path === '/extension/plugin-manager');
    expect(current).toHaveLength(1);
    expect(menus.some(item => item.path === '/system/plugin')).toBe(false);
    expect(menus.find(item => item.id === current[0].parentId)?.path).toBe('/setting');
    const manager = new PluginManagerPage(page);
    await manager.openFromSettings();
    await expect(manager.root).toBeVisible();
    await expect(page).toHaveURL(/\/extension\/plugin-manager$/);
    for (const category of categories.data.list)
      await expect(manager.root.getByRole('tab', { name: category.name, exact: true })).toBeVisible();
    const catalogResponse = page.waitForResponse(response => {
      const url = new URL(response.url());
      return url.pathname === '/x/plugin-manager/api/v1/plugins' && url.searchParams.get('category') === 'server';
    });
    await manager.tab('服务器');
    const catalog = await (await catalogResponse).json();
    expect(catalog.code).toBe(0);
    expect(catalog.data.total).toBeGreaterThan(0);
    for (const row of catalog.data.list) {
      expect(row.type).toBe('dynamic');
      expect(row.distribution).toBe('managed');
      await expect(manager.identity(row.id)).toContainText(row.name);
    }
    await page.screenshot({ path: info.outputPath(`${Date.now()}-plugin-manager-restored.png`), animations: 'disabled' });
    await page.reload();
    await expect(manager.root).toBeVisible();
    await expect(manager.navigationEntry).toBeVisible();
    await manager.openGuide();
    await expect(manager.guide).toBeVisible();
    await expect(manager.guide).not.toContainText('pages.system.plugin.guide.');
    await page.screenshot({ path: info.outputPath(`${Date.now()}-plugin-manager-guide.png`), animations: 'disabled' });
  });
});

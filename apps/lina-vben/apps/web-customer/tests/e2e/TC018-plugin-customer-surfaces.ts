import { expect, test } from '../fixtures/customer';
import { PluginSurfacePage } from '../pages/PluginSurfacePage';
import { installCustomerPluginSurfaceMock } from '../support/customer-plugin-surface-api-mock';

test.describe('TC018 客户中心插件页面受治理装配', () => {
  test('TC018a: 已启用插件只能在宿主发布的产品二级菜单中出现并打开宿主管理路由', async ({
    page,
  }) => {
    const requests = await installCustomerPluginSurfaceMock(page);
    const pluginSurface = new PluginSurfacePage(page);

    await pluginSurface.login();
    await pluginSurface.openProductsGroup();
    await expect(pluginSurface.surfaceMenu).toBeVisible();
    await pluginSurface.openSurface();
    await expect(
      pluginSurface.pluginFrame().getByTestId('phone-orders-plugin-page'),
    ).toContainText('手机订单内容');
    await expect(
      pluginSurface.pluginFrame().getByTestId('sandbox-result'),
    ).toContainText('已隔离');
    await expect(
      pluginSurface.pluginFrame().getByTestId('bridge-context'),
    ).toContainText(/zh-CN\|(light|dark)/u);
    await expect(
      pluginSurface.pluginFrame().getByTestId('bridge-api'),
    ).toContainText('current-customer|shipped');
    expect(requests.getBridgeRequestHeaders()).toMatchObject({
      'x-lina-plugin-page-grant': 'server-signed-opaque-grant',
    });
    expect(requests.getBridgeRequestHeaders()).not.toHaveProperty(
      'x-lina-plugin-page-capability',
    );
    expect(requests.getGrantRequestBody()).toEqual({
      capability: 'phone-orders.query-own',
      host: 'customer',
      method: 'GET',
      path: 'phone-orders',
      pluginId: 'phone-provider',
      surfaceKey: 'phone-orders',
      surfaceVersion: 'v1',
    });
    await expect(
      pluginSurface.pluginFrame().getByTestId('bridge-denied'),
    ).toContainText('bridge_request_denied');
    await expect(page).toHaveURL(/pluginPath=orders\/active/u);
    await expect(page.getByTestId('plugin-sandbox-skeleton')).toHaveCount(0);
    await expect(page.getByTestId('plugin-sandbox-frame')).toHaveCSS(
      'height',
      '777px',
    );
    await expect(page.getByTestId('plugin-sandbox-container')).toHaveAttribute(
      'data-plugin-title',
      '手机订单详情',
    );
    await expect(page.getByText('手机订单详情', { exact: true })).toBeVisible();
    await expect(
      page.locator(
        'link[data-plugin-sandbox-prewarm="/x-assets/phone-provider/v1.0.0/customer/phone-orders/index.html"]',
      ),
    ).toHaveCount(1);
    const sandbox = await page
      .getByTestId('plugin-sandbox-frame')
      .getAttribute('sandbox');
    expect(sandbox).not.toContain('allow-same-origin');
    const firstEntryRequests = requests.getAssetRequests();
    expect(firstEntryRequests).toBeGreaterThanOrEqual(1);
    await page.getByText('客户概览', { exact: true }).first().click();
    await page.waitForURL(/\/account$/u);
    await pluginSurface.openProductsGroup();
    await pluginSurface.openSurface();
    await expect(
      pluginSurface.pluginFrame().getByTestId('bridge-api'),
    ).toContainText('current-customer|shipped');
    expect(requests.getAssetRequests()).toBe(firstEntryRequests);
    await pluginSurface.screenshot('customer-plugin-surface-zh');
  });

  test('TC018b: 已停用插件不会留下客户菜单入口', async ({ page }) => {
    await installCustomerPluginSurfaceMock(page, { enabled: false });
    const pluginSurface = new PluginSurfacePage(page);

    await pluginSurface.login();
    await expect(pluginSurface.surfaceMenu).toHaveCount(0);
  });

  test('TC018c: 只停止新销售时老客户菜单和已有服务页面继续可用', async ({
    page,
  }) => {
    await installCustomerPluginSurfaceMock(page, {
      newBusinessState: 'stopped',
    });
    const pluginSurface = new PluginSurfacePage(page);

    await pluginSurface.login();
    await pluginSurface.openProductsGroup();
    await expect(pluginSurface.surfaceMenu).toBeVisible();
    await pluginSurface.openSurface();
    await expect(
      pluginSurface.pluginFrame().getByTestId('phone-orders-plugin-page'),
    ).toContainText('手机订单内容');
  });
});

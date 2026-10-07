import { expect, test } from '../fixtures/customer';
import { PluginSurfacePage } from '../pages/PluginSurfacePage';
import { installCustomerPluginSurfaceMock } from '../support/customer-plugin-surface-api-mock';

test.describe('TC019 动态插件页面隔离桥与连续体验', () => {
  test('TC019a: 慢资源期间显示同主题骨架并实时同步明暗主题', async ({
    page,
  }) => {
    await installCustomerPluginSurfaceMock(page, { slowAssetMs: 900 });
    const pluginSurface = new PluginSurfacePage(page);

    await pluginSurface.login();
    await pluginSurface.openProductsGroup();
    await pluginSurface.surfaceMenu.click();
    await expect(page.getByTestId('plugin-sandbox-skeleton')).toBeVisible();
    await expect(
      pluginSurface.pluginFrame().getByTestId('phone-orders-plugin-page'),
    ).toContainText('手机订单内容');
    await expect(page.getByTestId('plugin-sandbox-skeleton')).toHaveCount(0);

    const context = pluginSurface.pluginFrame().getByTestId('bridge-context');
    const initialContext = (await context.textContent()) || '';
    const targetTheme = initialContext.endsWith('|dark') ? 'light' : 'dark';
    await page.getByRole('button', { name: targetTheme }).click();
    await expect(context).toContainText(`zh-CN|${targetTheme}`);
    await expect(pluginSurface.pluginFrame().locator('html')).toHaveAttribute(
      'data-theme',
      targetTheme,
    );
    await page.getByTestId('language-toggle-trigger').click();
    await page.getByText('English', { exact: true }).click();
    await expect(context).toContainText(`en-US|${targetTheme}`);
    await expect(
      pluginSurface.pluginFrame().getByTestId('phone-orders-plugin-page'),
    ).toContainText('Phone order content');
  });

  test('TC019b: 英文窄屏、刷新和再次进入都保持宿主页面体验', async ({
    page,
  }) => {
    await installCustomerPluginSurfaceMock(page, { locale: 'en-US' });
    const pluginSurface = new PluginSurfacePage(
      page,
      'Phone orders',
      'Products & Services',
    );

    await pluginSurface.login();
    await pluginSurface.openProductsGroup();
    await pluginSurface.openSurface();
    await expect(
      pluginSurface.pluginFrame().getByTestId('phone-orders-plugin-page'),
    ).toContainText('Phone order content');
    await expect(
      pluginSurface.pluginFrame().getByTestId('bridge-context'),
    ).toContainText(/en-US\|(light|dark)/u);
    await expect(
      page.getByText('Phone order details', { exact: true }),
    ).toBeVisible();

    await page.setViewportSize({ height: 844, width: 390 });
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth - window.innerWidth,
      ),
    ).toBeLessThanOrEqual(1);
    await expect(page.getByTestId('plugin-sandbox-frame')).toBeVisible();

    await page.reload();
    await expect(
      pluginSurface.pluginFrame().getByTestId('bridge-api'),
    ).toContainText('current-customer|shipped');
    await expect(
      page.locator('iframe[data-testid="plugin-sandbox-frame"]'),
    ).toHaveCount(1);
  });

  test('TC019c: 插件报告失败时使用宿主统一错误页且不展示技术错误', async ({
    page,
  }) => {
    await installCustomerPluginSurfaceMock(page, {
      pluginReportsError: true,
    });
    const pluginSurface = new PluginSurfacePage(page);

    await pluginSurface.login();
    await pluginSurface.openProductsGroup();
    await pluginSurface.openSurface();
    const error = page.getByTestId('plugin-sandbox-error');
    await expect(error).toContainText('哎呀！出错了');
    await expect(error).toContainText('抱歉，服务器遇到错误。');
    await expect(error.getByRole('button', { name: '刷新' })).toBeVisible();
    await expect(error).not.toContainText('bridge_');
  });
});

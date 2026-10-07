import { expect, test } from '../fixtures/customer';
import { CustomerInnerPages } from '../pages/CustomerInnerPages';
import { mockCustomerAPIs } from '../support/customer-api-mock';
import {
  commerceOrderNo,
  installCustomerCommerceMock,
} from '../support/customer-commerce-api-mock';
import {
  balanceInvoiceNo,
  installCustomerFinanceMock,
} from '../support/customer-finance-api-mock';
import {
  installCustomerServerMock,
  serverServiceNo,
} from '../support/customer-server-api-mock';

test.describe('TC-30 客户内页视觉统一且外层布局不变', () => {
  test('TC-30a: 桌面支持列表、详情与通知共用内页规则，分页可见', async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await mockCustomerAPIs(page);
    const views = new CustomerInnerPages(page);
    await views.login();
    await views.inspect('/account/support', 'inner-support-desktop');
    await views.inspectTable();
    await views.inspect(
      '/account/support/TKT-2026-0001',
      'inner-support-detail-desktop',
    );
    await views.inspect(
      '/account/notifications',
      'inner-notifications-desktop',
    );
  });

  test('TC-30b: 英文手机财务金额、明细与导航不撑出页面', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await installCustomerFinanceMock(page, { locale: 'en-US' });
    const views = new CustomerInnerPages(page);
    await views.login();
    for (const path of [
      '/account/billing',
      '/account/billing/invoices',
      `/account/billing/invoices/${balanceInvoiceNo}`,
      '/account/billing/transactions',
      '/account/billing/renewals',
    ]) {
      await views.inspect(
        path,
        `inner-finance-mobile-${path.split('/').at(-1)}`,
      );
      if (path === '/account/billing') {
        await expect(page.locator('.customer-inner-heading')).toHaveText(
          'Financial overview',
        );
      }
    }
  });

  test('TC-30c: 中文桌面购物车、订单和详情沿用同一内容对齐', async ({
    page,
  }) => {
    await page.setViewportSize({ width: 1440, height: 1000 });
    await installCustomerCommerceMock(page);
    const views = new CustomerInnerPages(page);
    await views.login();
    for (const path of [
      '/account/cart',
      '/account/orders',
      `/account/orders/${commerceOrderNo}`,
    ]) {
      await views.inspect(
        path,
        `inner-commerce-desktop-${path.split('/').at(-1)}`,
      );
    }
  });

  test('TC-30d: 英文手机服务器详情保留全部操作且不溢出', async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await installCustomerServerMock(page, { locale: 'en-US' });
    const views = new CustomerInnerPages(page);
    await views.login();
    await views.inspect(
      `/account/servers/${serverServiceNo}`,
      'inner-server-detail-mobile',
    );
    await expect(page.locator('.customer-inner-heading')).toHaveText(
      'Server details',
    );
    await expect(page.locator('.ant-card-head-title').first()).toHaveCSS(
      'overflow',
      'visible',
    );
  });
});

import { CommercePage } from '../pages/CommercePage';
import {
  commerceOrderNo,
  installCustomerCommerceMock,
} from '../support/customer-commerce-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';
import { expect, test } from '../fixtures/customer';

test.describe('TC010 客户购物车与订单', () => {
  test('TC010a: 重新核价后确认差异并只创建一个订单', async ({ page }) => {
    const state = await installCustomerCommerceMock(page);
    const commerce = new CommercePage(page);
    await commerce.login();
    await commerce.open();

    await expect(commerce.root).toContainText('入门云服务器');
    await page.getByTestId('commerce-primary-action').click();
    await expect(page.getByTestId('commerce-final-quote')).toContainText(
      'CNY 108.12',
    );
    await page.getByRole('button', { name: '再次核价' }).click();
    await expect(commerce.root).toContainText('价格已经变化');
    await expect(page.getByTestId('commerce-final-quote')).toContainText(
      'CNY 109.12',
    );
    await page.getByTestId('commerce-primary-action').click();

    await expect(page.getByTestId('commerce-created-order')).toContainText(
      commerceOrderNo,
    );
    expect(state.checkoutKeys).toHaveLength(1);
    expect(state.checkoutKeys[0]).toMatch(/^customer-/u);
    await expect(commerce.root).toContainText('等待实名认证');
    await captureCustomerScreenshot(page, 'customer-commerce-checkout-zh-cn');
  });

  test('TC010b: 实名返回只重新检查原订单且不会伪造付款成功', async ({
    page,
  }) => {
    const state = await installCustomerCommerceMock(page);
    const commerce = new CommercePage(page);
    await commerce.login();
    await commerce.open(`/account/orders/${commerceOrderNo}`);

    await expect(commerce.root).toContainText('等待实名认证');
    await page.getByRole('button', { name: '我已完成，重新检查' }).click();
    await expect(commerce.root).toContainText('可以继续选择支付方式');
    await expect(commerce.root).not.toContainText('已付款');
    expect(state.paymentReady).toBe(true);
    expect(state.checkoutKeys).toHaveLength(0);
  });

  test('TC010c: 取消订单先二次确认并复用稳定请求号', async ({ page }) => {
    const state = await installCustomerCommerceMock(page);
    const commerce = new CommercePage(page);
    await commerce.login();
    await commerce.open(`/account/orders/${commerceOrderNo}`);

    await page.getByRole('button', { name: '取消未付款订单' }).click();
    await expect(page.getByRole('dialog')).toContainText(
      '重新核对付款、资金和资源状态',
    );
    await page.getByRole('button', { name: '确认取消' }).click();
    await expect(commerce.root).toContainText('已取消');
    expect(state.cancellationKeys).toHaveLength(1);
    expect(state.cancellationKeys[0]).toMatch(/^customer-/u);
  });

  test('TC010d: 英文手机订单列表无横向溢出', async ({ page }) => {
    await page.setViewportSize({ height: 844, width: 390 });
    await installCustomerCommerceMock(page, { locale: 'en-US' });
    const commerce = new CommercePage(page);
    await commerce.login();
    await commerce.open('/account/orders');

    await expect(commerce.root).toContainText('My orders');
    await expect(commerce.root).toContainText('Awaiting payment');
    expect(await commerce.hasHorizontalOverflow()).toBe(false);
    await captureCustomerScreenshot(page, 'customer-commerce-orders-en-mobile');
  });
});

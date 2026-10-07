import { CommercePage } from '../pages/CommercePage';
import {
  commerceOrderNo,
  installCustomerCommerceMock,
} from '../support/customer-commerce-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';
import { expect, test } from '../fixtures/customer';

test.describe('TC021 客户订单通用交付状态', () => {
  test('TC021a: 订单详情只显示每个商品的六态安全交付', async ({
    page,
  }) => {
    await installCustomerCommerceMock(page);
    const commerce = new CommercePage(page);
    await commerce.login();
    await commerce.open(`/account/orders/${commerceOrderNo}`);

    const fulfillment = page.getByTestId('commerce-deliveries');
    await expect(fulfillment).toContainText('处理中');
    await expect(fulfillment).toContainText('2 件商品');
    await expect(fulfillment).toContainText('包含持续服务');
    await expect(fulfillment).toContainText('一次性交付');
    await expect(fulfillment).toContainText('处理未完成');
    await expect(fulfillment).not.toContainText(
      /SVC_0001|FUL_|intent|task|plugin|provider|supplier|evidence|payload|secret/iu,
    );
    await expect(fulfillment).not.toContainText(/unitSequence|granularityCode/iu);
    const services = page.getByTestId('commerce-service-entitlements');
    await expect(services).toContainText('入门云服务器');
    await expect(services).toContainText('服务中');
    await expect(services).not.toContainText(
      /plugin|interface|contract|digest|SVC_|ENT_/iu,
    );
    await fulfillment.scrollIntoViewIfNeeded();
    await captureCustomerScreenshot(page, 'customer-order-safe-fulfillment');
  });
});

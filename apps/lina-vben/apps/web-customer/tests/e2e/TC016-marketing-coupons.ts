import { MarketingCouponsPage } from '../pages/MarketingCouponsPage';
import { expect, test } from '../fixtures/customer';
import { installCustomerMarketingCouponMock } from '../support/customer-marketing-coupon-api-mock';

test.describe('TC016 客户优惠码兑换', () => {
  test('TC016a: 错误码统一提示且小写优惠码按大写提交并成功领取', async ({
    page,
  }) => {
    const state = await installCustomerMarketingCouponMock(page);
    const coupons = new MarketingCouponsPage(page);
    await coupons.login();
    await coupons.open();

    await expect(coupons.root).toContainText('兑换优惠码');
    await expect(coupons.root).toContainText('还没有可用优惠');
    await coupons.screenshot('customer-marketing-coupons-empty-zh');

    await coupons.redeem('wrong2026');
    await expect(coupons.root).toContainText('优惠码无效');
    await expect(page.getByText('The promotion code is invalid')).toHaveCount(0);
    await coupons.screenshot('customer-marketing-coupon-invalid-zh');

    await coupons.redeem('save2026ab');
    await expect(coupons.root).toContainText('优惠已经加入你的账户');
    await expect(coupons.root).toContainText('夏日优惠');
    await expect(coupons.root).toContainText('可使用');
    expect(state.claimBodies).toHaveLength(2);
    expect(state.claimBodies[0]).toMatchObject({ code: 'WRONG2026' });
    expect(state.claimBodies[1]).toMatchObject({ code: 'SAVE2026AB' });
    expect(String(state.claimBodies[1]?.deviceId)).not.toHaveLength(0);
    await coupons.screenshot('customer-marketing-coupon-claimed-zh');
  });
});

test.describe('TC016 customer promotions in English', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC016b: English mobile page shows translated promotion details', async ({
    page,
  }) => {
    await installCustomerMarketingCouponMock(page, { locale: 'en-US' });
    const coupons = new MarketingCouponsPage(page);
    await coupons.login();
    await coupons.open();
    await coupons.redeem('save2026ab');

    await expect(coupons.root).toContainText('Redeem a promotion code');
    await expect(coupons.root).toContainText(
      'The promotion has been added to your account',
    );
    await expect(coupons.root).toContainText('Summer discount');
    await expect(coupons.root).toContainText('Available');
    await expect(coupons.root).not.toContainText('marketingCoupons.');
    await coupons.screenshot('customer-marketing-coupon-claimed-en-mobile');
  });
});

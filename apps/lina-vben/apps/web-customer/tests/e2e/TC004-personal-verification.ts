import { expect, test } from '@playwright/test';

import { PersonalVerificationPage } from '../pages/PersonalVerificationPage';
import {
  identityProviderURL,
  installPersonalVerificationMock,
} from '../support/personal-verification-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

const rawName = '测试用户';
const rawIdentityNumber = '11010519491231002X';

test.describe('TC004 客户个人实名与付款恢复', () => {
  test('TC004a: 单独同意、权威回查通过后才恢复原付款准备', async ({ page }) => {
    const state = await installPersonalVerificationMock(page);
    const identity = new PersonalVerificationPage(page);
    await identity.login();
    await identity.open(
      '/account/identity?return_to=%2Faccount%2Forders%2FORD_20260811_0001&payment_context=PAYCTX_20260811_0001',
    );

    await expect(identity.status).toHaveText('未认证');
    await expect(identity.root).toContainText('敏感个人信息单独同意');
    await captureCustomerScreenshot(
      page,
      'customer-personal-verification-consent-zh-cn',
    );

    await identity.start(rawName, rawIdentityNumber);
    await page.waitForURL(identityProviderURL);
    expect(state.consentRequests).toHaveLength(1);
    expect(state.consentRequests[0]).toMatchObject({
      accepted: true,
      locale: 'zh-CN',
      policyVersion: 'personal-spi-v1',
    });
    expect(state.startRequests).toEqual([
      {
        certName: rawName,
        certNo: rawIdentityNumber,
        certType: 'IDENTITY_CARD',
        consentNo: 'consent_20260812_0001',
      },
    ]);
    expect(state.startRequests[0]).not.toHaveProperty('customerNo');
    expect(state.startRequests[0]).not.toHaveProperty('returnUrl');

    await identity.open('/account/identity?verification_return=1');
    await expect(identity.status).toHaveText('已认证');
    await expect(identity.root).toContainText('原付款准备已重新核验');
    await expect(identity.root).toContainText('110***********002X');
    await expect(identity.root).not.toContainText(rawIdentityNumber);
    await expect(identity.root).not.toContainText(rawName);
    await expect(
      identity.root.getByRole('link', { name: '返回原订单继续付款' }),
    ).toHaveAttribute('href', '/account/orders/ORD_20260811_0001');
    expect(state.refreshRequests).toEqual([
      { attemptNo: 'pva_20260812_0001', body: null },
    ]);
    expect(state.paymentResumeRequests).toEqual(['PAYCTX_20260811_0001']);

    const stored = await page.evaluate(() => [
      ...Object.values(localStorage),
      ...Object.values(sessionStorage),
    ]);
    expect(stored.join('\n')).not.toContain(rawIdentityNumber);
    expect(stored.join('\n')).not.toContain(rawName);
    await captureCustomerScreenshot(
      page,
      'customer-personal-verification-payment-ready-zh-cn',
    );
  });

  test('TC004b: 伪造浏览器通过参数仍保持认证中', async ({ page }) => {
    const state = await installPersonalVerificationMock(page, {
      authoritativeStatus: 'verifying',
      initialStatus: 'verifying',
    });
    const identity = new PersonalVerificationPage(page);
    await identity.login();
    await identity.open(
      '/account/identity?verification_return=1&status=verified&passed=true',
    );

    await expect(identity.status).toHaveText('认证中');
    await expect(identity.root).toContainText('阿里云尚未确认通过');
    await expect(identity.root).not.toContainText('实名认证已通过');
    expect(state.refreshRequests).toEqual([
      { attemptNo: 'pva_20260812_0001', body: null },
    ]);
    expect(state.paymentResumeRequests).toHaveLength(0);
    await captureCustomerScreenshot(
      page,
      'customer-personal-verification-authoritative-pending',
    );
  });

  test('TC004c: 不安全供应商跳转失败关闭且身份明文不进存储', async ({
    page,
  }) => {
    await installPersonalVerificationMock(page, {
      unsafeContinuation: true,
    });
    const identity = new PersonalVerificationPage(page);
    await identity.login();
    await identity.open();
    await identity.start(rawName, rawIdentityNumber);

    await expect(page).toHaveURL(/\/account\/identity/u);
    await expect(identity.root).toContainText('实名认证服务暂时不可用');
    const stored = await page.evaluate(() => [
      ...Object.values(localStorage),
      ...Object.values(sessionStorage),
    ]);
    expect(stored.join('\n')).not.toContain(rawIdentityNumber);
    expect(stored.join('\n')).not.toContain(rawName);
    await captureCustomerScreenshot(
      page,
      'customer-personal-verification-unsafe-continuation',
    );
  });
});

test.describe('TC004 客户个人实名英文移动端', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC004d: 英文移动端完整展示单独同意且无横向溢出', async ({ page }) => {
    await installPersonalVerificationMock(page, { locale: 'en-US' });
    const identity = new PersonalVerificationPage(page);
    await identity.login();
    await identity.open();

    await expect(identity.root).toContainText(
      'Verify who you are before the first payment',
    );
    await expect(identity.root).toContainText(
      'Separate consent for sensitive personal information',
    );
    await expect(identity.root).toContainText(
      'End customers neither configure nor pay',
    );
    await expect(identity.root).not.toContainText('identity.form.title');
    expect(await identity.hasHorizontalOverflow()).toBe(false);
    await captureCustomerScreenshot(
      page,
      'customer-personal-verification-mobile-en-us',
    );
    await identity.consent.scrollIntoViewIfNeeded();
    await captureCustomerScreenshot(
      page,
      'customer-personal-verification-mobile-consent-en-us',
    );
  });
});

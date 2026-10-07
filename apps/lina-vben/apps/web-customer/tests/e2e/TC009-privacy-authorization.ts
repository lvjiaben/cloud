import { PrivacyPage } from '../pages/PrivacyPage';
import { installCustomerPrivacyMock } from '../support/customer-privacy-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';
import { expect, test } from '../fixtures/customer';

test.describe('TC009 隐私与授权', () => {
  test('TC009a: 客户只看到本人告知、授权和申请进度', async ({ page }) => {
    await installCustomerPrivacyMock(page);
    const privacy = new PrivacyPage(page);
    await privacy.login();
    await privacy.open();

    await expect(privacy.root).toContainText('我们如何处理资料');
    await expect(privacy.root).toContainText('个人实名认证');
    await expect(privacy.root).toContainText('365 天');
    await expect(privacy.root).toContainText('暂需保留');
    await expect(privacy.root).toContainText('税务证据仍需保留');
    await expect(privacy.root).not.toContainText('customerNo');
    await captureCustomerScreenshot(page, 'customer-privacy-overview-zh-cn');
  });

  test('TC009b: 撤回只提交未来授权且保留历史证据说明', async ({ page }) => {
    const state = await installCustomerPrivacyMock(page);
    const privacy = new PrivacyPage(page);
    await privacy.login();
    await privacy.open();

    await page.getByRole('button', { name: '撤回未来授权' }).click();
    await expect(page.getByRole('dialog').last()).toContainText(
      '已经形成且依法需要保留的证据不会立即删除',
    );
    await privacy.confirm();
    expect(state.withdrawals).toHaveLength(1);
    expect(state.withdrawals[0]).toMatchObject({
      consentTypeCode: 'sensitive_personal_information',
    });
    expect(Object.keys(state.withdrawals[0] ?? {}).sort()).toEqual([
      'consentTypeCode',
      'idempotencyKey',
    ]);
    await expect(privacy.root).toContainText('已撤回');
    await expect(
      page.getByRole('button', { name: '撤回未来授权' }),
    ).toHaveCount(0);
  });

  test('TC009c: 导出和删除申请都需要明确确认并显示最新进度', async ({
    page,
  }) => {
    const state = await installCustomerPrivacyMock(page);
    const privacy = new PrivacyPage(page);
    await privacy.login();
    await privacy.open();

    await page.getByRole('button', { name: '申请导出资料' }).click();
    await expect(page.getByRole('dialog').last()).toContainText(
      '仅属于你的资料',
    );
    await privacy.confirm();
    await page.getByRole('button', { name: '申请删除资料' }).click();
    await expect(page.getByRole('dialog').last()).toContainText(
      '合同、税务、争议、审计和法律保留要求',
    );
    await privacy.confirm();

    expect(state.requests.map((item) => item.requestTypeCode)).toEqual([
      'export',
      'delete',
    ]);
    await expect(privacy.root).toContainText('已提交');
  });
});

test.describe('TC009 privacy authorization on English mobile', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC009d: English mobile page is translated and has no horizontal overflow', async ({
    page,
  }) => {
    await installCustomerPrivacyMock(page, { locale: 'en-US' });
    const privacy = new PrivacyPage(page);
    await privacy.login();
    await privacy.open();

    await expect(privacy.root).toContainText('Privacy & Authorization');
    await expect(privacy.root).toContainText('Sensitive personal information');
    await expect(privacy.root).toContainText('Tax evidence must be retained');
    await expect(privacy.root).not.toContainText('privacy.');
    expect(await privacy.hasHorizontalOverflow()).toBe(false);
    await captureCustomerScreenshot(page, 'customer-privacy-mobile-en-us');
  });
});

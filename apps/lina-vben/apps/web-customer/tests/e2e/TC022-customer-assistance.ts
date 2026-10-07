import { expect, test } from '../fixtures/customer';

import { CustomerAssistancePage } from '../pages/CustomerAssistancePage';
import { installCustomerAssistanceMock } from '../support/customer-assistance-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

test.describe('TC022 客户只读协助', () => {
  test('TC022a: 口令离开地址栏且只展示安全概况和禁用能力', async ({
    page,
  }) => {
    const state = await installCustomerAssistanceMock(page);
    const assistance = new CustomerAssistancePage(page);
    const token = 'A'.repeat(43);
    await assistance.open(token);

    await expect(page).toHaveURL(/\/account\/assistance$/u);
    await expect(assistance.root).toContainText('当前是只读协助页面');
    await expect(assistance.root).toContainText('上海云舟科技');
    await expect(assistance.root).not.toContainText(
      'CUS_0123456789abcdef0123456789abcdef',
    );
    await expect(assistance.root).toContainText(
      '协助记录编号：AST_0123456789abcdef0123456789abcdef',
    );
    await expect(assistance.root).toContainText('付款');
    await expect(assistance.root).toContainText('服务器控制台');
    await expect(assistance.root).not.toContainText('登录密码');
    await expect(assistance.root).not.toContainText('身份证号码');
    await expect(assistance.countdown).toHaveText(/^14:\d{2}$|^15:00$/u);
    expect(state.requests).toHaveLength(1);
    expect(state.requests[0]?.body).toEqual({ accessToken: token });
    expect(state.requests[0]?.url).not.toContain(token);
    await captureCustomerScreenshot(page, 'customer-assistance-readonly-zh-cn');
  });

  test('TC022b: 无效或已结束口令只显示失效说明', async ({ page }) => {
    await installCustomerAssistanceMock(page, { unavailable: true });
    const assistance = new CustomerAssistancePage(page);
    await assistance.open();

    await expect(assistance.root).toContainText('本次协助已经失效');
    await expect(assistance.root).toContainText('请关闭本页');
    await expect(assistance.root).not.toContainText('客户编号');
    await expect(assistance.countdown).toHaveCount(0);
  });
});

test.describe('TC022 customer assistance on English mobile', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC022c: 英文窄屏完整翻译且没有横向溢出', async ({ page }) => {
    await installCustomerAssistanceMock(page);
    const assistance = new CustomerAssistancePage(page);
    await assistance.open();

    await expect(assistance.root).toContainText(
      'This is a read-only assistance page',
    );
    await expect(assistance.root).toContainText('Actions explicitly unavailable');
    await expect(assistance.root).not.toContainText('assistance.');
    expect(await assistance.hasHorizontalOverflow()).toBe(false);
    expect(await assistance.hasClippedContent()).toBe(false);
    await captureCustomerScreenshot(page, 'customer-assistance-mobile-en-us');
  });
});

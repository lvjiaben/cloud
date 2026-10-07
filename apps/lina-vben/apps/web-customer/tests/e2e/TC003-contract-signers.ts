import { expect, test } from '@playwright/test';

import { ContractSignersPage } from '../pages/ContractSignersPage';
import { installContractSignerMock } from '../support/contract-signer-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

test.describe('TC003 企业合同签署经办人', () => {
  test.beforeEach(async ({ page }) => {
    await installContractSignerMock(page);
    await page.goto('/auth/login');
    await page.getByPlaceholder('请输入用户名').fill('customer');
    await page.getByPlaceholder('请输入密码').fill('customer-password');
    await page.getByRole('button', { name: 'login' }).click();
    await page.waitForURL(/\/(?:account|profile)(?:$|\?)/);
  });

  test('TC003a: 企业客户单独添加经办人，完成本人实名和统一短信验证后才成为候选', async ({
    page,
  }) => {
    const signers = new ContractSignersPage(page);
    await signers.open();
    await expect(
      page.getByText('维护可以代表企业签署合同的人员'),
    ).toBeVisible();
    await signers.addSigner();
    await expect(signers.firstSigner).toContainText('验证未完成');

    await signers.startAndCheckIdentity();
    await expect(signers.firstSigner).toContainText('实名已通过');
    await expect(signers.firstSigner).toContainText('验证未完成');

    await signers.verifyPhone();
    await expect(signers.firstSigner).toContainText('手机号已验证');
    await expect(signers.firstSigner).toContainText('可用于新合同');
    await expect(signers.firstSigner).toContainText('138****8000');
    await expect(signers.firstSigner).not.toContainText('13800138000');

    const persisted = await page.evaluate(() => [
      ...Object.values(localStorage),
      ...Object.values(sessionStorage),
    ]);
    expect(persisted.join('\n')).not.toContain('测试经办人');
    expect(persisted.join('\n')).not.toContain('110105199001011234');
    expect(persisted.join('\n')).not.toContain('13800138000');
    await captureCustomerScreenshot(page, 'customer-contract-signer-verified');

    await signers.disableSigner();
    await expect(signers.firstSigner).toContainText('已停用');
    await expect(signers.firstSigner).not.toContainText('可用于新合同');
  });

  test('TC003b: 短信发送结果不安全时不自动重发，只提示主动获取新题', async ({
    page,
  }) => {
    let sendCount = 0;
    await page.route('**/phone-verification-codes', async (route) => {
      sendCount += 1;
      await route.fulfill({
        json: { code: 503, data: null, message: 'result unknown' },
        status: 503,
      });
    });
    const signers = new ContractSignersPage(page);
    await signers.open();
    await signers.addSigner();
    await signers.firstSigner.getByRole('button', { name: '验证手机号' }).click();
    await page.getByTestId('contract-signer-phone-answer').fill('7');
    await page
      .getByRole('button', { name: '验证并发送短信验证码' })
      .click();

    await expect(page.getByTestId('contract-signer-phone-uncertain')).toContainText(
      '系统不会自动重复发送',
    );
    await expect(page.getByText('result unknown')).toHaveCount(0);
    await page.waitForTimeout(500);
    expect(sendCount).toBe(1);
    await expect(page.getByRole('button', { name: '获取新题' })).toBeVisible();
    await captureCustomerScreenshot(page, 'customer-contract-signer-sms-unknown');
  });
});

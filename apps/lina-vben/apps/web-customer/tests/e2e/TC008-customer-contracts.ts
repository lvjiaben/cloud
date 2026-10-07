import { ContractsPage } from '../pages/ContractsPage';
import { installCustomerContractMock } from '../support/customer-contract-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';
import { expect, test } from '../fixtures/customer';

test.describe('TC008 客户合同签署', () => {
  test('TC008a: 返回页面不冒充成功，权威检查后才完成原合同', async ({
    page,
  }) => {
    const calls = await installCustomerContractMock(page);
    const contracts = new ContractsPage(page);
    await contracts.login();
    await contracts.open(true);

    await expect(
      page.getByTestId('contract-signing-return-notice'),
    ).toContainText('返回本页面不代表签署已经完成');
    const processing = contracts.contract('CONTRACT-PROCESSING');
    await expect(processing).toContainText('处理中');

    const popupPromise = page.waitForEvent('popup');
    await processing.getByTestId('contract-open-signing').click();
    const popup = await popupPromise;
    await popup.waitForURL(/sign\.tencent\.example\/h5\/CONTRACT-PROCESSING/u);
    expect(calls.signingRequests).toEqual(['CONTRACT-PROCESSING']);
    await popup.close();

    await processing.getByTestId('contract-recheck').click();
    await expect(processing).toContainText('已完成');
    await expect(processing.getByTestId('contract-open-signing')).toHaveCount(
      0,
    );
    await expect(processing.getByTestId('contract-recheck')).toHaveCount(0);
    expect(calls.authoritativeChecks).toEqual(['CONTRACT-PROCESSING']);
    await captureCustomerScreenshot(page, 'customer-contract-completed-zh-cn');
  });

  test('TC008b: 安全重试只为原合同取得新入口，终态失败不能重发', async ({
    page,
  }) => {
    const calls = await installCustomerContractMock(page);
    const contracts = new ContractsPage(page);
    await contracts.login();
    await contracts.open();

    const retryable = contracts.contract('CONTRACT-RETRY');
    await expect(retryable).toContainText('处理中');
    await expect(contracts.root).toContainText('原合同、原流程');

    for (const entry of [1, 2]) {
      const popupPromise = page.waitForEvent('popup');
      await retryable.getByTestId('contract-open-signing').click();
      const popup = await popupPromise;
      await popup.waitForURL(
        new RegExp(
          `sign\\.tencent\\.example/h5/CONTRACT-RETRY\\?entry=${entry}`,
          'u',
        ),
      );
      await popup.close();
    }
    expect(calls.signingRequests).toEqual(['CONTRACT-RETRY', 'CONTRACT-RETRY']);
    expect(calls.authoritativeChecks).toEqual([]);

    const terminal = contracts.contract('CONTRACT-TERMINAL-FAILED');
    await expect(terminal).toContainText('失败');
    await expect(terminal.getByTestId('contract-open-signing')).toHaveCount(0);
    await expect(terminal.getByTestId('contract-recheck')).toHaveCount(0);
  });

  test('TC008c: 非 HTTPS 签署地址会被关闭并明确提示', async ({ page }) => {
    await installCustomerContractMock(page, { unsafeSigningLink: true });
    const contracts = new ContractsPage(page);
    await contracts.login();
    await contracts.open();

    const popupPromise = page.waitForEvent('popup');
    await contracts
      .contract('CONTRACT-PROCESSING')
      .getByTestId('contract-open-signing')
      .click();
    const popup = await popupPromise;
    await expect.poll(() => popup.isClosed()).toBe(true);
    await expect(
      page.getByText('服务端没有返回安全的 HTTPS 签署入口'),
    ).toBeVisible();
  });
});

test.describe('TC008 customer contracts on English mobile', () => {
  test.use({ locale: 'en-US', viewport: { height: 844, width: 390 } });

  test('TC008d: English mobile page is translated and has no horizontal overflow', async ({
    page,
  }) => {
    await installCustomerContractMock(page, { locale: 'en-US' });
    const contracts = new ContractsPage(page);
    await contracts.login();
    await contracts.open(true);

    await expect(contracts.root).toContainText(
      'Review and complete your electronic contracts',
    );
    await expect(contracts.root).toContainText(
      'Returning to this page does not prove completion',
    );
    await expect(contracts.root).not.toContainText('contracts.mine.');
    expect(await contracts.hasHorizontalOverflow()).toBe(false);
    await captureCustomerScreenshot(page, 'customer-contracts-mobile-en-us');
  });
});

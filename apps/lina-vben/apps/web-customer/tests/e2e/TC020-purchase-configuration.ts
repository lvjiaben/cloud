import { expect, test } from '../fixtures/customer';
import { CommercePage } from '../pages/CommercePage';
import { installPurchaseConfigurationMock } from '../support/purchase-configuration-api-mock';

test.describe('TC020 通用商品购买配置与受治理文件', () => {
  test('TC020b: 购物车只相信服务端冻结状态且已配置项可以重新配置', async ({
    page,
  }) => {
    const state = await installPurchaseConfigurationMock(page);
    const commerce = new CommercePage(page);
    await commerce.login();
    await page.evaluate(() =>
      window.sessionStorage.setItem(
        'lina.customer.structured-configured-items',
        JSON.stringify(['quote-one-item']),
      ),
    );
    await page.goto('/account/cart');
    await expect(
      page.getByRole('button', { name: '填写购买配置' }),
    ).toBeVisible();
    await expect(page.getByTestId('commerce-primary-action')).toBeDisabled();

    state.cartFrozen = true;
    await page.evaluate(() => window.sessionStorage.clear());
    await page.reload();
    await expect(page.getByText('配置已完成')).toBeVisible();
    const reconfigure = page.getByRole('button', { name: '重新配置' });
    await expect(reconfigure).toBeVisible();
    await reconfigure.click();
    await expect.poll(() => state.quoteRequests).toBe(1);
    await expect(page).toHaveURL(/quoteItemNo=quote-reconfigured-item/u);
  });

  test('TC020a: 商品入口可达，文件可上传、撤销、重传并参与校验报价', async ({
    page,
  }) => {
    const state = await installPurchaseConfigurationMock(page);
    const commerce = new CommercePage(page);
    await commerce.login();
    await page.goto('/account/products/catalog');
    await page.getByRole('button', { name: '选择配置' }).click();
    await expect(page).toHaveURL(
      /\/account\/products\/ssl-certificate\/configure/u,
    );
    const input = page.locator('input[type="file"]');
    await input.setInputFiles({
      buffer: Buffer.from('%PDF governed request one'),
      mimeType: 'application/pdf',
      name: 'request.pdf',
    });
    await expect(page.getByText('request.pdf')).toBeVisible();
    await expect
      .poll(() => state.evaluationFileTokens.at(-1)?.length ?? 0)
      .toBe(1);
    await page.getByRole('button', { name: '移除' }).click();
    await expect.poll(() => state.revoked.length).toBe(1);
    await input.setInputFiles({
      buffer: Buffer.from('%PDF governed request two'),
      mimeType: 'application/pdf',
      name: 'request.pdf',
    });
    await expect.poll(() => state.uploaded.length).toBe(2);
    const requirement = page.getByRole('checkbox', {
      name: '需要证书申请文件',
    });
    await requirement.uncheck();
    await expect(page.getByText('request.pdf')).toHaveCount(0);
    await requirement.check();
    await expect
      .poll(() =>
        state.evaluationFileTokens.some((tokens) => tokens.length === 0),
      )
      .toBe(true);
    await expect(page.getByText('request.pdf')).toBeVisible();
    expect(state.revoked).toHaveLength(1);
    await page.getByRole('button', { name: '校验并查看价格' }).click();
    await expect(
      page.getByTestId('purchase-configuration-quote'),
    ).toContainText('CNY 10.00');
    expect(state.evaluationFileTokens.at(-1)).toEqual([state.uploaded[1]]);
  });
});

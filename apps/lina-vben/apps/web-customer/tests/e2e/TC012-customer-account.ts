import { expect, test } from '../fixtures/customer';
import { installCustomerAccountMock } from '../support/customer-account-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

const foreignCustomerNo = 'CUS_ffffffffffffffffffffffffffffffff';

async function login(page: Parameters<typeof installCustomerAccountMock>[0]) {
  await page.goto('/auth/login');
  const inputs = page.locator('input');
  await inputs.nth(0).fill('customer');
  await inputs.nth(1).fill('customer-password');
  await page.getByRole('button', { name: 'login' }).click();
  await page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
}

test.describe('TC012 客户账户所有权隔离', () => {
  test('TC-2d: 中国地址用省市区三级联动并在 Vben 弹窗内保存', async ({ page }) => {
    const state = await installCustomerAccountMock(page);
    await login(page);
    await page.goto('/account/addresses');
    await page.getByTestId('customer-address-add').click();
    const dialog = page.getByRole('dialog', { name: '添加地址' });
    await expect(dialog).toBeVisible();

    await page.getByTestId('customer-address-region').click();
    for (const name of ['北京市', '市辖区', '东城区']) {
      await page
        .locator('.ant-cascader-menus:visible .ant-cascader-menu-item-content')
        .filter({ hasText: name })
        .first()
        .click();
    }
    await dialog
      .getByTestId('customer-address-line1')
      .fill('朝阳门内大街 1 号');
    await dialog.getByRole('button', { name: /保\s*存/u }).click();

    await expect.poll(() => state.addressWrites.length).toBe(1);
    expect(state.addressWrites[0]).toMatchObject({
      city: '市辖区',
      countryCode: 'CN',
      district: '东城区',
      province: '北京市',
    });
  });
  test('TC-2c: 地址栏伪造客户编号仍只显示当前登录客户', async ({ page }) => {
    const state = await installCustomerAccountMock(page);
    await login(page);

    await page.goto(`/account/profile?customerNo=${foreignCustomerNo}`);
    const root = page.getByTestId('customer-profile-page');
    await root.waitFor({ state: 'visible' });

    await expect(page.getByTestId('customer-profile-display-name')).toHaveValue(
      '当前登录客户',
    );
    await expect(root).toContainText('10000');
    await expect(root).not.toContainText(state.localCustomerNo);
    await expect(root).not.toContainText(foreignCustomerNo);
    await expect.poll(() => state.profileRequestURLs.length).toBeGreaterThan(0);
    expect(state.profileRequestURLs).toEqual(
      expect.arrayContaining([
        expect.not.stringContaining(`customerNo=${foreignCustomerNo}`),
      ]),
    );
    expect(
      state.profileRequestURLs.every((url) => !url.includes('customerNo=')),
    ).toBe(true);
    await captureCustomerScreenshot(page, 'customer-account-ownership-zh-cn');
  });
});

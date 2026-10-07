import { expect, test } from '../fixtures/customer';
import {
  installCustomerActivationMock,
  installCustomerOnboardingMock,
} from '../support/customer-onboarding-api-mock';
import { captureCustomerScreenshot } from '../support/screenshot';

async function fillRegistration(page: import('@playwright/test').Page) {
  await page.locator('[name="username"]').fill('selfbuyer');
  await page.locator('[name="email"]').fill('selfbuyer@example.com');
  await page.locator('[name="password"]').fill('Passw0rd!');
  await page.locator('[name="confirmPassword"]').fill('Passw0rd!');
  await page
    .getByRole('checkbox', { name: /我同意|I agree/iu })
    .first()
    .click();
  await page.getByRole('button', { name: /register|注册|Sign up/iu }).click();
}

async function login(page: import('@playwright/test').Page) {
  const submit = page.getByRole('button', { name: 'login' });
  await submit.waitFor({ state: 'visible' });
  await page.locator('[name="username"]').last().fill('selfbuyer');
  await page.locator('[name="password"]').last().fill('Passw0rd!');
  await submit.click();
}

test.describe('TC023 客户开户与管理员邀请领取', () => {
  test('TC023-login: 登录页提供注册入口并保持客户路由边界', async ({
    page,
  }) => {
    await installCustomerOnboardingMock(page);
    await page.goto('/auth/login');

    await expect(page.getByTestId('login-create-account')).toContainText(
      '创建账号',
    );
    await captureCustomerScreenshot(page, 'customer-login-register-entry');
    await page.getByTestId('login-create-account-link').focus();
    await page.keyboard.press('Enter');
    await page.waitForURL(/\/auth\/register(?:$|\?)/u);
    expect(new URL(page.url()).pathname).toBe('/auth/register');
  });

  test('TC023a: 普通注册完成真人验证、验证码和一次性客户绑定', async ({
    page,
  }) => {
    const state = await installCustomerOnboardingMock(page);
    await page.goto('/auth/register');
    await fillRegistration(page);

    await expect(page.getByTestId('customer-onboarding-prompt')).toHaveText(
      '2 + 3 = ?',
    );
    await page.getByTestId('customer-onboarding-human-answer').fill('5');
    await page
      .getByRole('dialog', { name: '确认您是真人操作' })
      .locator('.ant-btn-primary')
      .click();
    await page.getByTestId('customer-onboarding-code').fill('123456');
    await page
      .getByRole('dialog', { name: '验证联系方式' })
      .locator('.ant-btn-primary')
      .click();

    await page.waitForURL(/\/auth\/login/u);
    expect(state.humanChallenges).toHaveLength(1);
    expect(state.humanCompletions).toHaveLength(1);
    expect(state.verificationRequests).toHaveLength(1);
    expect(state.intentRequests).toHaveLength(1);
    expect(state.registrations).toEqual([
      {
        email: 'selfbuyer@example.com',
        password: 'Passw0rd!',
        username: 'selfbuyer',
      },
    ]);

    await login(page);
    await page.waitForURL(/\/auth\/customer-onboarding-complete/u);
    await expect(page.getByText('客户开户完成', { exact: true })).toBeVisible();
    expect(state.redeemRequests).toHaveLength(1);
    expect(state.redeemRequests[0]).toMatchObject({
      hostRegistrationUserId: 2001,
      token: 'T'.repeat(43),
    });
    expect(String(state.redeemRequests[0]?.browserNonce)).toMatch(
      /^browser-[0-9a-f]{36}$/u,
    );
  });

  test('TC023b: 管理员邀请进入注册时不重复走自行开户', async ({ page }) => {
    const state = await installCustomerOnboardingMock(page);
    await page.goto(
      '/auth/register?redirect=%252Fauth%252Fcustomer-activation',
    );
    await expect(page.getByText('客户类型', { exact: true })).toHaveCount(0);
    await fillRegistration(page);
    await page.waitForURL(/\/auth\/login/u);
    expect(state.humanChallenges).toHaveLength(0);
    expect(state.intentRequests).toHaveLength(0);
    expect(state.registrations).toHaveLength(1);
    expect(new URL(page.url()).searchParams.get('redirect')).toBe(
      '%2Fauth%2Fcustomer-activation',
    );
  });

  test('TC023c: 已被重发替换和已过期邀请都拒绝，当前有效邀请可领取', async ({
    page,
  }) => {
    const state = await installCustomerActivationMock(page);
    await page.goto('/auth/login');
    await login(page);
    await page.waitForURL(/\/account/u);

    await page.goto(
      `/auth/customer-activation?token=${encodeURIComponent(`replaced-${'R'.repeat(40)}`)}`,
    );
    await expect(
      page.getByText('无法领取这条邀请', { exact: true }),
    ).toBeVisible();
    await page.goto(
      `/auth/customer-activation?token=${encodeURIComponent(`expired-${'E'.repeat(40)}`)}`,
    );
    await expect(
      page.getByText('无法领取这条邀请', { exact: true }),
    ).toBeVisible();
    await page.goto(
      `/auth/customer-activation?token=${encodeURIComponent(`active-${'A'.repeat(40)}`)}`,
    );
    await expect(
      page.getByText('客户账号已激活', { exact: true }),
    ).toBeVisible();
    expect(state.tokens.map((token) => token.split('-')[0])).toEqual([
      'replaced',
      'expired',
      'active',
    ]);
  });
});

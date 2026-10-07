import type { Locator, Page } from '@playwright/test';

export class PaymentChannelsPage {
  readonly page: Page;
  readonly financeRoot: Locator;

  constructor(page: Page) {
    this.page = page;
    this.financeRoot = page.getByTestId('customer-finance-page');
  }

  async login() {
    await this.page.goto('/auth/login');
    const inputs = this.page.locator('input');
    await inputs.nth(0).fill('customer');
    await inputs.nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login' }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async openRecharge() {
    await this.page.goto('/account/billing');
    await this.financeRoot.waitFor();
    await this.page.getByTestId('finance-recharge-open').click();
  }

  async createRecharge(amount = '12.34'): Promise<{ attemptNo: string }> {
    const redirectRequest = this.page.waitForRequest(
      (request) => request.url() === 'https://payments.example.test/submit.php',
    );
    await this.page.getByRole('spinbutton', { name: '充值金额' }).fill(amount);
    await this.page.getByRole('button', { name: '继续付款' }).click();
    const request = await redirectRequest;
    const values = new URLSearchParams(request.postData() || '');
    return { attemptNo: values.get('out_trade_no') || '' };
  }

  async selectRechargeChannel(label: string) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('.ant-select').nth(1).click();
    await this.page.getByText(label, { exact: true }).last().click();
  }

  async selectRechargeMode(label: string) {
    const dialog = this.page.getByRole('dialog');
    await dialog.locator('.ant-select').nth(2).click();
    await this.page.getByText(label, { exact: true }).last().click();
  }

  async submitRechargeWithoutWaitingForPost(amount = '12.34') {
    await this.page.getByRole('spinbutton', { name: '充值金额' }).fill(amount);
    await this.page.getByRole('button', { name: '继续付款' }).click();
  }

  async openReturn(attemptNo: string) {
    await this.page.goto(`/payment/return/${encodeURIComponent(attemptNo)}`);
    return this.page.getByTestId('payment-return-page');
  }

  async balance(currency = 'CNY') {
    await this.page.goto('/account/billing');
    await this.financeRoot.waitFor();
    const text = await this.page
      .getByTestId(`finance-balance-${currency}`)
      .locator('dd')
      .first()
      .textContent();
    return Number(String(text || '0').replace(/[^0-9.-]/gu, ''));
  }
}

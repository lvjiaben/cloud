import type { Page } from '@playwright/test';

export class InvoiceProfilesPage {
  constructor(readonly page: Page) {}

  get root() {
    return this.page.getByTestId('invoice-profile-page');
  }

  profile(profileNo: string) {
    return this.page.getByTestId(`invoice-profile-${profileNo}`);
  }

  async login() {
    await this.page.goto('/auth/login');
    const inputs = this.page.locator('input');
    await inputs.nth(0).fill('customer');
    await inputs.nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login' }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async open() {
    await this.page.goto('/account/invoice-profiles');
    await this.root.waitFor({ state: 'visible' });
  }

  async addEnterprise(values: {
    bankAccount: string;
    bankName: string;
    invoiceTitle: string;
    taxNo: string;
  }) {
    await this.page.getByTestId('invoice-profile-add').click();
    await this.page.getByTestId('invoice-profile-title').fill(values.invoiceTitle);
    await this.page.getByTestId('invoice-profile-tax-no').fill(values.taxNo);
    await this.page.getByTestId('invoice-profile-bank-name').fill(values.bankName);
    await this.page.getByTestId('invoice-profile-bank-account').fill(values.bankAccount);
    await this.page
      .getByRole('button', { name: /保\s*存|确\s*认|Save|Confirm/u })
      .click();
  }

  async replaceBank(profileNo: string, value: string) {
    const card = this.profile(profileNo);
    await card.getByTestId('invoice-profile-edit').click();
    await this.page.getByText(/更换账号|Replace account/u).click();
    await this.page.getByTestId('invoice-profile-bank-account').fill(value);
    await this.page
      .getByRole('button', { name: /保\s*存|确\s*认|Save|Confirm/u })
      .click();
  }

  async setDefault(profileNo: string) {
    await this.page.getByTestId(`invoice-profile-default-${profileNo}`).click();
  }

  async deactivate(profileNo: string) {
    await this.page.getByTestId(`invoice-profile-deactivate-${profileNo}`).click();
    await this.page
      .getByRole('button', { name: /确\s*认\s*停\s*用|Deactivate/u })
      .click();
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () =>
        document.documentElement.scrollWidth >
        document.documentElement.clientWidth,
    );
  }
}

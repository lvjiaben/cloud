import type { Locator, Page } from '@playwright/test';

export class TaxInvoicesPage {
  readonly history: Locator;
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId('tax-invoice-page');
    this.history = page.getByTestId('tax-invoice-history');
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
    await this.page.goto('/account/tax-invoices');
    await this.root.waitFor();
  }

  async selectBill(billingInvoiceNo: string, amount: string) {
    await this.page
      .getByTestId(`tax-invoice-select-${billingInvoiceNo}`)
      .check();
    await this.page
      .getByTestId(`tax-invoice-amount-${billingInvoiceNo}`)
      .fill(amount);
  }

  async submit() {
    await this.page.getByTestId('tax-invoice-submit').click();
  }

  historyItem(taxInvoiceNo: string) {
    return this.page.getByTestId(`tax-invoice-history-${taxInvoiceNo}`);
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
  }
}

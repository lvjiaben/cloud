import type { Locator, Page } from '@playwright/test';

export class ContractsPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId('customer-contract-page');
  }

  async login() {
    await this.page.goto('/auth/login');
    const inputs = this.page.locator('input');
    await inputs.nth(0).fill('customer');
    await inputs.nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login' }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async open(returnedFromSigning = false) {
    await this.page.goto(
      returnedFromSigning
        ? '/account/contracts?signing_return=1'
        : '/account/contracts',
    );
    await this.root.waitFor();
  }

  contract(contractNo: string) {
    return this.page.getByTestId(`contract-${contractNo}`);
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
  }
}

import type { Locator, Page } from '@playwright/test';

import { captureCustomerScreenshot } from '../support/screenshot';

export class CreditPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId('customer-credit-page');
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
    await this.page.goto('/account/billing/credit');
    await this.root.waitFor({ state: 'visible' });
  }

  async screenshot(description: string) {
    return captureCustomerScreenshot(this.page, description);
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
  }
}

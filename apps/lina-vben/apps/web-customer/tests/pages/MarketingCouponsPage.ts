import type { Locator, Page } from '@playwright/test';

import { captureCustomerScreenshot } from '../support/screenshot';

export class MarketingCouponsPage {
  readonly codeInput: Locator;
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId('customer-marketing-coupons-page');
    this.codeInput = page.getByRole('textbox', {
      name: /优惠码|Promotion code/iu,
    });
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
    await this.page.goto('/account/coupons');
    await this.root.waitFor({ state: 'visible' });
  }

  async redeem(code: string) {
    await this.codeInput.fill(code);
    await this.page.getByTestId('customer-marketing-coupon-redeem').click();
  }

  async screenshot(description: string) {
    return captureCustomerScreenshot(this.page, description);
  }
}

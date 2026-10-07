import type { Locator, Page } from '@playwright/test';

export class PrivacyPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId('customer-privacy-page');
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
    await this.page.goto('/account/privacy');
    await this.root.waitFor({ state: 'visible' });
  }

  async confirm() {
    const dialog = this.page.getByRole('dialog').last();
    await dialog.locator('.ant-btn-primary').last().click();
    await dialog.waitFor({ state: 'hidden' });
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
  }
}

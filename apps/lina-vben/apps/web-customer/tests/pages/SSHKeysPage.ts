import type { Locator, Page } from '@playwright/test';

import { captureCustomerScreenshot } from '../support/screenshot';

export class SSHKeysPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId('customer-ssh-keys-page');
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
    await this.page.goto('/account/ssh-keys');
    await this.root.waitFor({ state: 'visible' });
  }

  async openCreate() {
    await this.page.getByTestId('ssh-key-create').click();
    const dialog = this.page.getByRole('dialog');
    await dialog.waitFor({ state: 'visible' });
    await this.page.waitForTimeout(500);
    return dialog;
  }

  async screenshot(description: string) {
    return captureCustomerScreenshot(this.page, description);
  }
}

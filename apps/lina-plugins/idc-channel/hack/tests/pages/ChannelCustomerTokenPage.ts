import { mkdir } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';

import type { Locator, Page } from '@playwright/test';

const screenshotDirectory = fileURLToPath(
  new URL('../../../../../../temp/', import.meta.url),
);

export class ChannelCustomerTokenPage {
  readonly createButton: Locator;
  readonly oneTimeToken: Locator;
  readonly root: Locator;
  readonly tokenItems: Locator;

  constructor(readonly page: Page) {
    this.root = page.getByTestId('channel-customer-token-page');
    this.createButton = this.root.getByTestId('create-own-token');
    this.oneTimeToken = this.root.getByTestId('one-time-token');
    this.tokenItems = this.root.locator('.token-item');
  }

  async login() {
    const baseURL =
      process.env.E2E_CUSTOMER_BASE_URL?.trim() || 'http://127.0.0.1:6032';
    await this.page.goto(`${baseURL.replace(/\/$/u, '')}/auth/login`);
    const inputs = this.page.locator('input');
    await inputs.nth(0).fill('customer');
    await inputs.nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login' }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async goto() {
    await this.page.goto('/account/products/channel-tokens', {
      waitUntil: 'domcontentloaded',
    });
  }

  async createToken() {
    await this.createButton.click();
  }

  async revokeFirstToken() {
    await this.root.locator('[data-testid^="revoke-own-token-"]').first().click();
  }

  async screenshot(description: string) {
    await mkdir(screenshotDirectory, { recursive: true });
    const timestamp = new Date().toISOString().slice(0, 19).replaceAll(/[:T]/g, '-');
    await this.page.screenshot({
      path: `${screenshotDirectory}${timestamp}-${description}.png`,
      fullPage: false,
    });
  }
}

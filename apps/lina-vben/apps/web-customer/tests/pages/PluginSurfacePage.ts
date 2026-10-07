import type { Locator, Page } from '@playwright/test';

import { captureCustomerScreenshot } from '../support/screenshot';

export class PluginSurfacePage {
  readonly page: Page;
  readonly productsGroupTitle: string;
  readonly surfaceMenu: Locator;

  constructor(
    page: Page,
    surfaceTitle = '手机订单',
    productsGroupTitle = '产品与服务',
  ) {
    this.page = page;
    this.productsGroupTitle = productsGroupTitle;
    this.surfaceMenu = page
      .locator('li.vben-sub-menu')
      .filter({ hasText: productsGroupTitle })
      .first()
      .getByText(surfaceTitle, { exact: true })
      .first();
  }

  async login() {
    await this.page.goto('/auth/login');
    const inputs = this.page.locator('input');
    await inputs.nth(0).fill('customer');
    await inputs.nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login' }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async openProductsGroup() {
    const group = this.page
      .locator('li.vben-sub-menu')
      .filter({ hasText: this.productsGroupTitle })
      .first();
    if ((await group.getAttribute('aria-expanded')) !== 'true') {
      await group
        .getByText(this.productsGroupTitle, { exact: true })
        .first()
        .click();
    }
    await group.waitFor({ state: 'visible' });
    await this.surfaceMenu.waitFor({ state: 'visible' });
  }

  async openSurface() {
    await this.surfaceMenu.click();
    await this.page.waitForURL(
      /\/account\/plugins\/phone-provider\/phone-orders(?:\?.*)?$/u,
    );
  }

  pluginFrame() {
    return this.page.frameLocator(
      'iframe[src="/x-assets/phone-provider/v1.0.0/customer/phone-orders/index.html"]',
    );
  }

  async screenshot(description: string) {
    return captureCustomerScreenshot(this.page, description);
  }
}

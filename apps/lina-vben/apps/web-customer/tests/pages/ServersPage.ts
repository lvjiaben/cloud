import type { Locator, Page } from '@playwright/test';

import { captureCustomerScreenshot } from '../support/screenshot';
import { serverServiceNo } from '../support/customer-server-api-mock';

export class ServersPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId('customer-servers-page');
  }

  async login() {
    await this.page.goto('/auth/login');
    const inputs = this.page.locator('input');
    await inputs.nth(0).fill('customer');
    await inputs.nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login' }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async openDetail() {
    await this.page.goto(`/account/servers/${serverServiceNo}`);
    await this.page
      .getByTestId('customer-server-detail')
      .waitFor({ state: 'visible' });
  }

  async screenshot(description: string) {
    return captureCustomerScreenshot(this.page, description);
  }

  get trafficPackages() { return this.page.getByTestId('traffic-package-purchase'); }
  get trafficConfirmation() { return this.page.getByRole('dialog').filter({ hasText: '已确认 100 GB 套餐' }); }

  async openTrafficPackages() {
    await this.page.getByTestId('server-action-traffic_add').click();
    await this.trafficPackages.waitFor();
  }

  async selectTrafficPackage() {
    await this.trafficPackages.locator('.vxe-table--main-wrapper .vxe-body--row')
      .filter({ hasText: '加量 100 GB' }).getByRole('button', { name: '购买' }).click();
    await this.trafficConfirmation.waitFor();
  }

  async confirmTrafficPackage() {
    await this.trafficConfirmation.getByRole('button', { name: '确认下单' }).click();
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth,
    );
  }
}

// Customer trial conversion interactions; no host files are modified.
import { fileURLToPath } from 'node:url';
import { mkdir } from 'node:fs/promises';
import type { Page } from '@playwright/test';
export class TrialConversionPage {
  constructor(readonly page: Page) {}
  async login() {
    await this.page.goto('/auth/login');
    await this.page.locator('input').nth(0).fill('customer');
    await this.page.locator('input').nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login', exact: true }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }
  async goto() { await this.page.goto('/account/plugins/idc-core/trial-conversions'); }
  async open() { await this.page.getByTestId('trial-convert').click(); }
  async quote() { await this.page.getByRole('button', { name: '查看报价', exact: true }).click(); }
  async pay() { await this.page.getByRole('button', { name: '确认并用余额付款', exact: true }).click(); }
  async screenshot(name: string) {
    const directory = fileURLToPath(new URL('../../../../../../temp/20261015/', import.meta.url));
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({ path: `${directory}${Date.now()}-trial-${name}.png`, fullPage: false });
  }
}

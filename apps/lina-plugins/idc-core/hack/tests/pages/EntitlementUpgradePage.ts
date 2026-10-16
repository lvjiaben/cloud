// Customer upgrade interactions and screenshots remain in the owning plugin.
import { fileURLToPath } from 'node:url';
import { mkdir } from 'node:fs/promises';
import type { Page } from '@playwright/test';
export class EntitlementUpgradePage {
  constructor(readonly page: Page) {}
  get dialog() { return this.page.getByRole('dialog'); }
  async login() {
    await this.page.goto('/auth/login');
    await this.page.locator('input').nth(0).fill('customer');
    await this.page.locator('input').nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login', exact: true }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }
  async goto() { await this.page.goto('/account/plugins/idc-core/entitlement-upgrades'); }
  async open() { await this.page.getByTestId('upgrade-open').first().click(); }
  async chooseProduct() {
    await this.page.getByTestId('assisted-item-product-0').click();
    await this.page.locator('.ant-select-item-option-content').getByText('测试云服务器', { exact: true }).click();
    await this.dialog.getByText('处理器核心数', { exact: true }).click();
  }
  async preview() { await this.page.getByTestId('upgrade-preview').click(); }
  async pay() { await this.page.getByTestId('upgrade-pay').click(); }
  async screenshot(name: string) {
    const directory = fileURLToPath(new URL('../../../../../../temp/20261015/', import.meta.url));
    await mkdir(directory, { recursive: true });
    await this.page.locator('#__app-loading__').waitFor({ state: 'hidden' });
    await this.page.screenshot({ path: `${directory}${Date.now()}-upgrade-${name}.png`, animations: "disabled", fullPage: false });
  }
}

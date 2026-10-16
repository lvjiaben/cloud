// Customer add-on interactions and visual evidence stay with their owning plugin.
import { fileURLToPath } from 'node:url';
import { mkdir } from 'node:fs/promises';
import type { Page } from '@playwright/test';
export class CustomerAddonPage {
  constructor(readonly page: Page) {}
  async login() {
    await this.page.goto('/auth/login');
    await this.page.locator('input').nth(0).fill('customer');
    await this.page.locator('input').nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login', exact: true }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }
  async goto() { await this.page.goto('/account/plugins/idc-core/addons'); }
  async choose() {
    await this.page.getByTestId('assisted-item-product-0').click();
    await this.page.locator('.ant-select-item-option-content').getByText('运维服务', { exact: true }).click();
    await this.page.getByText('处理器核心数', { exact: true }).click();
  }
  async quote() { await this.page.getByTestId('addon-quote').click(); }
  async pay() { await this.page.getByTestId('addon-pay').click(); }
  async reset() { await this.page.getByRole('button', {name:'重新选择',exact:true}).click(); }
  async screenshot(name: string) {
    const directory = fileURLToPath(new URL('../../../../../../temp/20261015/', import.meta.url));
    await mkdir(directory, {recursive:true});
    await this.page.locator('#__app-loading__').waitFor({state:'hidden'});
    await this.page.screenshot({path:`${directory}${Date.now()}-addon-${name}.png`,animations:"disabled",fullPage:false});
  }
}

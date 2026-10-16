import { workspacePath } from '@host-tests/fixtures/config';
import { expect, type Page } from '@host-tests/support/playwright';
import { mkdir } from 'node:fs/promises';
import path from 'node:path';

export class CurrencyManagementPage {
  constructor(readonly page: Page) {}
  get workspace() { return this.page.getByTestId('currency-management'); }
  get modal() { return this.page.getByTestId('currency-create-modal'); }
  async goto() {
    const loaded = this.page.waitForResponse(r => r.url().includes('/admin/currencies') && r.request().method() === 'GET');
    await this.page.goto(workspacePath('/idc/currencies')); return (await loaded).json();
  }
  async openCreate() { await this.page.getByTestId('currency-add').click(); }
  async select(code: string) {
    await this.page.getByTestId('currency-code').click();
    await this.page.getByTestId('currency-code').getByRole('combobox').fill(code);
    await this.page.locator('.ant-select-dropdown:visible .ant-select-item-option-content').filter({ hasText: `(${code})` }).click();
  }
  async submit() {
    const saved = this.page.waitForResponse(r => r.url().endsWith('/admin/currencies') && r.request().method() === 'POST');
    await this.page.getByRole('button', { name: /确\s*认/u }).click(); return (await saved).json();
  }
  async close() { await this.page.getByRole('button', { name: /取\s*消/u }).click(); }
  async change(code: string, makeDefault = false) {
    const saved = this.page.waitForResponse(r => r.url().endsWith(`/admin/currencies/${code}`) && r.request().method() === 'PUT');
    await expect(this.page.locator('.ant-popover:visible')).toHaveCount(0);
    const reloaded = this.page.waitForResponse(r => r.url().includes('/admin/currencies?') && r.request().method() === 'GET');
    await this.page.getByTestId(`currency-${makeDefault ? 'default' : 'toggle'}-${code}`).click();
    await this.page.locator('.ant-popover:visible').getByRole('button', { name: /确\s*定|确\s*认/u }).click();
    const result = await (await saved).json();
    await reloaded;
    await expect(this.page.locator('.ant-popover:visible')).toHaveCount(0);
    return result;
  }
  async remove(code: string) {
    await expect(this.page.locator('.ant-popover:visible')).toHaveCount(0);
    const deleted = this.page.waitForResponse(r => r.url().includes(`/admin/currencies/${code}`) && r.request().method() === 'DELETE');
    const reloaded = this.page.waitForResponse(r => r.url().includes('/admin/currencies?') && r.request().method() === 'GET');
    await this.page.getByTestId(`currency-delete-${code}`).click();
    await this.page.locator('.ant-popover:visible').getByRole('button', { name: /确\s*定|确\s*认/u }).click();
    const result = await (await deleted).json(); await reloaded;
    await expect(this.page.getByTestId(`currency-delete-${code}`)).toHaveCount(0);
    return result;
  }
  async screenshot(name: string) {
    const folder = path.resolve(import.meta.dirname, '../../../../../../temp/20261012/currency'); await mkdir(folder, { recursive: true });
    await this.page.screenshot({ path: path.join(folder, `${Date.now()}-${name}.png`), animations: 'disabled' });
  }
}

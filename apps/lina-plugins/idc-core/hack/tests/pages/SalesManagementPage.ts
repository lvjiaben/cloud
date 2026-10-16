// Page objects own agent management selectors and user interactions.
import { mkdir } from 'node:fs/promises';
import path from 'node:path';
import type { Page, Response } from '@host-tests/support/playwright';
import { workspacePath } from '@host-tests/fixtures/config';

export class SalesManagementPage {
  constructor(private readonly page: Page) {}
  get workspace() { return this.page.getByTestId('sales-channel-workspace'); }
  get drawer() { return this.page.getByTestId('sales-management-drawer'); }
  get addButton() { return this.page.getByTestId('sales-add'); }
  get formError() { return this.page.getByTestId('sales-form-error'); }
  get levelSearch() { return this.page.getByPlaceholder('输入等级名称'); }
  get performancePanel() { return this.page.getByTestId('sales-performance'); }
  async openPerformance() {
    const loaded = this.waitForResponse(response => response.url().includes('/sales-management/performance'));
    await this.tab('销售业绩').click(); return (await loaded).json();
  }
  async searchPerformance(month: string, name: string, currency = '') {
    const panel = this.performancePanel;
    await panel.getByPlaceholder('到账月份', { exact: true }).click();
    const picker = this.page.locator('.ant-picker-dropdown:visible');
    const targetYear = Number(month.slice(0, 4));
    for (let attempt = 0; attempt < 12; attempt++) {
      const year = Number((await picker.locator('.ant-picker-year-btn').innerText()).replace(/\D/gu, ''));
      if (year === targetYear) break;
      await picker.locator(year > targetYear ? '.ant-picker-header-super-prev-btn' : '.ant-picker-header-super-next-btn').click();
    }
    await picker.locator(`td[title="${month}"]`).click();
    await panel.getByPlaceholder('输入销售或推广人姓名').fill(name);
    await panel.getByPlaceholder('全部币种；可输入 CNY').fill(currency);
    const loaded = this.waitForResponse(response => response.url().includes('/sales-management/performance') && new URL(response.url()).searchParams.get('search') === name && new URL(response.url()).searchParams.get('month') === month && (new URL(response.url()).searchParams.get('currencyCode') || '') === currency);
    await panel.getByRole('button', { name: /搜\s*索/u, exact: true }).click();
    return (await loaded).json();
  }
  async showPerformanceMoney() {
    await this.performancePanel.locator('.vxe-table--scroll-x-handle').evaluate(element => { element.scrollLeft = element.scrollWidth; });
  }
  async goto() { await this.page.goto(workspacePath('/idc/sales-channel'), { waitUntil: 'domcontentloaded' }); }
  tab(name: string) { return this.page.getByRole('tab', { name, exact: true }); }
  button(name: string | RegExp) { return this.page.getByRole('button', { name, exact: true }); }
  text(value: string) { return this.page.getByText(value, { exact: true }); }
  field(name: string) { return this.page.getByTestId(`sales-field-${name}`); }
  statusLabel(value: string) { return this.page.locator('.ant-tag').filter({ hasText: value }); }
  async showRecordStatus() {
    await this.page.getByTestId('sales-panel-commission/records').locator('.vxe-table--scroll-x-handle').evaluate(element => { element.scrollLeft = element.scrollWidth; });
  }
  async editLevel(name: string) {
    const loaded = this.waitForResponse(response => response.url().includes('/sales-management/levels') && new URL(response.url()).searchParams.get('search') === name);
    await this.levelSearch.fill(name); await this.levelSearch.press('Enter'); await loaded;
    await this.button(/修\s*改/u).filter({ visible: true }).click();
  }
  async editAgent(name: string) {
    const loaded = this.waitForResponse(response => response.url().includes('/sales-management/agents') && new URL(response.url()).searchParams.get('search') === name);
    const panel = this.page.getByTestId('sales-panel-agents');
    const input = panel.getByPlaceholder('输入客户姓名或编号');
    await input.fill(name); await input.press('Enter'); await loaded;
    await panel.getByRole('button', { name: /修\s*改/u }).filter({ visible: true }).click();
  }
  waitForResponse(predicate: Parameters<Page['waitForResponse']>[0]) { return this.page.waitForResponse(predicate); }
  async openCommissionAccounts(name: string) {
    await this.goto(); await this.tab('佣金账户').click();
    const loaded = this.waitForResponse(response => response.url().includes('/commission/accounts') && new URL(response.url()).searchParams.get('search') === name);
    const input = this.page.getByPlaceholder('输入收款人姓名'); await input.fill(name); await input.press('Enter');
    await loaded;
  }
  async openCommissionRecords(search: string) {
    const initial = this.waitForResponse(response => response.url().includes('/commission/records'));
    await this.tab('佣金明细').click(); await initial;
    const loaded = this.waitForResponse(response => response.url().includes('/commission/records') && new URL(response.url()).searchParams.get('search') === search);
    const input = this.page.getByPlaceholder('输入订单号、客户或收款人姓名'); await input.fill(search);
    await this.page.getByTestId('sales-panel-commission/records').getByRole('button', { name: /搜\s*索/u, exact: true }).click();
    return (await loaded).json();
  }
  async payoutButton(currency: string) {
    // VXE renders fixed actions in a second row with the same stable row ID.
    const row = this.page.locator('.vxe-table--body tr').filter({ has: this.page.getByText(currency, { exact: true }) }).first();
    const rowid = await row.getAttribute('rowid');
    if (!rowid) throw new Error('Commission account row is missing');
    return this.page.locator(`tr[rowid="${rowid}"]`).getByRole('button', { name: '登记打款', exact: true }).filter({ visible: true });
  }
  async openPayout(currency: string) { await (await this.payoutButton(currency)).click(); }
  async replayPayout(response: Response, overrides: Record<string, unknown> = {}) {
    const request = response.request(); const headers = await request.allHeaders();
    delete headers['content-length']; delete headers.host;
    const result = await this.page.request.post(response.url(), { headers, data: { ...request.postDataJSON(), ...overrides } });
    return result.json();
  }
  async input(name: string) {
    const locator = this.field(name); const tag = await locator.evaluate(element => element.tagName.toLowerCase());
    return tag === 'input' || tag === 'textarea' ? locator : locator.locator('input,textarea').first();
  }
  async fill(name: string, value: string) { await (await this.input(name)).fill(value); }
  async select(name: string, label: string) {
    const locator = this.field(name); if (await locator.getByRole('combobox').getAttribute('aria-expanded') !== 'true') await locator.click();
    await this.page.locator('.ant-select-dropdown:visible .ant-select-item-option-content').getByText(label, { exact: true }).click();
  }
  async screenshot(name: string) {
    const folder = path.resolve(import.meta.dirname, `../../../../../../temp/${new Date().toISOString().slice(0, 10).replaceAll('-', '')}`); await mkdir(folder, { recursive: true });
    await this.page.screenshot({ path: path.join(folder, `${Date.now()}-sales-${name}.png`), animations: 'disabled' });
  }
}

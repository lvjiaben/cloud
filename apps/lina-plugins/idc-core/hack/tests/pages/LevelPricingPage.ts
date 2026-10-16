import type { Page } from '@host-tests/support/playwright';
import { SalesManagementPage } from './SalesManagementPage';

export class LevelPricingPage extends SalesManagementPage {
  constructor(private readonly browserPage: Page) { super(browserPage); }
  get pricingDrawer() { return this.browserPage.getByTestId('sales-level-pricing-modal'); }
  get percentageInput() { return this.browserPage.getByTestId('sales-level-percentage').locator('input').or(this.browserPage.locator('input[data-testid="sales-level-percentage"]')).first(); }
  get pricingError() { return this.pricingDrawer.locator('.ant-alert-error'); }
  pricingButton(levelNo: string) { return this.browserPage.getByTestId(`sales-level-pricing-${levelNo}`); }
  async findLevel(name: string) {
    await this.goto();
    const loaded = this.waitForResponse(response => response.url().includes('/sales-management/levels') && new URL(response.url()).searchParams.get('search') === name);
    await this.tab('代理等级').click(); await this.levelSearch.fill(name); await this.levelSearch.press('Enter'); return (await loaded).json();
  }
  async openPricing(levelNo: string) {
    const loaded = this.browserPage.waitForResponse(response => response.url().includes(`/levels/${levelNo}/pricing`) && response.request().method() === 'GET', { timeout: 15_000 });
    await this.pricingButton(levelNo).click(); return (await loaded).json();
  }
  async savePricing(levelNo: string) {
    const saved = this.browserPage.waitForResponse(response => response.url().includes(`/levels/${levelNo}/pricing`) && response.request().method() === 'PUT', { timeout: 15_000 });
    await this.button(/确\s*认/u).click(); return (await saved).json();
  }
  async clearPricing(levelNo: string) { await this.percentageInput.fill(''); return this.savePricing(levelNo); }
  async closePricing() { await this.button(/取\s*消/u).click(); }
}

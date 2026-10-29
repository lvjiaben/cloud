import { CatalogEditorTabsPage } from './CatalogEditorTabsPage';
export class CatalogPricePreviewPage extends CatalogEditorTabsPage {
  get priceSummary() { return this.page.getByTestId('catalog-preview-price-summary'); }
  async selectPreviewPlan(label:string) { await this.choose('catalog-option-preview-local_plan',label); }
  async selectPreviewCurrency(code:string) {
    const control=this.page.getByTestId('catalog-preview-currency').getByRole('combobox');
    await control.focus();await control.press('ArrowDown');
    // Ant Design also renders an offscreen accessibility option whose text
    // equals the ISO code; choose the visible item content explicitly.
    await this.page.locator('.ant-select-dropdown:visible .ant-select-item-option-content').filter({hasText:new RegExp(`^${code}$`)}).click();
  }
  async selectPreviewPeriod(label:string) { await this.choose('catalog-preview-period',label); }
  async settlePriceDebounce() { await this.page.waitForTimeout(600); }
  async retryPrice() { await this.page.getByTestId('catalog-preview-price-failed').getByRole('button',{name:'重新计算',exact:true}).click(); }
}

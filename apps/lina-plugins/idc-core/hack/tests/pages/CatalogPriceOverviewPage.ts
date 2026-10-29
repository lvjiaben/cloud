import { expect } from '@host-tests/support/playwright';
import { CatalogEditorTabsPage } from './CatalogEditorTabsPage';

export class CatalogPriceOverviewPage extends CatalogEditorTabsPage {
  get priceTable() { return this.page.getByTestId('catalog-base-price-table'); }
  get priceCopy() { return this.page.getByTestId('catalog-base-price-copy-modal'); }
  get priceCopyDialog() { return this.page.getByRole('dialog').filter({has:this.priceCopy}); }
  priceInput(field:'amount'|'renewal'|'setup', rule:string, currency:string) { return this.priceTable.getByTestId(`catalog-base-price-${field}-${rule}:${currency}`); }
  async openPriceCopy() { await this.page.getByTestId('catalog-base-price-copy').click(); await expect(this.priceCopy).toBeVisible(); }
  get copyChoices() { return this.page.locator('.ant-select-dropdown:visible:not(.ant-slide-up-leave-active)'); }
  async openCopySources() { const control=this.page.getByTestId('catalog-base-price-copy-source').getByRole('combobox'); await control.focus(); await control.press('ArrowDown'); return this.copyChoices; }
  async chooseCopySource(label:string) { await (await this.openCopySources()).getByText(label,{exact:true}).click(); }
  async closeCopyChoices() { await this.priceCopy.getByText('来源价格',{exact:true}).click(); }
  async openCopyTargets() { const control=this.page.getByTestId('catalog-base-price-copy-targets').getByRole('combobox'); await control.focus(); await control.press('ArrowDown'); return this.copyChoices; }
  async chooseCopyTarget(label:string) { await (await this.openCopyTargets()).getByText(label,{exact:true}).click(); await this.closeCopyChoices(); }
  async fillPriceDraft() { await this.priceCopyDialog.getByRole('button',{name:'填入草稿',exact:true}).click(); await expect(this.priceCopy).toBeHidden(); }
  async cancelPriceCopy() { await this.priceCopyDialog.getByRole('button',{name:/^取\s*消$/}).click(); await expect(this.priceCopy).toBeHidden(); }
  async savePriceTable() { await this.page.getByTestId('catalog-base-price-table-save').click(); }
}

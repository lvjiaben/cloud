import { expect } from '@host-tests/support/playwright';
import { MerchantOfferPage } from './MerchantOfferPage';

export class CatalogEditorTabsPage extends MerchantOfferPage {
  async goto() {
    // Hash navigation retains the old document; load installed resource mocks first.
    await this.page.reload({waitUntil:"domcontentloaded"});
    await super.goto();
  }
  get parameterModal() {return this.page.getByTestId('catalog-parameter-modal');}
  get parameterDialog() {return this.page.getByRole('dialog').filter({has:this.parameterModal});}
  async choose(testId:string,label:string) {
    const root=this.page.getByTestId(testId);
    const control=root.and(this.page.getByRole('combobox')).or(root.getByRole('combobox'));
    await control.focus(); await control.press('ArrowDown');
    await this.page.locator('.ant-select-dropdown:visible').getByText(label,{exact:true}).click();
  }
  async tab(key:string) {await this.page.getByTestId(`catalog-editor-tab-${key}`).click();}
  async customParameter() {await this.page.getByTestId('catalog-local-option-add').click();await expect(this.parameterModal).toBeVisible();}
  parameterInput(suffix:string) {return this.parameterModal.getByTestId(new RegExp(`^option_\\d+_${suffix}$`));}
  async chooseParameter(suffix:string,label:string) {
    const control=this.parameterInput(suffix).getByRole('combobox');await control.focus();await control.press('ArrowDown');
    await this.page.locator('.ant-select-dropdown:visible').getByText(label,{exact:true}).click();await this.parameterModal.getByRole('heading',{name:'参数定义',exact:true}).click();
  }
  async cancelParameter() {await this.page.getByTestId('catalog-parameter-cancel').click();await expect(this.parameterModal).toBeHidden();}
  async saveParameter() {await this.parameterDialog.getByTestId('catalog-customer-options-save').click();}
  async addChoice(value:string,name:string) {
    const editor=this.parameterModal.locator('[data-testid$="_choices"]');
    await editor.getByTestId('catalog-option-child-add').click();
    await editor.getByTestId('catalog-option-child-value').fill(value);
    await editor.getByTestId('catalog-option-child-zhCN').fill(name);
  }
  get optionPriceBatch() {return this.page.getByRole('dialog').filter({has:this.page.getByTestId('catalog-option-price-batch')});}
  async batchOptionPrices(targets:string[],amount:string,renewal:string) {
    await this.parameterModal.getByTestId('catalog-option-price-batch-open').click();
    for(const target of targets) await this.choose('catalog-option-price-batch-targets',target);
    await this.optionPriceBatch.getByRole('heading',{name:'批量设置价格',exact:true}).click();
    await this.optionPriceBatch.getByTestId('catalog-option-price-batch-amount').fill(amount);
    await this.optionPriceBatch.getByTestId('catalog-option-price-batch-renewal').fill(renewal);
    await this.optionPriceBatch.getByRole('button',{name:'填入草稿',exact:true}).click();
  }
  async copyDefinition(key:string) {await this.page.getByTestId(`catalog-option-copy-${key}`).click();await expect(this.parameterModal).toBeVisible();}
  async addPrice() {await this.page.getByTestId('catalog-priced-period-add').click();await expect(this.page.getByTestId('catalog-priced-billing-form')).toBeVisible();}
  get agentPriceDialog() {return this.page.getByRole('dialog').filter({has:this.page.getByTestId('sales-management-drawer')});}
  async openAgentPrice() {await this.page.getByTestId('price-add').click();await expect(this.agentPriceDialog).toBeVisible();}
  async saveAgentPrice() {await this.agentPriceDialog.getByRole('button',{name:/^保\s*存$/}).click();}
  async addPriceBand(upper:string) {
    const prices=this.parameterModal.getByTestId('catalog-offer-field-prices');
    await prices.getByTestId('catalog-offer-price-upper').first().fill(upper);
    await prices.getByRole('button',{name:'增加一档',exact:true}).click();
  }
  async clippedPriceActions() {
    return this.parameterModal.locator('.offer-price-actions button').evaluateAll(buttons=>buttons.filter(button=>{
      const rect=button.getBoundingClientRect(),cell=button.closest('td')?.getBoundingClientRect();
      return cell && rect.width>0 && rect.height>0 && (rect.bottom>cell.bottom+1 || rect.top<cell.top-1);
    }).length);
  }
  sortInput(key:string) {return this.page.locator(`input[data-testid="catalog-option-sort-${key}"], [data-testid="catalog-option-sort-${key}"] input`);}
  async sortParameter(key:string,value:string) {await this.sortInput(key).fill(value);await this.sortInput(key).press('Tab');}
  async saveTable() {await this.page.getByTestId('catalog-configuration-table-save').click();}
  async toggleUpgrade(key:string) {await this.page.getByTestId(`catalog-option-upgrade-${key}`).click();}
  async saveBasic() {await this.page.getByTestId('catalog-description-save').click();}
  async saveAutomation() {await this.page.getByTestId('catalog-fulfillment-policy-save').click();}
  async retryConfiguration() {await this.page.getByTestId('catalog-settings-retry').click();}
}

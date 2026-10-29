import { CatalogManagementPage } from './CatalogManagementPage';

export class CatalogWorkspacePage extends CatalogManagementPage {
  get settingsLoading() { return this.page.locator('[data-testid="catalog-settings-loading"]:visible'); }
  get settingsFailed() { return this.page.locator('[data-testid="catalog-settings-failed"]:visible'); }
  get settingsRetry() { return this.page.locator('[data-testid="catalog-settings-retry"]:visible'); }
  get productParameters() { return this.page.getByTestId('catalog-product-parameters-form'); }
  get productCosts() { return this.page.getByTestId('catalog-product-costs'); }
  get descriptionName() { return this.page.getByTestId('catalog-description-name-zh'); }
  async showProductCosts() { const header=this.productCosts.locator('.ant-collapse-header'); if(await header.getAttribute('aria-expanded')!=='true') await header.click(); await this.productCosts.scrollIntoViewIfNeeded(); }
  async showQuantityPrices() {
    await this.page.getByTestId('catalog-quantity-price').scrollIntoViewIfNeeded();
  }
  get copyModal() { return this.page.getByTestId('catalog-product-copy-modal'); }
  async openCopy(no: string) { await this.page.getByTestId(`catalog-product-copy-${no}`).click(); await this.copyModal.waitFor({ state: 'visible' }); }
  async fillCopy(zh: string, en: string, slug: string) {
    await this.page.getByTestId('catalog-copy-name-zh').fill(zh);
    await this.page.getByTestId('catalog-copy-name-en').fill(en);
    await this.page.getByTestId('catalog-copy-slug').fill(slug);
  }
  async confirmCopy() { await this.page.getByRole('dialog').filter({ has: this.copyModal }).getByRole('button', { name: '生成草稿', exact: true }).click(); }

  get sortModal() { return this.page.getByTestId('catalog-sort-modal'); }
  async openProductSort(no: string) {
    await this.page.getByTestId(`catalog-product-more-${no}`).click();
    await this.page.getByTestId(`catalog-product-sort-${no}`).click();
    await this.sortModal.waitFor({state:'visible'});
  }
  async openCategorySort() { await this.page.getByTestId('catalog-category-sort').click(); await this.sortModal.waitFor({state:'visible'}); }
  async saveSort(value: number) {
    await this.page.getByTestId('catalog-sort-value').fill(String(value));
    await this.page.getByRole('dialog').filter({has:this.sortModal}).getByRole('button',{name:/确\s*认|Confirm/}).click();
  }
  async openProductMore(no: string) { await this.page.getByTestId(`catalog-product-more-${no}`).click(); }
  async filterStatus(label: string) {
    await this.page.getByTestId('catalog-search-status').click();
    await this.page.locator('.ant-select-dropdown:visible').getByText(label,{exact:true}).click();
    await this.page.getByRole('button',{name:/搜\s*索|Search/}).click();
  }
  get categoryPanel() { return this.page.getByTestId('catalog-category-panel'); }
  async selectCategory(no: string) {
    const response = this.page.waitForResponse(r => r.url().includes('/catalog/products?') && new URL(r.url()).searchParams.get('categoryNo') === no);
    await this.page.getByTestId(`catalog-category-select-${no}`).click();
    return (await response).json();
  }
  async allProducts() {
    const response = this.page.waitForResponse(r => r.url().includes('/catalog/products?') && !new URL(r.url()).searchParams.get('categoryNo'));
    await this.page.getByTestId('catalog-category-all').click();
    return (await response).json();
  }
  async createCategory(zh: string, en: string) {
    await this.page.getByTestId('catalog-category-create-inline').click();
    await this.categoryCreateModal.waitFor({ state: 'visible' });
    await this.page.getByTestId('catalog-category-create-name-zh').fill(zh);
    await this.page.getByTestId('catalog-category-create-name-en').fill(en);
  }
  async confirmCategory() {
    await this.page.getByRole('dialog').filter({ has: this.categoryCreateModal }).getByRole('button', { name: '创建分类', exact: true }).click();
  }
  async editCategory(zh: string) {
    await this.page.getByTestId('catalog-category-edit-inline').click();
    const modal = this.page.getByTestId('catalog-category-edit-modal');
    await modal.waitFor({ state: 'visible' });
    await modal.getByRole('textbox').first().fill(zh);
    await this.page.getByRole('dialog').filter({ has: modal }).getByRole('button', { name: /确\s*认/ }).click();
    await modal.waitFor({ state: 'hidden' });
  }
  async mutateCategory(action: 'delete' | 'restore') {
    const confirmation = this.page.locator('.ant-popover:visible').getByRole('button', { name: /确\s*定/ });
    if (!(await confirmation.isVisible())) await this.page.getByTestId(`catalog-category-${action}-inline`).click();
    await confirmation.click();
  }
  async showDeletedCategories() {
    await this.page.getByTestId('catalog-category-state').click();
    await this.page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({ hasText: '已删除分类' }).click();
  }
  async closeModal() { await this.page.keyboard.press('Escape'); }
  async addBillingPeriod() {
    await this.selectEditSection('价格与税务');
    await this.page.getByTestId('catalog-billing-add').click();
    await this.page.getByTestId('catalog-billing-modal').waitFor({ state: 'visible' });
  }
  async addTrialPeriod() {
    await this.selectEditSection('价格与税务');
    await this.page.getByTestId('catalog-trial-add').click();
    await this.page.getByTestId('catalog-billing-modal').waitFor({ state: 'visible' });
  }
  async setTrialDuration(duration: string, limit: string) {
    await this.page.getByTestId('catalog-billing-count').fill(duration);
    await this.page.getByTestId('catalog-trial-limit').fill(limit);
  }
  async confirmBillingPeriod() {
    await this.page.getByRole('dialog').filter({ has: this.page.getByTestId('catalog-billing-modal') }).getByRole('button', { name: /确\s*认/ }).click();
  }
  async addBasePrice(ruleNo: string, amount: string, renewal: string) {
    await this.page.getByTestId(`catalog-base-price-add-${ruleNo}`).click();
    await this.page.getByTestId('catalog-base-price-modal').waitFor({ state: 'visible' });
    const input = this.page.getByTestId('catalog-base-price-amount');
    if (await input.isEditable()) await input.fill(amount);
    if (await this.page.getByTestId('catalog-base-price-renewal').count()) await this.page.getByTestId('catalog-base-price-renewal').fill(renewal);
  }
  async selectBillingPeriod(label: string) {
    await this.page.getByTestId('catalog-billing-period').click();
    await this.page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({ hasText: label }).click();
  }
  async confirmBasePrice() {
    await this.page.getByRole('dialog').filter({ has: this.page.getByTestId('catalog-base-price-modal') }).getByRole('button', { name: /确\s*认/ }).click();
  }
  async addTax(rate: string, categoryCode: string) {
    await this.openTax('应税', categoryCode);
    await this.page.getByTestId('catalog-tax-rate').fill(rate);
  }
  async openTax(treatment: string, categoryCode: string) {
    await this.selectEditSection('价格与税务');
    await this.page.getByTestId('catalog-tax-add').click();
    await this.page.getByTestId('catalog-tax-modal').waitFor({ state: 'visible' });
    await this.page.getByTestId('catalog-tax-treatment').click();
    await this.page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({ hasText: treatment }).click();
    await this.page.getByTestId('catalog-tax-category').click();
    const labels: Record<string,string>={cloud_service:'云服务',hosting:'服务器与托管',domain:'域名服务',certificate:'证书服务',other:'其他商品与服务'};
    await this.page.locator('.ant-select-dropdown:visible').getByText(labels[categoryCode] || '自定义分类（高级）',{exact:true}).click();
    if(!labels[categoryCode]) await this.page.getByTestId('catalog-tax-category-custom').fill(categoryCode);
  }
  async setTaxRate(rate: string) { await this.page.getByTestId('catalog-tax-rate').fill(rate); }
  async confirmTax() {
    await this.page.getByRole('dialog').filter({ has: this.page.getByTestId('catalog-tax-modal') }).getByRole('button', { name: /确\s*认/ }).click();
  }
  async openAdvancedConfigurationPrices() {
    const header=this.page.getByTestId('catalog-advanced-configuration-prices').locator('.ant-collapse-header').first();
    if(await header.getAttribute('aria-expanded') !== 'true') await header.click();
  }
  async openConfigurationPrice() {
    await this.selectEditSection('购买配置');
    await this.openAdvancedConfigurationPrices();
    await this.page.getByTestId('catalog-configuration-price-add').click();
    await this.page.getByTestId('catalog-configuration-price-modal').waitFor({ state: 'visible' });
  }
  async setConfigurationPrice(amount: string, renewal: string) {
    await this.page.getByTestId('catalog-configuration-price-amount').fill(amount);
    await this.page.getByTestId('catalog-configuration-price-renewalAmount').fill(renewal);
  }
  async selectConfigurationPriceMode(label: string) {
    await this.page.getByTestId('catalog-configuration-price-mode').click();
    await this.page.locator('.ant-select-dropdown:visible .ant-select-item-option').filter({ hasText: label }).click();
  }
  async addConfigurationPriceBand(minimum: string, amount: string, renewal: string) {
    await this.page.getByTestId('catalog-configuration-price-add-band').click();
    await this.page.getByTestId('catalog-configuration-price-minimum_1').fill(minimum);
    await this.page.getByTestId('catalog-configuration-price-amount_1').fill(amount);
    await this.page.getByTestId('catalog-configuration-price-renewalAmount_1').fill(renewal);
  }
  async removeLastConfigurationPriceBand() {
    await this.page.getByTestId("catalog-configuration-price-remove-band").click();
  }
  async editConfigurationPrice(priceNo: string) {
    await this.openAdvancedConfigurationPrices();
    await this.page.getByTestId(`catalog-configuration-price-edit-${priceNo}`).click();
    await this.page.getByTestId("catalog-configuration-price-modal").waitFor({state:"visible"});
  }
  async deleteConfigurationPrice(priceNo: string) {
    await this.openAdvancedConfigurationPrices();
    await this.page.getByTestId("catalog-configuration-price-modal").waitFor({state:"hidden"});
    await this.page.getByTestId(`catalog-configuration-price-delete-${priceNo}`).click();
    await this.page.locator(".ant-popover:visible").getByRole("button",{name:/确\s*定/}).click();
  }
  async openFieldPrice(fieldKey: string) { await this.openAdvancedConfigurationPrices(); await this.page.getByTestId(`catalog-field-price-${fieldKey}`).click(); await this.page.getByTestId('catalog-configuration-price-modal').waitFor({state:'visible'}); }
  async retryConfiguration() {await this.page.getByTestId('catalog-configuration-retry').click();}
  async openProductAgentPrice() {
    await this.selectEditSection('价格与税务'); await this.page.getByTestId('catalog-edit-agent-prices').click();
    await this.page.getByTestId('price-add').click(); await this.page.getByTestId('sales-management-drawer').waitFor({state:'visible'});
  }
  async confirmAgentPrice() {await this.page.getByRole('dialog').filter({has:this.page.getByTestId('sales-management-drawer')}).last().getByRole('button',{name:/确\s*认/}).click();}
  async previewEditorPublication() { await this.page.getByTestId("catalog-editor-publication-preview").click(); await this.publicationPreview.waitFor({state:"visible"}); }
  async confirmConfigurationPrice() {
    await this.page.getByRole('dialog').filter({ has: this.page.getByTestId('catalog-configuration-price-modal') }).getByRole('button', { name: /确\s*认/ }).click();
  }
  async retryCategories() { await this.categoryPanel.getByRole('button', { name: /重\s*试/ }).click(); }
}

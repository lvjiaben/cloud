import { CatalogWorkspacePage } from './CatalogWorkspacePage';
export class CustomerOptionsPage extends CatalogWorkspacePage {
  get optionsForm() { return this.page.getByTestId('catalog-customer-options-form'); }
  async openOptions() {
    await this.selectEditSection('购买配置');
    await this.optionsForm.waitFor({state:'visible'});
  }
  async addParameter(label: string) {
    await this.page.getByTestId('catalog-customer-option-parameter').click();
    await this.page.locator('.ant-select-dropdown:visible').getByText(label,{exact:true}).click();
    await this.page.getByTestId('catalog-customer-option-add').click();
  }
  async selectCPUControl(label:string) {
    await this.page.getByTestId('option_0_control').click();
    await this.page.locator('.ant-select-dropdown:visible').getByText(label,{exact:true}).click();
  }
  async setCPUChoices(step:string,numbers:string) {
    await this.page.getByTestId('option_0_step').fill(step);
    await this.page.getByTestId('option_0_numbers').fill(numbers);
  }
  async enableCPU() { await this.page.getByTestId('option_0_enabled').click(); }
  async setCPUBounds(minimum: string, maximum: string) {
    await this.page.getByTestId('option_0_minimum').fill(minimum);
    await this.page.getByTestId('option_0_maximum').fill(maximum);
  }
  async customizeCPU(zh: string, en: string, order: string) {
    await this.page.getByTestId('option_0_name_zhCN').fill(zh);
    await this.page.getByTestId('option_0_name_enUS').fill(en);
    await this.page.getByTestId('option_0_order').fill(order);
  }
  async confirmOptions() { await this.page.getByTestId('catalog-customer-options-save').click(); }
  async reopenOptions(productNo='product-manual-service') { await this.closeProductDetail();await this.openProductDetail(productNo);await this.openOptions(); }
}

import { CatalogWorkspacePage } from './CatalogWorkspacePage';
export class CustomerOptionsPage extends CatalogWorkspacePage {
  get optionsModal() { return this.page.getByRole('dialog').filter({ has: this.page.getByRole('heading', { name: '客户可选配置', exact: true }) }); }
  async openOptions() {
    await this.selectEditSection('购买配置');
    await this.page.getByTestId('catalog-customer-options-edit').click();
    await this.optionsModal.waitFor({state:'visible'});
  }
  async enableCPU() { await this.page.getByTestId('option_0_enabled').click(); }
  async setCPUBounds(minimum: string, maximum: string) {
    await this.page.getByTestId('option_0_minimum').fill(minimum);
    await this.page.getByTestId('option_0_maximum').fill(maximum);
  }
  async confirmOptions() { await this.optionsModal.getByRole('button',{name:/确\s*认/}).click(); }
}

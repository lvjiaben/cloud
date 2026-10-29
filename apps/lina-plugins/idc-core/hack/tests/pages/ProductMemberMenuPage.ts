import { workspacePath } from '@host-tests/fixtures/config';
import { CatalogWorkspacePage } from './CatalogWorkspacePage';

export class ProductMemberMenuPage extends CatalogWorkspacePage {
  async goto() { await this.page.goto(workspacePath('/idc/catalog'), { waitUntil: 'domcontentloaded' }); await this.pageRoot.waitFor({ state: 'visible' }); }
  get createMenu() { return this.productCreateModal.getByTestId('catalog-product-create-member-menu'); }
  get editMenu() { return this.page.getByTestId('catalog-description-member-menu'); }
  get menuOptions() { return this.page.locator('.ant-select-dropdown:visible:not(.ant-slide-up-leave-active)'); }
  get descriptionEditor() { return this.page.getByTestId('catalog-description-editor'); }
  async selectMenu(name: string, editing = false) {
    await (editing ? this.editMenu : this.createMenu).click();
    await this.menuOptions.getByText(name, { exact: true }).click();
  }
  async selectProductCategory() {
    await this.productCreateModal.getByTestId('catalog-product-create-category').click();
    await this.page.locator('.ant-select-dropdown:visible:not(.ant-slide-up-leave-active)').getByText('证书服务', { exact: true }).click();
  }
  async fillNewProduct() {
    for (const [field, value] of [['slug', 'member-menu-test'], ['name-zh', '导航商品'], ['name-en', 'Menu product'], ['summary-zh', '导航归属测试'], ['summary-en', 'Menu assignment test']]) {
      await this.productCreateModal.getByTestId('catalog-product-create-' + field).fill(value);
    }
  }
  async createProduct() { await this.page.getByRole('dialog').filter({ has: this.productCreateModal }).getByRole('button', { name: '创建商品', exact: true }).click(); }
  async editDescription() { await this.selectEditSection('基本资料'); }
  async saveDescription() { await this.page.getByTestId('catalog-description-save').click(); }
}

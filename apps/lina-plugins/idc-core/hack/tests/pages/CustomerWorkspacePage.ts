import { CustomerManagementPage } from "./CustomerManagementPage";
export class CustomerWorkspacePage extends CustomerManagementPage {
  async showProductStatistics() {
    await this.page.getByTestId("customer-product-statistics").scrollIntoViewIfNeeded();
  }
  async viewProductsFromOverview() {
    await this.page
      .getByTestId("customer-product-statistics")
      .getByRole("button", { name: "查看产品", exact: true })
      .click();
  }
  get products() {
    return this.page.getByTestId("customer-products-panel");
  }
  get api() {
    return this.page.getByTestId("customer-api-panel");
  }
  get editor() {
    return this.page.getByTestId("customer-server-edit");
  }
  async closeOverview() {
    await this.page.keyboard.press("Escape");
    await this.overviewDrawer.waitFor({ state: "hidden" });
  }
  async tab(name: string) {
    await this.overviewDrawer.getByRole("tab", { name, exact: true }).click();
  }
  async nextProducts() {
    await this.products.locator(".vxe-pager--next-btn").click();
  }
  async nextCalls() {
    await this.api.locator(".vxe-pager--next-btn").click();
  }
  async retryProducts() {
    await this.products.getByRole("button", { name: /^重\s*试$/u }).click();
  }
  async resetSecret(confirm = true) {
    await this.api.getByTestId("customer-api-reset").click();
    await this.page
      .locator(".ant-modal-confirm:visible")
      .getByRole("button", { name: confirm ? /确\s*(定|认)/u : /取\s*消/u })
      .click();
  }
  async scrollProductsRight() {
    await this.products
      .locator(".vxe-table--main-wrapper .vxe-table--body-wrapper")
      .evaluate((el) => {
        el.scrollLeft = el.scrollWidth;
      });
  }
  async retrySecretConfirm() {
    await this.page
      .locator(".ant-modal-confirm:visible")
      .getByRole("button", { name: /确\s*(定|认)/u })
      .click();
  }
  async editServer() {
    await this.products.getByTestId("customer-product-edit-SVC_SERVER").click();
    await this.editor.waitFor({ state: "visible" });
  }
  async saveNote(note: string) {
    await this.editor.getByRole("textbox").last().fill(note);
    await this.editor.getByTestId("customer-server-save-note").click();
  }
  async previewChange() {
    await this.editor.getByTestId("customer-server-preview").click();
  }
  async confirmChange(confirm = true) {
    await this.editor.getByTestId("customer-server-confirm-change").click();
    await this.page
      .locator(".ant-modal-confirm:visible")
      .getByRole("button", { name: confirm ? /确\s*(定|认)/u : /取\s*消/u })
      .click();
  }
}

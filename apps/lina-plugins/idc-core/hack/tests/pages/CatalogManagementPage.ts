import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class CatalogManagementPage {
  readonly body: Locator;
  readonly categoryCreateModal: Locator;
  readonly detailDrawer: Locator;
  readonly fulfillmentPolicyModal: Locator;
  readonly fulfillmentPolicySummary: Locator;
  readonly fulfillmentPolicyStrategy: Locator;
  readonly nextButton: Locator;
  readonly page: Page;
  readonly pageRoot: Locator;
  readonly previousButton: Locator;
  readonly productCreateModal: Locator;
  readonly publicationPreview: Locator;

  constructor(page: Page) {
    this.page = page;
    this.body = page.locator("body");
    this.pageRoot = page.getByTestId("catalog-management-page");
    this.categoryCreateModal = page.getByTestId(
      "catalog-category-create-modal",
    );
    this.detailDrawer = page.getByTestId("catalog-product-detail-drawer");
    this.fulfillmentPolicyModal = page.getByTestId(
      "catalog-fulfillment-policy-modal",
    );
    this.fulfillmentPolicySummary = page.getByTestId(
      "catalog-fulfillment-policy-summary",
    );
    this.fulfillmentPolicyStrategy = page.getByTestId(
      "catalog-fulfillment-policy-strategy",
    );
    this.nextButton = this.pageRoot.locator(".vxe-pager--next-btn");
    this.previousButton = this.pageRoot.locator(".vxe-pager--prev-btn");
    this.productCreateModal = page.getByTestId("catalog-product-create-modal");
    this.publicationPreview = page.getByTestId("catalog-publication-preview");
  }

  async saveAddons() { await this.page.getByTestId('catalog-addons-save').click(); }

  async openCategoryParentChoices() {
    await this.page.getByTestId('catalog-category-create-inline').click();
    await this.page.getByTestId('catalog-category-create-parent').click();
    return this.page.locator('.ant-select-dropdown:visible');
  }

  async writeAddons(productNo: string, productNos: string[], version: number) {
    return this.page.evaluate(async ({ productNo, productNos, version }) => {
      const key = Object.keys(localStorage).find(key => key.endsWith('core-access'));
      const token = key ? JSON.parse(localStorage.getItem(key)!).accessToken : '';
      const response = await fetch('/x/idc-core/api/v1/admin/catalog/products/' + productNo + '/addons', {
        method: 'PUT', headers: { 'Content-Type': 'application/json', Authorization: 'Bearer ' + token },
        body: JSON.stringify({ productNos, version }),
      });
      return response.json();
    }, { productNo, productNos, version });
  }

  async goto() {
    // Finish an earlier root navigation before hash routing, so its initial
    // permission redirect cannot overwrite the requested catalog page.
    await waitForRouteReady(this.page,15_000);
    await this.page.goto(workspacePath("/idc/catalog"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.page.getByText(/加载菜单中|Loading menu/iu).waitFor({
      state: "hidden",
      timeout: 15_000,
    });
    await this.waitForDataSettled();
  }

  async openMerchantConfiguration(showAdvanced = true) {
    await this.selectEditSection("交付设置");
    await this.merchantConfigurationForm().waitFor({state:"visible"});
    const advanced=this.page.getByTestId('catalog-product-advanced-toggle');
    if(showAdvanced && await advanced.isVisible() && await advanced.getAttribute('aria-expanded') !== 'true') await advanced.click();
  }

  merchantConfigurationForm() {
    return this.page.getByTestId("catalog-product-parameters-form");
  }

  async expandCustomerChoices(fieldKey: string) {
    await this.page.getByTestId(`catalog-customer-choices-${fieldKey}`).getByRole("button").click();
  }

  async toggleAdvancedConfiguration() {
    await this.page.getByTestId("catalog-product-advanced-toggle").click();
  }

  async selectMerchantField(label: string, choice: string) {
    const dialog=this.merchantConfigurationForm();
    await dialog.getByLabel(label,{exact:true}).click();
    await this.page.locator('.ant-select-dropdown:visible').getByText(choice,{exact:true}).click();
  }

  async fillMerchantField(label: string, value: string) {
    await this.merchantConfigurationForm().getByLabel(label, { exact: true }).fill(value);
  }

  async selectMerchantCharacter(label: string) {
    const dialog = this.merchantConfigurationForm();
    await dialog.getByLabel("密码字符规则", { exact: true }).locator("xpath=../..").click();
    await this.page.locator(".ant-select-dropdown:visible").getByText(label, { exact: true }).click();
    await dialog.getByText("主机名前缀", { exact: true }).click();
  }

  async saveMerchantConfiguration() {
    await this.page.getByTestId("catalog-product-parameters-save").click();
  }

  productRow(productNo: string) {
    return this.pageRoot
      .getByTestId(`catalog-product-${productNo}`)
      .locator("xpath=ancestor::tr");
  }

  async searchNamePrefix(prefix: string) {
    await this.page.getByTestId("catalog-search-name").fill(prefix);
    await this.page.getByRole("button", { name: /搜\s*索|Search/iu }).click();
    await this.waitForDataSettled();
  }

  async openProductDetail(productNo: string) {
    await this.page.getByTestId(`catalog-product-detail-${productNo}`).click();
    await this.detailDrawer.waitFor({ state: "visible" });
    await this.detailDrawer
      .locator(".bg-overlay-content:not(.invisible)")
      .waitFor({
        state: "detached",
        timeout: 15_000,
      });
  }

  async openProductCreate(advanced = true) {
    await this.page.getByTestId("catalog-product-create-open").click();
    await this.productCreateModal.waitFor({ state: "visible" });
    if(advanced) await this.productCreateModal.getByTestId('catalog-create-advanced').click();
  }

  fulfillmentBindingModal() {
    return this.page.getByTestId("catalog-fulfillment-binding-modal");
  }

  async openFulfillmentBinding() {
    await this.selectEditSection("交付设置");
    await this.page.getByTestId("catalog-fulfillment-binding-open").click();
    await this.fulfillmentBindingModal().waitFor({ state: "visible" });
  }

  async selectFulfillmentBindingOption(kind: "provider" | "interface", label: string | RegExp) {
    await this.page.getByTestId(`catalog-fulfillment-${kind}`).click();
    await this.page.locator(".ant-select-dropdown:visible").getByText(label, { exact: true }).click();
  }

  async confirmFulfillmentBinding() {
    await this.page.getByRole("dialog").filter({ has: this.fulfillmentBindingModal() }).getByRole("button", { name: "确认绑定", exact: true }).click();
  }

  async openFulfillmentPolicy() {
    await this.selectEditSection(/交付设置|Delivery settings/u);
    await this.fulfillmentPolicyModal.waitFor({ state: "visible" });
  }

  fulfillmentPolicyConfirm() {
    return this.page.getByTestId('catalog-fulfillment-policy-save');
  }

  async openFulfillmentPolicyStrategyOptions() {
    await this.fulfillmentPolicyStrategy.click();
    const dropdown = this.page.locator(".ant-select-dropdown:visible");
    await dropdown.waitFor({ state: "visible" });
    return dropdown;
  }

  fulfillmentPolicyStrategyOption(label: string) {
    return this.page
      .locator(".ant-select-dropdown:visible")
      .locator(".ant-select-item-option")
      .filter({ hasText: label });
  }

  async selectFulfillmentPolicyStrategy(label: string) {
    await this.openFulfillmentPolicyStrategyOptions();
    await this.fulfillmentPolicyStrategyOption(label).click();
  }

  async closeFulfillmentPolicyStrategyOptions() {
    // Close only the Select popup. Sending Escape at page level also reaches
    // the parent modal and can close the entire fulfillment form.
    await this.fulfillmentPolicyModal.click({ position: { x: 12, y: 12 } });
    await this.page
      .locator(".ant-select-dropdown:visible")
      .waitFor({ state: "hidden" });
  }

  async closeProductDetail() {
    await this.page.getByTestId('catalog-editor-back').click();
    await this.detailDrawer.waitFor({ state: "hidden" });
  }

  fulfillmentPolicyError(message: string) {
    return this.page
      .locator(".ant-message-notice-content")
      .filter({ hasText: message });
  }

  fulfillmentPolicyNotice(message: string) {
    return this.page
      .locator(".ant-message-notice-content")
      .filter({ hasText: message });
  }

  async waitForFulfillmentPolicyNoticeHidden(message: string) {
    await this.fulfillmentPolicyNotice(message).waitFor({ state: "hidden" });
  }

  async revealFulfillmentPolicySummary() {
    await this.selectEditSection(/交付设置|Delivery settings/u);
    await this.fulfillmentPolicySummary.scrollIntoViewIfNeeded();
  }

  async selectEditSection(label: string | RegExp) {
    const aliases: Array<[string,string,string]> = [['基本资料','Product details','basic'],['交付设置','Delivery settings','fulfillment'],['购买配置','Purchase configuration','configuration'],['价格与税务','Pricing and tax','pricing'],['套餐售价','Package price','pricing'],['商品配置','Product configuration','configuration'],['代理价格','Agent prices','agents'],['自动开通','Automation','fulfillment'],['附加商品','Add-on products','basic'],['上架检查','Publication review','publication']];
    const section = aliases.find(([zh,en]) => typeof label === 'string' ? label === zh || label === en : label.test(zh) || label.test(en))?.[2];
    if(!section) throw new Error(`Unknown product edit section: ${String(label)}`);
    await this.page.getByTestId(`catalog-editor-tab-${section}`).click();
  }

  async openPublicationPreview(productNo: string) {
    await this.page.getByTestId(`catalog-product-publish-${productNo}`).click();
    await this.publicationPreview.waitFor({ state: "visible" });
  }

  async screenshot(description: string) {
    await this.waitForDataSettled();
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(fileURLToPath(new URL("../../../../../../temp/", import.meta.url)), day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ animations: "disabled", fullPage: false, path });
    return path;
  }

  async waitForDataSettled() {
    await this.pageRoot.locator(".bg-overlay-content:not(.invisible)").waitFor({
      state: "detached",
      timeout: 15_000,
    });
  }
}

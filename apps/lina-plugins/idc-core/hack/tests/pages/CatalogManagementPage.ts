import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

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

  async goto() {
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

  async openProductCreate() {
    await this.page.getByTestId("catalog-product-create-open").click();
    await this.productCreateModal.waitFor({ state: "visible" });
  }

  async openFulfillmentPolicy() {
    await this.detailDrawer
      .getByRole("button", {
        name: /设置履约方式|Modify fulfillment mode/iu,
      })
      .click();
    await this.fulfillmentPolicyModal.waitFor({ state: "visible" });
    const dialog = this.page
      .getByRole("dialog")
      .filter({ has: this.fulfillmentPolicyModal });
    await dialog.evaluate(async (element) => {
      await Promise.all(
        element
          .getAnimations({ subtree: true })
          .map((animation) => animation.finished.catch(() => undefined)),
      );
    });
  }

  fulfillmentPolicyConfirm() {
    return this.page
      .getByRole("dialog")
      .filter({ has: this.fulfillmentPolicyModal })
      .getByRole("button", {
        name: /保存履约方式|Save fulfillment mode/iu,
      });
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
    await this.page.keyboard.press("Escape");
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
    await this.fulfillmentPolicySummary.scrollIntoViewIfNeeded();
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
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ fullPage: false, path });
    return path;
  }

  async waitForDataSettled() {
    await this.pageRoot.locator(".bg-overlay-content:not(.invisible)").waitFor({
      state: "detached",
      timeout: 15_000,
    });
  }
}

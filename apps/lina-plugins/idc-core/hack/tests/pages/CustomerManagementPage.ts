import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";

export class CustomerManagementPage {
  readonly nextButton: Locator;
  readonly overviewDrawer: Locator;
  readonly page: Page;
  readonly pageIndicator: Locator;
  readonly pageRoot: Locator;
  readonly previousButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.overviewDrawer = page.getByTestId("customer-overview-drawer");
    this.pageRoot = page.getByTestId("customer-management-page");
    this.pageIndicator = this.pageRoot.locator(".vxe-pager--num-btn.is--active");
    this.previousButton = this.pageRoot.locator(".vxe-pager--prev-btn");
    this.nextButton = this.pageRoot.locator(".vxe-pager--next-btn");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/customers"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible", timeout:15000 });
  }

  customerRow(customerNo: string) {
    return this.pageRoot.locator(
      `.vxe-table--main-wrapper tr.vxe-body--row[rowid="${customerNo}"]`,
    );
  }

  async searchNamePrefix(prefix: string) {
    await this.page.getByTestId("customer-search-name").fill(prefix);
    await this.page.getByRole("button", { name: /搜\s*索/u }).click();
  }

  async selectSegment(kind: "group" | "tag", name: string) {
    await this.page.getByTestId(`customer-search-${kind}`).click();
    const option = this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(name, { exact: true });
    await option.waitFor({ state: "visible" });
    await option.click();
    await this.page.getByRole("button", { name: /搜\s*索/u }).click();
  }

  async nextPage() {
    await this.nextButton.click();
  }

  async previousPage() {
    await this.previousButton.click();
  }

  async openOverview(customerNo: string) {
    await this.page
      .locator(`[data-testid="customer-overview-${customerNo}"]:visible`)
      .click();
    await this.overviewDrawer.waitFor({
      state: "visible",
    });
  }

  async openOverviewByPublicId(publicId: string) {
    const rows = this.pageRoot.locator(
      ".vxe-table--main-wrapper .vxe-table--body-wrapper tr.vxe-body--row",
    );
    await rows.first().waitFor({ state: "visible" });
    const rowIndex = await rows.evaluateAll(
      (elements, expected) =>
        elements.findIndex((element) =>
          element.textContent?.includes(String(expected)),
        ),
      publicId,
    );
    if (rowIndex < 0) {
      throw new Error(
        `customer ${publicId} is not visible in the current page`,
      );
    }
    await this.pageRoot
      .locator('[data-testid^="customer-overview-"]:visible')
      .nth(rowIndex)
      .click();
    await this.overviewDrawer.waitFor({ state: "visible" });
  }

  async reopenStatusConfirm() {
    for(let index=0;index<2;index++) {
      await this.overviewDrawer.getByTestId("customer-ban-toggle").click();
      await this.page.locator(".ant-modal-confirm:visible").getByRole("button",{name:/取\s*消|Cancel/u}).click();
    }
  }
  async reopenActivityDialog(label: string) {
    for(let index=0;index<2;index++) {
      await this.overviewDrawer.getByRole("button",{name:label,exact:true}).click();
      await this.page.getByRole("dialog").last().getByRole("button",{name:/取\s*消|Cancel/u}).click();
    }
  }
  async addNote(content: string) {
    await this.overviewDrawer.getByRole("button",{name:"添加备注",exact:true}).click();
    await this.page.getByTestId("customer-note-content").fill(content);
    const dialog = this.page.getByRole("dialog").last();
    await dialog.getByRole("button",{name:/确\s*(定|认)/u}).click();
    await this.page.getByTestId("customer-note-content").waitFor({state:"hidden"});
  }
  async copyCustomerNumber() {
    await this.page.context().grantPermissions(["clipboard-read","clipboard-write"]);
    await this.overviewDrawer.getByTestId("customer-id-copy").click();
    return this.page.evaluate(()=>navigator.clipboard.readText());
  }

  async startAssistance() {
    const popupPromise = this.page.context().waitForEvent("page");
    const responsePromise = this.page.waitForResponse(response => response.url().includes("/assistance-sessions") && response.request().method() === "POST");
    await this.overviewDrawer.getByTestId("customer-assistance-open").click();
    const response = await responsePromise;
    if (!response.ok()) throw new Error(`customer assistance request failed: ${response.status()}`);
    return popupPromise;
  }

  async selectCustomer(customerNo: string) {
    await this.customerRow(customerNo)
      .locator("td.col--checkbox .vxe-cell--checkbox")
      .click();
  }

  async openTagBatch() {
    await this.page.getByTestId("customer-tag-batch-open").click();
    await this.page.getByRole("dialog").waitFor({ state: "visible" });
  }
  async enterBatchTag(name: string) {
    const dialog = this.page.getByRole("dialog");
    const input = dialog.getByTestId("customer-tag-batch-tags").locator("input");
    await input.fill(name);await input.press("Enter");
    await dialog.getByTestId("customer-tag-batch-summary").click();
  }
  async confirmTagBatch() { await this.page.getByRole("dialog").getByRole("button",{name:/确\s*认/u}).click(); }
  async cancelTagBatch() { await this.page.getByRole("dialog").getByRole("button",{name:/取\s*消/u}).click(); }
  async chooseExistingDetailTag(name: string) {
    await this.overviewDrawer.getByTestId("customer-tag-input").click();
    await this.page.locator(".ant-select-dropdown:visible").getByText(name,{exact:true}).click();
  }

  async openDirectCustomer() {
    await this.page.getByTestId("customer-direct-open").click();
    await this.page.getByTestId("customer-direct-email").waitFor({state:"visible"});
  }

  async openPasswordReset(customerNo: string) {
    await this.page.locator(`[data-testid="customer-password-reset-${customerNo}"]:visible`).click();
    await this.page.locator(".ant-modal-confirm:visible").waitFor({state:"visible"});
  }

  async createDirectCustomer(input: {email?: string; phone?: string}) {
    if(input.email) await this.page.getByTestId("customer-direct-email").fill(input.email);
    if(input.phone) await this.page.getByTestId("customer-direct-phone").fill(input.phone);
    await this.page.getByRole("dialog").getByRole("button",{name:/^添\s*加$/u}).click();
  }

  async confirmPasswordReset() {
    await this.page.locator(".ant-modal-confirm:visible").getByRole("button",{name:/确\s*(定|认)|OK/u}).click();
  }

  async closeGeneratedPassword() {
    await this.page.locator(".ant-modal-confirm:visible").getByRole("button",{name:/^完\s*成$|^Done$/u}).click();
  }

  async openDetailTab(name: string) {await this.overviewDrawer.getByRole("tab",{name,exact:true}).click();}
  async addGroup(name: string, kind: "group" | "tag" = "group") {
    if (kind === "tag") {
      const input = this.overviewDrawer.getByTestId("customer-tag-input").locator("input");
      await input.fill(name);await input.press("Enter");
      return;
    }
    await this.overviewDrawer.getByTestId(`customer-${kind}-add`).click();
    await this.page.getByTestId("customer-segment-name").fill(name);
    await this.page.getByRole("dialog").last().getByRole("button",{name:/确\s*(定|认)/u}).click();
  }
  async removeSegment(name: string) {
    await this.overviewDrawer.locator(".ant-tag").filter({hasText:name}).locator(".ant-tag-close-icon").click();
    await this.page.locator(".ant-modal-confirm:visible").getByRole("button",{name:/确\s*(定|认)/u}).click();
  }
  async toggleAccount() {
    await this.overviewDrawer.getByTestId("customer-ban-toggle").click();
    await this.page.locator(".ant-modal-confirm:visible").getByRole("button",{name:/确\s*(定|认)/u}).click();
  }

  async assertToolbarAlignment() {
    const add = this.pageRoot.getByTestId("customer-direct-open");
    const batch = this.pageRoot.getByTestId("customer-tag-batch-open");
    const toolbar = this.pageRoot.locator(".vxe-toolbar");
    const [a,b,t] = await Promise.all([add.boundingBox(),batch.boundingBox(),toolbar.boundingBox()]);
    if (!a || !b || !t || Math.abs(a.y-b.y)>1 || Math.abs(a.y+a.height/2-(t.y+t.height/2))>2 || a.x>b.x) throw new Error("Customer toolbar actions are misaligned");
  }
  async updateVerification(action: "approve" | "clear", confirm=true) {
    await this.overviewDrawer.getByTestId(`customer-verification-${action}`).click();
    const dialog=this.page.locator(".ant-modal-confirm:visible");
    await dialog.waitFor({state:"visible"});
    if (await dialog.locator("input, textarea").count()) throw new Error("Manual verification unexpectedly requires a form");
    await dialog.getByRole("button",{name:confirm ? /确\s*(定|认)/u : /取\s*消/u}).click();
    await dialog.waitFor({state:"hidden"});
  }
  async screenshot(description: string) {
    await this.page.getByText("加载菜单中...").waitFor({ state: "hidden" });
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ animations: "disabled", fullPage: false, path });
    return path;
  }
}

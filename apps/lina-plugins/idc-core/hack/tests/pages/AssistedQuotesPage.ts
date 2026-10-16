import { mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

const repositoryRoot = fileURLToPath(
  new URL("../../../../../../", import.meta.url),
);

export class AssistedQuotesPage {
  readonly page: Page;
  readonly pageRoot: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("customer-management-page");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/customers"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
  }

  row(requestNo: string) {
    return this.pageRoot.locator(`tr:has-text("${requestNo}")`);
  }

  async openDetail(requestNo: string) {
    await this.pageRoot.getByTestId(`assisted-quote-detail-${requestNo}`).click();
    const drawer = this.page.getByRole("dialog");
    await drawer.waitFor({ state: "visible" });
  }

  get createDrawer() { return this.page.getByTestId("assisted-create-drawer"); }

  async checkDeadlinePicker() {
    // Ant Design's date picker does not forward arbitrary data attributes.
    const input = this.createDrawer.locator('.ant-picker input');
    await input.click();
    await this.page.locator('.ant-picker-dropdown:visible .ant-picker-date-panel').waitFor({ state: 'visible' });
    await this.page.locator('.ant-picker-dropdown:visible .ant-picker-time-panel').waitFor({ state: 'visible' });
    await this.page.locator('.ant-picker-dropdown:visible .ant-picker-ok button').click();
    await this.page.locator('.ant-picker-dropdown:visible').waitFor({ state: 'hidden' });
  }

  async openCreate(customerNo = "CUS_ASSISTED_0001") {
    this.page.setDefaultTimeout(10_000);
    await this.page.getByTestId(`customer-assist-${customerNo}`).click();
    await this.createDrawer.waitFor({ state: "visible" });
    await this.page.waitForFunction(() => {
      const drawer = document.querySelector('[data-testid="assisted-create-drawer"]');
      return drawer && drawer.getBoundingClientRect().right <= window.innerWidth + 1;
    });
  }

  async select(testId: string, label: string) {
    const select = this.page.getByTestId(testId);
    if (await select.getByRole("combobox").getAttribute("aria-expanded") !== "true") await select.click();
    await this.page.locator(".ant-select-dropdown:visible .ant-select-item-option-content").getByText(label, { exact: true }).click({ timeout: 10_000 });
  }

  async fill(testId: string, value: string) {
    const field = this.page.getByTestId(testId);
    const tag = await field.evaluate(element => element.tagName.toLowerCase());
    await (tag === "input" || tag === "textarea" ? field : field.locator("input, textarea").first()).fill(value);
  }

  async configureProduct(english = false) {
    await this.select("assisted-item-product-0", english ? "High-memory cloud server" : "高内存云服务器");
    await this.createDrawer.getByLabel(english ? "Host name" : "主机名称", { exact: true }).fill("assisted-test-server");
    await this.page.getByTestId("assisted-item-price-0").waitFor({ state: "visible" });
  }

  async addProduct() {await this.page.getByTestId("assisted-create-add-item").click();}

  async fillCreate(english = false) {
    await this.select("assisted-create-currency", "USD");
    await this.fill("assisted-create-manualDiscount", "12.34");
    await this.fill("assisted-create-reason", "客户需要协助选购及优惠");
    await this.configureProduct(english);
    await this.fill("assisted-item-quantity-0", "2");
    await this.fill("assisted-item-cycles-0", "3");
  }

  async submitCreate(english = false) {
    await this.page.getByRole("button", { name: english ? "Create customer order" : "生成客户订单", exact: true }).click();
  }

  async screenshot(description: string) {
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = path.join(repositoryRoot, "temp", day);
    await mkdir(directory, { recursive: true });
    const screenshotPath = path.join(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ animations: "disabled", fullPage: false, path: screenshotPath });
    return screenshotPath;
  }
}

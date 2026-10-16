import type { Page } from "@host-tests/support/playwright";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

export class OperationsDataPage {
  constructor(readonly page: Page) {}
  root() {
    return this.page.getByTestId("operations-data");
  }
  period(key: string) {
    return this.root().getByTestId(`period-${key}`);
  }
  async goto() {
    await this.page.goto("/admin/idc/operations/workbench", {
      waitUntil: "domcontentloaded",
    });
  }
  async screenshot(name: string) {
    const directory = resolve("../../temp/20261010");
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({
      path: resolve(directory, `${Date.now()}-operations-${name}.png`),
      fullPage: false,
    });
  }
  async income(key: string) {
    await this.period(key).locator(".income-link").click();
  }
  async orders(key: string) {
    await this.period(key).locator(".orders-link").click();
  }
  async metric(name: string) {
    await this.root()
      .getByRole("button", { name: new RegExp(name) })
      .click();
  }
  async selectCurrency(currency: string) {
    await this.root().locator(".currency-select").click();
    await this.page
      .locator(".ant-select-dropdown:visible")
      .locator(".ant-select-item-option-content")
      .getByText(currency, { exact: true })
      .click();
  }
  async refresh() {
    await this.root()
      .getByRole("button", { name: /^刷\s*新$/u })
      .click();
  }
  async chartIncome() {
    const chart = this.root().getByTestId("operations-trend").locator("canvas");
    await chart.scrollIntoViewIfNeeded();
    const box = await chart.boundingBox();
    if (!box) throw new Error("Trading chart is unavailable");
    await chart.click({
      position: { x: 44 + ((box.width - 76) * 9.5) / 31, y: 200 },
    });
  }
  async chartProduct() {
    const chart = this.root()
      .getByTestId("operations-products")
      .locator("canvas");
    await chart.scrollIntoViewIfNeeded();
    const box = await chart.boundingBox();
    if (!box) throw new Error("Product chart is unavailable");
    await chart.click({ position: { x: box.width * 0.7, y: 142 } });
  }
  async retry() {
    await this.root()
      .getByRole("button", { name: /^重\s*试$/u })
      .click();
  }
}

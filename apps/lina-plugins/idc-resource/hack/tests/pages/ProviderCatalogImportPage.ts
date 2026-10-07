import { expect } from "@host-tests/support/playwright";
import type { Page } from "@host-tests/support/playwright";

/** Real Vben drawer interactions shared by both catalog delivery surfaces. */
export class ProviderCatalogImportPage {
  constructor(readonly page: Page) {}
  get entry() { return this.page.getByTestId("provider-catalog").locator('[data-action="provider-catalog-import"]'); }
  get dialog() { return this.page.getByRole("dialog", { name: /^(导入商品草稿|Import product draft)/u }); }
  get result() { return this.dialog.getByTestId("provider-import-result"); }
  get confirm() { return this.dialog.getByRole("button", { name: /^(确\s*认|确\s*定|Confirm)$/u }); }
  get removePrice() { return this.dialog.getByRole("button", { name: /^(移除最后一条售价|Remove last selling price)$/u }); }
  input(id: string) { return this.dialog.locator(`input[data-testid="${id}"], [data-testid="${id}"] input`); }
  async open() {
    await this.entry.click();
    await expect(this.dialog).toBeVisible();
    const titleId = await this.dialog.getAttribute("aria-labelledby");
    expect(titleId).toBeTruthy();
    // Separate Vue apps must not label both nested drawers with the parent's title.
    expect(await this.page.locator("[id]").evaluateAll((nodes, id) => nodes.filter(node => node.id === id).length, titleId)).toBe(1);
    await expect(this.dialog.getByRole("heading", { name: /^(导入商品草稿|Import product draft)$/u })).toHaveAttribute("id", titleId!);
    const labels = await this.page.getByRole("dialog").evaluateAll(nodes => nodes.map(node => node.getAttribute("aria-labelledby")));
    expect(new Set(labels).size).toBe(labels.length);
    await expect.poll(() => this.dialog.evaluate(element => {
      const title = document.getElementById(element.getAttribute("aria-labelledby") ?? "");
      const description = document.getElementById(element.getAttribute("aria-describedby") ?? "");
      if (!title || !description) return false;
      const titleBox = title.getBoundingClientRect(), descriptionBox = description.getBoundingClientRect(), drawerBox = element.getBoundingClientRect();
      return titleBox.height > 0 && descriptionBox.height > 0 && titleBox.bottom <= descriptionBox.top + 1
        && descriptionBox.right <= drawerBox.right && descriptionBox.left >= drawerBox.left;
    })).toBe(true);
    await this.expectFullWidth("provider-import-category");
    await this.expectFullWidth("provider-import-pricing-mode");
  }
  async expectFullWidth(id: string) {
    await expect.poll(async () => {
      const control = await this.dialog.getByTestId(id).boundingBox();
      const reference = await this.input("provider-import-name-zh").boundingBox();
      return control && reference ? control.width / reference.width : 0;
    }).toBeGreaterThanOrEqual(0.95);
  }
  async category(name: string) {
    await this.dialog.getByTestId("provider-import-category").click();
    await this.dialog.locator(".ant-select-dropdown:visible").getByText(name, { exact: true }).click();
  }
  async markup(value: string) { await this.input("provider-import-markup").fill(value); }
  private async select(id: string, label: string | RegExp) {
    const control = this.dialog.getByTestId(id);
    await control.click();
    const listId = await control.getByRole("combobox").getAttribute("aria-controls");
    expect(listId).toBeTruthy();
    // An outgoing popup can remain visible during its leave animation. Select only this control's menu.
    const popup = this.dialog.locator(".ant-select-dropdown").filter({ has: this.page.locator(`[id="${listId}"]`) });
    await expect(popup).toBeVisible();
    await popup.getByText(label, { exact: true }).click();
  }
  async manual() {
    await this.select("provider-import-pricing-mode", /^(自行填写最终售价|Enter final selling prices)$/u);
    await expect(this.input("provider-import-currency-0")).toBeVisible();
  }
  async addPrice() { await this.dialog.getByRole("button", { name: /^(增加一条售价|Add a selling price)$/u }).click(); }
  async removeLastPrice() { await this.removePrice.click(); }
  async price(index: number, input: { currency: string; periodLabel: string; count: string; amount: string; setup: string }) {
    await this.input(`provider-import-currency-${index}`).fill(input.currency);
    await this.expectFullWidth(`provider-import-period-${index}`);
    await this.select(`provider-import-period-${index}`, input.periodLabel);
    await this.input(`provider-import-count-${index}`).fill(input.count);
    await this.input(`provider-import-amount-${index}`).fill(input.amount);
    await this.input(`provider-import-setup-${index}`).fill(input.setup);
  }
  async submit() { await this.confirm.click(); }
  async submitAndWait() {
    const [response] = await Promise.all([
      this.page.waitForResponse(response => response.request().method() === "POST"
        && new URL(response.url()).pathname === "/x/idc-core/api/v1/admin/catalog/product-imports"),
      this.submit(),
    ]);
    return response;
  }
  async close() { await this.page.keyboard.press("Escape"); await expect(this.dialog).toBeHidden(); }
}

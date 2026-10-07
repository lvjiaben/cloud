import { expect } from "@host-tests/support/playwright";
import type { Page } from "@host-tests/support/playwright";

/** Common price and pagination interactions for both provider-detail surfaces. */
export class ProviderCatalogPage {
  constructor(readonly page: Page) {}
  get dialog() { return this.page.getByRole("dialog").filter({ has: this.page.getByRole("tab", { name: /^(上游商品|Upstream products)$/u }) }); }
  get root() { return this.page.getByTestId("provider-catalog"); }
  get grid() { return this.root.locator(".vxe-grid").first(); }
  get pager() { return this.grid.locator(".vxe-pager"); }
  get prices() { return this.page.getByTestId("provider-catalog-prices"); }
  async expectDirectoryPagerVisible() {
    const pager = this.page.getByTestId("provider-configuration-page").locator(".ant-tabs-tabpane-active .vxe-pager");
    await expect.poll(() => pager.evaluate(element => {
      const bounds = element.getBoundingClientRect(), card = element.closest(".ant-card"), footer = document.querySelector("footer");
      if (!card || !bounds.height || bounds.bottom > Math.min(innerHeight, card.getBoundingClientRect().bottom, footer?.getBoundingClientRect().top ?? innerHeight) + 1) return false;
      for (let parent = element.parentElement; parent; parent = parent.parentElement) {
        if (["auto", "hidden", "scroll", "clip"].includes(getComputedStyle(parent).overflowY)
          && bounds.bottom > parent.getBoundingClientRect().bottom + 1) return false;
      }
      return true;
    })).toBe(true);
  }
  row(name: string) { return this.grid.locator(".vxe-table--main-wrapper .vxe-body--row").filter({ has: this.page.getByText(name, { exact: true }) }); }
  async open() { await this.dialog.getByRole("tab", { name: /^(上游商品|Upstream products)$/u }).click(); await expect(this.root).toBeVisible(); }
  async numberPage(page: number) { await this.pager.locator(".vxe-pager--num-btn").filter({ hasText: new RegExp(`^${page}$`, "u") }).click(); }
  async viewPrices(name: string) {
    const rowId = await this.row(name).getAttribute("rowid");
    if (!rowId) throw new Error("The upstream product has no stable selection key");
    // Vxe only displays its fixed-column clone when horizontal overflow exists.
    // Select the visible business action in either layout, not a hidden clone.
    await this.grid.locator(`.vxe-body--row[rowid="${rowId}"]`).getByRole("button", { name: /^(查看价格|View prices)$/u }).filter({ visible: true }).click();
  }
  async search(keyword: string) {
    const field = this.root.getByTestId("provider-catalog-search");
    await field.fill(keyword);
    // Vben forms do not submit on Enter by default; exercise the visible action.
    const [response] = await Promise.all([
      this.page.waitForResponse(response => {
        const url = new URL(response.url());
        return response.request().method() === "GET"
          && url.pathname.endsWith("/catalog-products")
          && url.searchParams.get("page") === "1"
          && (url.searchParams.get("keyword") ?? "") === keyword;
      }),
      this.grid.getByRole("button", { name: /^(搜\s*索|Search)$/u }).click(),
    ]);
    return response;
  }
  async close() { await this.page.keyboard.press("Escape"); await expect(this.root).toBeHidden(); }
}

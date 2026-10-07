import type { Locator, Page } from "@host-tests/support/playwright";
import { expect } from "@host-tests/support/playwright";

export class NumberedTable {
  readonly grid: Locator;
  readonly rows: Locator;
  readonly pager: Locator;

  constructor(private readonly page: Page, root: Locator) {
    this.grid = root.locator(".vxe-grid:visible").first();
    this.rows = this.grid.locator(".vxe-table--main-wrapper .vxe-body--row");
    this.pager = this.grid.locator(".vxe-pager");
  }

  async selectPage(page: number) {
    await this.pager.scrollIntoViewIfNeeded();
    await this.pager.locator(".vxe-pager--num-btn").filter({ hasText: new RegExp(`^${page}$`, "u") }).click();
  }

  async selectSize(size: number) {
    await this.pager.locator(".vxe-select").click();
    await this.page.locator(".vxe-select-option").filter({ hasText: new RegExp(`^${size}\\s*(?:条/页|/page)$`, "u") }).click();
  }

  activePage() { return this.pager.locator(".vxe-pager--num-btn.is--active"); }

  async assertBoundedGeometry() {
    await expect(this.pager).toBeInViewport();
    await expect.poll(() => this.grid.evaluate((element) => {
      const bounds = element.getBoundingClientRect();
      return bounds.left >= 0 && bounds.right <= window.innerWidth + 1
        && document.documentElement.scrollWidth <= window.innerWidth + 1;
    })).toBe(true);
    await expect.poll(() => this.grid.locator('.vxe-table--main-wrapper .vxe-table--body-wrapper').evaluate((element) => element.clientHeight)).toBeGreaterThan(90);
    let previousHeight = (await this.grid.boundingBox())?.height ?? 0;
    let stableSamples = 0;
    await expect.poll(async () => {
      const height = (await this.grid.boundingBox())?.height ?? 0;
      stableSamples = height > 0 && Math.abs(height - previousHeight) <= 1 ? stableSamples + 1 : 0;
      previousHeight = height;
      return stableSamples;
    }, { intervals: [200, 200, 200], timeout: 5000 }).toBeGreaterThanOrEqual(3);
  }
}

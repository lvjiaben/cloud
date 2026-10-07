import type { Page } from "@host-tests/support/playwright";
import { expect } from "../../../../../../hack/tests/fixtures/auth";

export class MigrationStandalonePage {
  constructor(protected page: Page, private frameSelector = "#migration") {}
  frame() { return this.page.frameLocator(this.frameSelector); }
  grid() { return this.frame().locator("#conflict-grid .vxe-grid"); }
  rows() { return this.grid().locator(".vxe-table--main-wrapper .vxe-body--row"); }
  pager() { return this.grid().locator(".vxe-pager"); }
  async createBatch() {
    await this.frame().locator("#preflight-form [name=sourceVersion]").fill("v10");
    await this.frame().locator("#preflight-form button[type=submit]").click();
    await expect(this.frame().locator("#preflight-state")).toHaveText("检查通过");
    await this.frame().locator("#batch-form [name=batchNo]").fill("MIG-PAGE-42");
    await this.frame().locator("#create-batch").click();
    await this.frame().locator('[data-action="run"]').click();
  }
  async selectPage(page: number) { await this.pager().locator(".vxe-pager--num-btn").filter({ hasText: new RegExp(`^${page}$`) }).click(); }
  async selectSize(size: number) {
    await this.pager().locator(".vxe-pager--sizes").click();
    await this.frame().locator(".vxe-select-option:visible").filter({ hasText: new RegExp(`^${size}\\s*条/页$`) }).dispatchEvent("click");
  }
  async assertTableSurface() {
    await this.pager().scrollIntoViewIfNeeded();
    await expect(this.pager()).toBeInViewport();
    await this.page.mouse.move(0, 0);
    const surfaces = await this.grid().evaluate((root) => {
      const visibleColor = (element: Element) => {
        let current: Element | null = element;
        while (current) {
          const value = getComputedStyle(current).backgroundColor;
          if (value !== "transparent" && value !== "rgba(0, 0, 0, 0)") return value;
          current = current.parentElement;
        }
        return "transparent";
      };
      const themeColor = (token: string) => {
        const probe = document.createElement("span");
        probe.style.backgroundColor = `hsl(${getComputedStyle(document.documentElement).getPropertyValue(token)})`;
        document.body.append(probe);
        const value = getComputedStyle(probe).backgroundColor;
        probe.remove();
        return value;
      };
      const colors = (selector: string) => [...root.querySelectorAll(selector)]
        .filter((element) => element.getBoundingClientRect().height > 0).map(visibleColor);
      return {
        header: themeColor("--muted"), panel: themeColor("--card"),
        headers: colors(".vxe-table--main-wrapper .vxe-header--column"),
        panels: colors(".vxe-table--main-wrapper .vxe-body--column,.vxe-toolbar,.vxe-pager"),
      };
    });
    expect(surfaces.header).not.toBe(surfaces.panel);
    expect(surfaces.panel).toBe("rgb(255, 255, 255)");
    expect(surfaces.headers.length).toBeGreaterThan(0);
    expect(surfaces.panels.length).toBeGreaterThan(0);
    expect(surfaces.headers.every((value) => value === surfaces.header)).toBe(true);
    expect(surfaces.panels.every((value) => value === surfaces.panel)).toBe(true);
  }
}

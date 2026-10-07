import { expect } from "@host-tests/support/playwright";
import { ProviderConfigurationPage } from "./ProviderConfigurationPage";

/** Stable page-level interactions for the paged usage directory and Vben drawer. */
export class ProviderUsagePage extends ProviderConfigurationPage {
  get grid() { return this.root.locator(".ant-tabs-tabpane-active .vxe-grid").first(); }
  get pager() { return this.grid.locator(".vxe-pager"); }
  get activePage() { return this.pager.locator(".vxe-pager--num-btn.is--active"); }
  get drawer() { return this.page.getByRole("dialog").filter({ hasText: "服务器接口详情" }); }
  usageRow(name: string) {
    return this.grid
      .locator(".vxe-table--main-wrapper")
      .getByText(name, { exact: true })
      .locator("xpath=ancestor::tr[1]");
  }
  async details(name: string) {
    const row = this.usageRow(name);
    const rowId = await row.getAttribute("rowid");
    if (!rowId) throw new Error("The business row has no stable Vben row key");
    await this.grid.locator(`.vxe-table--fixed-right-wrapper .vxe-body--row[rowid="${rowId}"]`).getByRole("button", { name: "详情", exact: true }).click();
    await expect(this.drawer).toBeVisible();
  }
  async closeDetails() { await this.page.keyboard.press("Escape"); await expect(this.drawer).toBeHidden(); }
  async numberPage(number: number) { await this.pager.locator(".vxe-pager--num-btn").filter({ hasText: new RegExp(`^${number}$`, "u") }).click(); }
  async nextPage() { await this.pager.locator(".vxe-pager--next-btn").click(); }
  async pageSize(size: number) {
    await this.pager.locator(".vxe-pager--sizes").click();
    await this.page.locator(".vxe-select-option:visible").filter({ hasText: new RegExp(`^${size}\\s*条/页$`, "u") }).click({ force: true });
  }
  async activePageText() {
    return this.grid.evaluate((root) =>
      root.querySelector(".vxe-pager--num-btn.is--active")?.textContent?.trim() ?? "",
    );
  }
  async pools() { await this.root.getByRole("tab", { name: "资源池绑定" }).click(); }
  async refresh() { await this.root.getByRole("button", { name: /刷\s*新/u }).click({ force: true }); }
  async refreshWhileDetailsOpen() {
    // Exercise an already-mounted directory refresh while its detail is open.
    await this.root.evaluate(root => {
      const refresh = Array.from(root.querySelectorAll("button")).find(button => button.textContent?.replaceAll(/\s/gu, "") === "刷新");
      if (!refresh) throw new Error("The directory refresh action is missing");
      refresh.click();
    });
  }
  async stableScreenshot(name: string) {
    await this.page.evaluate(async () => {
      await Promise.all(document.getAnimations().filter(animation => Number.isFinite(animation.effect?.getComputedTiming().endTime)).map(animation => animation.finished.catch(() => undefined)));
    });
    return this.screenshot(name);
  }
}

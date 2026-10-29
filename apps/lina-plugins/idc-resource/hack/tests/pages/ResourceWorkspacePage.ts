import { expect } from "@host-tests/support/playwright";
import type { Page } from "@host-tests/support/playwright";
import { ResourceOverviewPage } from "./ResourceOverviewPage";
import { workspacePath } from "../../../../../../hack/tests/fixtures/config";

export class ResourceWorkspacePage extends ResourceOverviewPage {
  constructor(page: Page) { super(page); }
  async open(_workspace: "providers") {
    await this.page.goto(workspacePath("/idc/resource/provider-configuration"), { waitUntil: "domcontentloaded" });
    await this.page.getByTestId("provider-operations-page").waitFor();
  }
  async openShortcut() {
    await this.page.goto(workspacePath("/idc/resource/provider-tools"));
    await this.page.getByTestId("provider-operations-page").waitFor();
  }
  async expectGridFits(testId: string) {
    const panel = this.page.getByTestId(testId);
    await expect(panel).toBeVisible();
    await expect.poll(() => panel.evaluate(root => {
      const grid = root.querySelector('.vxe-grid');
      const pager = root.querySelector('.vxe-pager');
      if (!grid || !pager) return false;
      const bounds = root.getBoundingClientRect();
      const table = grid.getBoundingClientRect();
      const pagination = pager.getBoundingClientRect();
      return table.width > 0 && table.width >= bounds.width - 20 && pagination.bottom <= bounds.bottom + 1 && pagination.bottom <= innerHeight;
    })).toBe(true);
  }
  async expectSearchAligned() {
    const panel = this.page.getByTestId('provider-task-page');
    const fields = await panel.locator('[data-form-render-field]').all();
    const first = await fields[0]!.boundingBox();
    const last = await fields.at(-1)!.boundingBox();
    const button = await panel.getByRole('button', { name: /搜\s*索/ }).boundingBox();
    expect(Math.abs(first!.y - last!.y)).toBeLessThan(2);
    expect(Math.abs(last!.y - button!.y)).toBeLessThan(2);
    expect(button!.x - (last!.x + last!.width)).toBeLessThan(120);
    await expect(panel.getByText('收起', { exact: true })).toHaveCount(0);
  }
  tab(name: string) { return this.page.getByRole("tab", { name, exact: true }); }
  async selectTab(name: string) { await this.tab(name).click(); }
  async expandMenu() { await this.page.getByRole("menu").getByText("服务器与资源", { exact: true }).click(); }
  async refreshOverview() { await this.page.getByTestId("resource-overview-page").getByRole("button", { name: /刷\s*新/ }).click(); }
}

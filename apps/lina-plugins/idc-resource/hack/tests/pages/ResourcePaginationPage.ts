import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
import type { Page } from "@host-tests/support/playwright";
import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { resourceDirectories, type ResourceDirectoryKind } from "../support/resource-pagination-mock";

export class ResourcePaginationPage {
  constructor(readonly page: Page, readonly kind: ResourceDirectoryKind) {}
  get root() { return this.page.getByTestId(resourceDirectories[this.kind].root); }
  grid(index = 0) { return this.root.locator(".vxe-grid").nth(index); }
  pager(index = 0) { return this.grid(index).locator(".vxe-pager"); }
  active(index = 0) { return this.pager(index).locator(".vxe-pager--num-btn.is--active"); }
  rows(index = 0) { return this.grid(index).locator(".vxe-table--main-wrapper .vxe-body--row"); }
  async goto() { await this.page.goto(workspacePath(`/idc/resource/${resourceDirectories[this.kind].route}`), { waitUntil: "domcontentloaded" }); await this.pager().waitFor(); }
  async next(index = 0) { await this.pager(index).locator(".vxe-pager--next-btn").click(); }
  async size(value: number, index = 0) {
    await this.pager(index).locator(".vxe-pager--sizes").click();
    await this.page.locator(".vxe-select-option:visible").filter({ hasText: new RegExp(`^${value}\\s*条/页$`, "u") }).click();
  }
  async refresh() {
    if (this.kind === "overview") await this.root.getByRole("button", { name: "刷新本地数据" }).click();
    else if (this.kind === "renewals") await this.root.getByTestId("server-renewal-refresh").click();
    else await this.grid().locator('.vxe-toolbar [title="刷新"], .vxe-toolbar [title="Refresh"]').click();
  }
  async screenshot(name: string) {
    await this.page.evaluate(async () => { await Promise.all(document.getAnimations().filter(a => Number.isFinite(a.effect?.getComputedTiming().endTime)).map(a => a.finished.catch(() => undefined))); });
    const now = new Date();
    const directory = fileURLToPath(new URL(`../../../../../../temp/${now.toISOString().slice(0, 10).replaceAll("-", "")}/`, import.meta.url));
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({ path: resolve(directory, `${now.toISOString().slice(11, 19).replaceAll(":", "")}-${name}.png`) });
  }
}

import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class ProductFulfillmentPage {
  readonly page: Page;
  readonly pageRoot: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("product-fulfillment-page");
  }

  async goto() {
    // The authenticated fixture boots before this test installs its route
    // contract. Reload once so the host discovers the mocked plugin menu.
    await this.page.reload({ waitUntil: "domcontentloaded" });
    await this.page.goto(workspacePath("/idc/product-fulfillment/tasks"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible", timeout: 15_000 });
    await waitForRouteReady(this.page, 15_000);
    await this.pageRoot.locator(".vxe-body--row").first().waitFor({ timeout: 15_000 });
  }

  row(orderNo: string) {
    return this.pageRoot.locator(".vxe-body--row", { hasText: orderNo }).first();
  }

  async openDetail(orderNo: string) {
    await this.pageRoot
      .getByRole("button", { name: /详\s*情/u })
      .first()
      .click();
    await this.page.locator('[role="dialog"]:visible').filter({ hasText: orderNo }).waitFor();
  }

  async screenshot(description: string) {
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ fullPage: false, path });
    return path;
  }
}

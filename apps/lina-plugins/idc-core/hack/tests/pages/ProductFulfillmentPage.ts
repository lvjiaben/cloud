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
    await this.page.getByText("资金结清", { exact: true }).waitFor();
  }

  detail() {
    return this.page.locator('[role="dialog"]:visible').filter({ hasText: "ORD-MOCK-1" }).last();
  }

  actionModal() {
    return this.page.locator('[role="dialog"]:visible').filter({ has: this.page.getByLabel("处理原因") });
  }

  async openManualAction(completed = true) {
    await this.detail().getByRole("button", { name: completed ? "登记已完成" : "登记无法完成", exact: true }).click();
    await this.actionModal().waitFor({ state: "visible" });
  }

  async fillManualAction(note: string, completed = true) {
    await this.actionModal().getByLabel("处理原因").click();
    await this.page.locator(".ant-select-dropdown:visible").getByText(completed ? "人工履约已完成" : "无法完成履约", { exact: true }).click();
    await this.page.locator(".ant-select-dropdown:visible").waitFor({ state: "hidden" });
    await this.actionModal().getByLabel("处理说明").fill(note);
  }

  async submitManualAction() {
    await this.actionModal().getByRole("button", { name: /确\s*(认|定)/u }).click();
  }

  async screenshot(description: string) {
    // Wait for finite drawer/modal transitions so visual evidence captures
    // the actual layout, rather than an intermediate slide-in frame.
    await this.page.evaluate(async () => {
      await new Promise<void>((resolveFrame) => requestAnimationFrame(() => resolveFrame()));
      await Promise.all(document.getAnimations().filter((animation) => animation.effect?.getTiming().iterations !== Infinity).map((animation) => animation.finished.catch(() => undefined)));
    });
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

import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import {
  waitForRouteReady,
  waitForTableReady,
} from "../../../../../../hack/tests/support/ui";

export class ProviderTaskPage {
  readonly detailDrawer: Locator;
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("provider-task-page");
    this.detailDrawer = page.getByTestId("provider-task-detail");
  }

  async goto() {
    await this.page.reload({ waitUntil: "domcontentloaded" });
    await this.page.goto(workspacePath("/idc/resource/tasks"), {
      waitUntil: "domcontentloaded",
    });
    await this.root.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await waitForTableReady(this.page);
  }

  row(taskNo: string) {
    return this.page.getByTestId(`provider-task-status-${taskNo}`);
  }

  async openDetail(taskNo: string) {
    await this.page.getByTestId(`provider-task-detail-${taskNo}`).click();
    await this.detailDrawer.waitFor({ state: "visible", timeout: 15_000 });
  }

  evidence(evidenceNo: string) {
    return this.page.getByTestId(`provider-task-evidence-${evidenceNo}`);
  }

  async selectReason(label: string) {
    await this.page.getByTestId("provider-task-takeover-reason").click();
    const option = this.page
      .locator(".ant-select-item-option")
      .filter({ hasText: label });
    await option.waitFor({ state: "visible", timeout: 15_000 });
    await option.click();
  }

  async screenshot(name: string) {
    const now = new Date();
    const date = now.toISOString().slice(0, 10).replaceAll("-", "");
    const time = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "temp", date);
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({
      animations: "disabled",
      fullPage: false,
      path: resolve(directory, `${time}-${name}.png`),
    });
  }
}

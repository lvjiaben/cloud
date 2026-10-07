import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class CatalogAutomationPage {
  readonly createModal: Locator;
  readonly page: Page;
  readonly pageRoot: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("catalog-automation-interface-page");
    this.createModal = page.getByTestId("automation-interface-create-modal");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/catalog/automation-interfaces"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.page.getByText(/加载菜单中|Loading Menu/iu).waitFor({
      state: "hidden",
      timeout: 15_000,
    });
    await this.waitForDataSettled();
  }

  row(name: string) {
    return this.pageRoot.locator(".vxe-body--row", { hasText: name }).first();
  }

  async waitForDataSettled() {
    await this.page.getByText(/加载菜单中|Loading Menu/iu).waitFor({
      state: "hidden",
      timeout: 15_000,
    });
    await this.pageRoot.locator(".bg-overlay-content:not(.invisible)").waitFor({
      state: "detached",
      timeout: 15_000,
    });
  }

  async screenshot(description: string) {
    await this.waitForDataSettled();
    return this.capture(description);
  }

  async screenshotWithBusinessError(description: string, messageText: string) {
    const { directory, stamp } = await this.prepareScreenshotTarget();
    await this.page.getByText(/加载菜单中|Loading Menu/iu).waitFor({
      state: "hidden",
      timeout: 15_000,
    });
    const notice = this.page
      .locator(".ant-message-notice-content")
      .filter({ hasText: messageText });
    await notice.waitFor({ state: "visible" });
    const noticePath = resolve(
      directory,
      `${stamp}-${description}-error-notice.png`,
    );
    const pagePath = resolve(
      directory,
      `${stamp}-${description}-error-page.png`,
    );
    await Promise.all([
      this.page.screenshot({ fullPage: false, path: pagePath }),
      notice.screenshot({ path: noticePath }),
    ]);
    return { noticePath, pagePath };
  }

  private async capture(description: string) {
    const { directory, stamp } = await this.prepareScreenshotTarget();
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ fullPage: false, path });
    return path;
  }

  private async prepareScreenshotTarget() {
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    return { directory, stamp };
  }
}

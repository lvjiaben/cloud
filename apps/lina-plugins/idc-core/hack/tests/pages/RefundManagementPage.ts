import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { LoginPage } from "../../../../../../hack/tests/pages/LoginPage";
import {
  config,
  workspacePath,
} from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class RefundManagementPage {
  readonly page: Page;
  readonly panel: Locator;

  constructor(page: Page) {
    this.page = page;
    this.panel = page.getByTestId("refund-management-panel");
  }

  async goto() {
    const target = workspacePath("/idc/finance");
    await this.page.goto(target, {
      waitUntil: "domcontentloaded",
    });
    const refundTab = this.page.getByTestId("refund-management-tab");
    await Promise.race([
      this.page.waitForURL((url) => url.pathname.includes("/auth/login"), {
        timeout: 15_000,
      }),
      refundTab.waitFor({ state: "visible", timeout: 15_000 }),
    ]).catch(() => undefined);
    if (this.page.url().includes("/auth/login")) {
      const login = new LoginPage(this.page);
      await login.loginAndWaitForRedirect(config.adminUser, config.adminPass);
      await this.page.goto(target, { waitUntil: "domcontentloaded" });
    }
    await waitForRouteReady(this.page, 15_000);
    await refundTab.click();
    await this.panel.waitFor({ state: "visible" });
  }

  row(refundNo: string) {
    return this.panel
      .getByText(refundNo, { exact: true })
      .locator("xpath=ancestor::tr");
  }

  async openDetail(_refundNo: string) {
    await this.panel.getByTestId("refund-detail-open").first().click();
    await this.page
      .getByTestId("refund-detail-drawer-content")
      .waitFor({ state: "visible" });
  }

  async openReview(_refundNo: string) {
    await this.panel.getByTestId("refund-review-open").first().click();
    await this.page
      .getByTestId("refund-review-modal-content")
      .waitFor({ state: "visible" });
  }

  async chooseDecision(name: "批准" | "驳回") {
    await this.page
      .getByTestId("refund-review-modal-content")
      .getByText(name, { exact: true })
      .click();
  }

  async fillEvidence(value: string) {
    await this.page.getByTestId("refund-review-evidence").fill(value);
  }

  async submitReview() {
    await this.page
      .getByRole("button", { name: /确\s*认|确\s*定|Confirm/iu })
      .last()
      .click();
  }

  async screenshot(description: string) {
    await this.page
      .getByText(/加载菜单中|Loading Menu/u)
      .waitFor({ state: "hidden" });
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ animations: "disabled", fullPage: false, path });
    return path;
  }
}

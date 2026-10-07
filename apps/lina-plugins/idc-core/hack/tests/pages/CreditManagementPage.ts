import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { LoginPage } from "../../../../../../hack/tests/pages/LoginPage";
import {
  config,
  workspacePath,
} from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class CreditManagementPage {
  readonly page: Page;
  readonly panel: Locator;

  constructor(page: Page) {
    this.page = page;
    this.panel = page.getByTestId("credit-management-panel");
  }

  async goto() {
    const target = workspacePath("/idc/finance");
    await this.page.goto(target, { waitUntil: "domcontentloaded" });
    const tab = this.page.getByTestId("credit-management-tab");
    await Promise.race([
      this.page.waitForURL((url) => url.pathname.includes("/auth/login"), {
        timeout: 15_000,
      }),
      tab.waitFor({ state: "visible", timeout: 15_000 }),
    ]).catch(() => undefined);
    if (this.page.url().includes("/auth/login")) {
      const login = new LoginPage(this.page);
      await login.loginAndWaitForRedirect(config.adminUser, config.adminPass);
      await this.page.goto(target, { waitUntil: "domcontentloaded" });
    }
    await waitForRouteReady(this.page, 15_000);
    await tab.click({ timeout: 15_000 });
    await this.panel.waitFor({ state: "visible" });
  }

  applicationRow(applicationNo: string) {
    return this.panel
      .locator(".vxe-table--main-wrapper .vxe-body--row")
      .filter({ has: this.page.getByText(applicationNo, { exact: true }) });
  }

  async openReview(applicationNo: string) {
    const rowId =
      await this.applicationRow(applicationNo).getAttribute("rowid");
    await this.panel
      .locator(
        `.vxe-body--row[rowid="${rowId}"] [data-testid="credit-application-review"]:visible`,
      )
      .last()
      .click();
    await this.page
      .getByTestId("credit-review-modal")
      .waitFor({ state: "visible" });
    await this.page
      .getByRole("dialog")
      .filter({ has: this.page.getByTestId("credit-review-modal") })
      .evaluate(async (element) => {
        await Promise.all(
          element
            .getAnimations({ subtree: true })
            .map((animation) => animation.finished.catch(() => undefined)),
        );
      });
  }

  async openDetail(applicationNo: string) {
    const rowId = await this.applicationRow(applicationNo).getAttribute("rowid");
    await this.panel.locator(`.vxe-body--row[rowid="${rowId}"] [data-testid="credit-application-detail"]:visible`).last().click();
    await this.page.getByTestId("credit-application-detail-drawer").waitFor({ state: "visible" });
  }

  async screenshot(description: string) {
    await this.page
      .getByText("加载菜单中...", { exact: true })
      .waitFor({ state: "hidden" });
    await this.page
      .getByText("Loading Menu...", { exact: true })
      .waitFor({ state: "hidden" });
    await this.page.evaluate(async () => {
      const finiteAnimations = document.getAnimations().filter((animation) => {
        const endTime = animation.effect?.getComputedTiming().endTime;
        return typeof endTime === "number" && Number.isFinite(endTime);
      });
      await Promise.all(
        finiteAnimations.map((animation) =>
          animation.finished.catch(() => undefined),
        ),
      );
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

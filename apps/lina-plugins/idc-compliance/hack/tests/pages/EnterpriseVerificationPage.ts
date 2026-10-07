import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class EnterpriseVerificationPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("enterprise-verification-page");
  }

  async goto() {
    await this.page.goto(
      workspacePath("/idc/compliance/enterprise-verifications"),
      {
        waitUntil: "domcontentloaded",
      },
    );
    await waitForRouteReady(this.page, 15_000);
    await this.root.waitFor({ state: "visible" });
    await this.page.locator(".ant-message-notice").waitFor({ state: "hidden" });
  }

  async openCase(caseNo: string) {
    await this.page.getByTestId(`enterprise-case-detail-${caseNo}`).click();
    const detail = this.page.getByTestId("enterprise-case-detail");
    await detail.waitFor({ state: "visible" });
    // Visibility becomes true at the start of Ant Design's slide-in transition.
    // Wait for that short visual transition so screenshots prove the full drawer.
    await this.page.waitForTimeout(400);
    return detail;
  }

  async downloadMaterial(materialNo: string) {
    await this.page
      .getByTestId(`enterprise-material-download-${materialNo}`)
      .click();
  }

  async startReview() {
    await this.page.getByTestId("enterprise-review-start").click();
    await this.confirm();
  }

  async approve() {
    await this.page.getByTestId("enterprise-review-approve").click();
    await this.confirm();
  }

  async reject(reasonLabel: string) {
    await this.page.getByTestId("enterprise-rejection-reason").click();
    await this.page.getByText(reasonLabel, { exact: true }).last().click();
    await this.page.getByTestId("enterprise-review-reject").click();
  }

  confirmationDialog() {
    return this.page.getByRole("dialog").last();
  }

  async confirm() {
    const dialog = this.confirmationDialog();
    await dialog.locator(".ant-btn-primary").last().click();
    await this.page.waitForTimeout(400);
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

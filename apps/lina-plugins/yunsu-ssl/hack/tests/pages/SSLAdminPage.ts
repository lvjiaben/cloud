import type { Page } from "@playwright/test";

import { workspacePath } from "@host-tests/fixtures/config";
import { waitForRouteReady } from "@host-tests/support/ui";

export class SSLAdminPage {
  constructor(readonly page: Page) {}

  frame() {
    return this.page.frameLocator(
      'iframe[src="/x-assets/yunsu-ssl/v0.3.4/admin/certificates/index.html"]',
    );
  }

  async open() {
    await this.page.goto(workspacePath("/dashboard/analytics"), {
      waitUntil: "domcontentloaded",
    });
    await waitForRouteReady(this.page, 15_000);
    await this.page
      .getByText("SSL 证书管理", { exact: true })
      .last()
      .click({ timeout: 15_000 });
    await this.page.waitForURL(/\/plugins\/yunsu-ssl\/ssl-certificates/u);
    await this.frame()
      .locator('.vxe-table--main-wrapper .vxe-table--body-wrapper')
      .getByText("admin-example.test", { exact: true })
      .waitFor({ timeout: 10_000 });
    const routeOverlay = this.page.locator(".bg-overlay-content").first();
    await routeOverlay
      .waitFor({ state: "visible", timeout: 3_000 })
      .catch(() => undefined);
    await routeOverlay.waitFor({ state: "hidden", timeout: 10_000 });
  }

  async viewDetails() {
    await this.frame()
      .getByRole("button", { name: /查看详情|View details/u })
      .first()
      .click();
    await this.frame()
      .getByRole("dialog")
      .filter({ hasText: /证书详情|Certificate details/u })
      .waitFor();
  }
}

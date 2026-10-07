import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class FinancialManagementPage {
  readonly page: Page;
  readonly pageRoot: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("financial-management-page");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/finance"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
  }

  invoiceRow(invoiceNo: string) {
    return this.pageRoot.locator(`tr:has-text("${invoiceNo}")`);
  }
}

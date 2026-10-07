import type { Locator, Page } from "@host-tests/support/playwright";
import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class ServerBatchRenewalPage {
  readonly page: Page;
  readonly root: Locator;
  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("server-batch-renewal-page");
  }
  async goto() {
    await this.page.goto(workspacePath("/idc/resource/server-renewals"), {
      waitUntil: "domcontentloaded",
    });
    await this.root.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.root
      .getByText("SVR-AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA", { exact: true })
      .waitFor({ state: "visible" });
  }
  async select(serverNo: string) {
    await this.root
      .locator("tr", { hasText: serverNo })
      .locator(".ant-checkbox-input")
      .click();
  }
  async preview() {
    await this.page.getByTestId("server-renewal-preview").click();
  }
}

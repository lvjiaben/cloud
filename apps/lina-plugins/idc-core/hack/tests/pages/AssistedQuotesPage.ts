import { mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import path from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

const repositoryRoot = fileURLToPath(
  new URL("../../../../../../", import.meta.url),
);

export class AssistedQuotesPage {
  readonly page: Page;
  readonly pageRoot: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("assisted-quote-page");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/assisted-quotes"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
  }

  row(requestNo: string) {
    return this.pageRoot.locator(`tr:has-text("${requestNo}")`);
  }

  async openDetail(requestNo: string) {
    await this.pageRoot.getByTestId(`assisted-quote-detail-${requestNo}`).click();
    const drawer = this.page.getByRole("dialog");
    await drawer.waitFor({ state: "visible" });
  }

  async screenshot(description: string) {
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = path.join(repositoryRoot, "temp", day);
    await mkdir(directory, { recursive: true });
    const screenshotPath = path.join(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ fullPage: false, path: screenshotPath });
    return screenshotPath;
  }
}

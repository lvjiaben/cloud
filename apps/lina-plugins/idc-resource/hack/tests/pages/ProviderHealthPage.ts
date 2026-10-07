import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import {
  waitForDialogReady,
  waitForRouteReady,
  waitForTableReady,
} from "../../../../../../hack/tests/support/ui";

export class ProviderHealthPage {
  readonly page: Page;
  readonly policyDialog: Locator;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.policyDialog = page.getByRole("dialog");
    this.root = page.getByTestId("provider-health-page");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/resource/health"), {
      waitUntil: "domcontentloaded",
    });
    await this.root.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await waitForTableReady(this.page);
  }

  row(channelName: string) {
    return this.root.locator("tr").filter({ hasText: channelName }).first();
  }

  async openPolicy(channelNo: string) {
    await this.page.getByTestId(`provider-health-policy-${channelNo}`).click();
    await waitForDialogReady(this.policyDialog);
  }

  async screenshot(name: string) {
    const now = new Date();
    const date = now.toISOString().slice(0, 10).replaceAll("-", "");
    const time = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "temp", date);
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({
      fullPage: false,
      path: resolve(directory, `${time}-${name}.png`),
    });
  }
}

import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class ResourceOverviewPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("resource-overview-page");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/resource/overview"), { waitUntil: "domcontentloaded" });
    await this.root.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
  }

  async screenshot(name: string) {
    const now = new Date();
    const directory = resolve(process.cwd(), "temp", now.toISOString().slice(0, 10).replaceAll("-", ""));
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({ fullPage: false, path: resolve(directory, `${now.toISOString().slice(11, 19).replaceAll(":", "")}-${name}.png`) });
  }
}

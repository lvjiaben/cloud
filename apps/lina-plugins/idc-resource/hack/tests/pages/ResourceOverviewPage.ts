import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";

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
    await this.page.getByText("加载菜单中...", { exact: true }).waitFor({ state: "hidden" });
    await this.page.waitForTimeout(400);
    const now = new Date();
    const directory = resolve(fileURLToPath(new URL("../../../../../../temp/", import.meta.url)), now.toISOString().slice(0, 10).replaceAll("-", ""));
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({ fullPage: false, path: resolve(directory, `${now.toISOString().slice(11, 19).replaceAll(":", "")}-${name}.png`) });
  }
}

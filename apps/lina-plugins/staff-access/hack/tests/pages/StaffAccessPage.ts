import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";

export class StaffAccessPage {
  readonly page: Page;
  readonly pageRoot: Locator;
  readonly total: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("staff-access-page");
    this.total = page.getByTestId("staff-access-total");
  }

  async goto() {
    await this.page.goto(workspacePath("/system/staff-access"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible", timeout: 15_000 });
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

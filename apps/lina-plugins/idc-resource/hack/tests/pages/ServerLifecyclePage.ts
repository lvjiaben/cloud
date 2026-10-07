import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { lifecycleServiceNo } from "../support/server-lifecycle-mock";

export class ServerLifecyclePage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("customer-servers-page");
  }

  async login() {
    await this.page.goto("/auth/login");
    const inputs = this.page.locator("input");
    await inputs.nth(0).fill("customer");
    await inputs.nth(1).fill("customer-password");
    await this.page.getByRole("button", { name: "login" }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async openServer() {
    await this.page.goto(`/account/servers/${lifecycleServiceNo}`);
    await this.page
      .getByTestId("customer-server-detail")
      .waitFor({ state: "visible" });
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

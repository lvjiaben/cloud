import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Page } from "@host-tests/support/playwright";

export class CustomerAssistancePage {
  readonly page: Page;

  constructor(page: Page) {
    this.page = page;
  }

  async waitUntilReady() {
    await this.page.waitForLoadState("domcontentloaded");
    await this.page.waitForURL(/127\.0\.0\.1:6032\/account$/u);
    await this.page.getByTestId("customer-assistance-banner").waitFor({
      state: "visible",
    });
    await this.page.getByTestId("customer-overview").waitFor({
      state: "visible",
    });
  }

  async gotoOrders() {
    await this.page.goto("http://127.0.0.1:6032/account/orders", {
      waitUntil: "domcontentloaded",
    });
    await this.page.getByTestId("customer-commerce-page").waitFor({
      state: "visible",
    });
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

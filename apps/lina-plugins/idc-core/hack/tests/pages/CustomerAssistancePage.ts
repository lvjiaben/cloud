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
    await this.page.waitForURL(url => url.pathname === "/account");
    await this.page.getByTestId("customer-assistance-banner").waitFor({
      state: "visible",
    });
    await this.page.getByTestId("customer-overview").waitFor({
      state: "visible",
    });
  }

  async gotoOrders() {
    const loaded = this.page.waitForResponse(response =>
      new URL(response.url()).pathname.endsWith("/orders") && response.request().method() === "GET"
      && ["fetch", "xhr"].includes(response.request().resourceType()),
    );
    await this.page.goto(new URL("/account/orders", this.page.url()).href, {
      waitUntil: "domcontentloaded",
    });
    await this.page.getByTestId("customer-commerce-page").waitFor({
      state: "visible",
    });
    return loaded;
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

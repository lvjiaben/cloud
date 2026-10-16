// Page object for the customer-owned platform; screenshots remain in dated temporary output.
import { mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import type { Locator, Page } from "@playwright/test";
const directory = fileURLToPath(
  new URL("../../../../../../temp/20261012/open-platform/", import.meta.url),
);
export class ChannelCustomerTokenPage {
  readonly root: Locator;
  constructor(readonly page: Page) {
    this.root = page.getByTestId("open-platform-page");
  }
  async login() {
    await this.page.goto("/auth/login");
    const inputs = this.page.locator("input");
    await inputs.nth(0).fill("customer");
    await inputs.nth(1).fill("customer-password");
    await this.page.getByRole("button", { name: "login" }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }
  async goto() {
    await this.page.goto("/account/plugins/idc-channel/open-platform", {
      waitUntil: "domcontentloaded",
    });
  }
  async screenshot(name: string) {
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({
      path: `${directory}${Date.now()}-${name}.png`,
      fullPage: false,
    });
  }
}

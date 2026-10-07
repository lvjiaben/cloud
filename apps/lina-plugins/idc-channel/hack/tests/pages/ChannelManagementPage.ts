import type { Locator, Page } from "@playwright/test";

export class ChannelManagementPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page
      .frameLocator('[data-testid="plugin-sandbox-frame"]')
      .getByTestId("idc-channel-admin-page");
  }

  async goto() {
    await this.page.goto("/admin/idc/channel", { waitUntil: "domcontentloaded" });
    await this.root.waitFor({ state: "visible" });
  }

  async createToken() {
    await this.root.getByTestId("channel-selector").selectOption("channel_primary");
    await this.root.getByRole("tab", { name: "令牌" }).click();
    await this.root.locator('textarea[name="scopes"]').fill("catalog.read\norder.create");
    await this.root.getByRole("button", { name: "创建一次性令牌" }).click();
    return this.root.getByTestId("one-time-token");
  }

  async showCapabilities() {
    await this.root.getByRole("tab", { name: "能力清单" }).click();
    return this.root.locator('section[data-panel="capabilities"]');
  }

  async revokeToken() {
    await this.root.getByRole("button", { name: "撤销", exact: true }).click();
    return this.root.getByRole("status");
  }
}

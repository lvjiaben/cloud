import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { expect } from "@host-tests/support/playwright";
import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import {
  waitForDialogReady,
  waitForRouteReady,
} from "../../../../../../hack/tests/support/ui";

export type RealProviderChannelInput = {
  connector: string;
  endpoint: string;
  name: string;
  protocol?: string;
  secrets: Record<string, string>;
};

export class ProviderConfigurationPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("provider-configuration-page");
  }

  async goto() {
    await this.page.goto(
      workspacePath("/idc/resource/provider-configuration"),
      { waitUntil: "domcontentloaded" },
    );
    await this.root.waitFor({ state: "visible", timeout: 20_000 });
    await waitForRouteReady(this.page, 20_000);
    await this.waitForDataSettled();
  }

  async createChannel(input: RealProviderChannelInput) {
    await this.root.getByRole("button", { name: "新增接口" }).click();
    const dialog = await waitForDialogReady(
      this.page.getByRole("dialog", { name: "新增接口", exact: true }),
    );

    // Vben renders Select as a div-based control; the field name is not
    // forwarded to an input element. Target the first select in the dialog
    // (the connector selector) instead of relying on an implementation detail.
    await dialog.locator(".ant-select").first().click();
    await this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(input.connector, { exact: true })
      .click();
    await dialog.locator('input[name="displayName"]').fill(input.name);
    await dialog.locator('input[name="endpoint"]').fill(input.endpoint);

    if (input.protocol) {
      await dialog.locator(".ant-select").nth(1).click();
      await this.page
        .locator(".ant-select-dropdown:visible")
        .getByText(input.protocol, { exact: true })
        .click();
    }
    for (const [field, value] of Object.entries(input.secrets)) {
      const control = dialog.locator(`input[name="${field}"]`);
      await control.waitFor({ state: "visible" });
      await control.fill(value);
    }

    const failedResponses: string[] = [];
    const responseHandler = async (response: any) => {
      if (response.url().includes("/x/idc-resource/api/v1/") && response.status() >= 400) {
        failedResponses.push(`${response.status()} ${response.url()} ${await response.text()}`);
      }
    };
    this.page.on("response", responseHandler);
    await dialog.getByRole("button", { name: /确\s*认/u }).click();
    if (failedResponses.length > 0) {
      this.page.off("response", responseHandler);
      throw new Error(`创建接口请求失败: ${failedResponses.join(" | ")}`);
    }
    try {
      await dialog.waitFor({ state: "hidden", timeout: 3_000 });
    } catch {
      const visibleText = (await dialog.innerText()).replace(/\s+/gu, " ").trim();
      const invalid = await dialog.locator('[aria-invalid="true"]').count();
      const alerts = await this.page.locator('[role="alert"]:visible').allInnerTexts();
      throw new Error(`新增接口未关闭，页面提示: ${visibleText}; invalid=${invalid}; alerts=${alerts.join(" | ")}`);
    }
    this.page.off("response", responseHandler);
    await this.waitForDataSettled();
    // VxeGrid may virtualize rows outside the current viewport; the create
    // response and the settled reload are the authoritative browser checks.
  }

  channelRow(name: string) {
    return this.root
      .locator(".vxe-table--main-wrapper .vxe-body--row")
      .filter({ hasText: name });
  }

  async waitForDataSettled() {
    await this.root.locator(".bg-overlay-content:not(.invisible)").waitFor({
      state: "detached",
      timeout: 20_000,
    });
  }

  async screenshot(description: string) {
    await this.waitForDataSettled();
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ fullPage: false, path });
    return path;
  }
}

import type { Page } from "@host-tests/support/playwright";

import { mkdirSync } from "node:fs";
import path from "node:path";

import { expect } from "@host-tests/fixtures/auth";

import { waitForRouteReady } from "@host-tests/support/ui";

const root = path.resolve(__dirname, "../../../../../..");

export class PermissionHelpPage {
  constructor(readonly page: Page) {}

  get button() {
    return this.page.getByTestId("permission-help-button");
  }
  get content() {
    return this.page.getByTestId("permission-help-content");
  }
  get drawer() {
    return this.page.getByRole("dialog");
  }

  async goto(topic: string) {
    await this.page.goto(`/system/${topic}`);
    await waitForRouteReady(this.page);
  }

  async open() {
    await this.button.click();
    await expect
      .poll(async () => {
        const box = await this.drawer.boundingBox();
        const viewport = this.page.viewportSize();
        return (
          !!box &&
          !!viewport &&
          box.x >= -1 &&
          box.x + box.width <= viewport.width + 1
        );
      })
      .toBe(true);
  }

  async close() {
    await this.page.keyboard.press("Escape");
    await this.drawer.waitFor({ state: "hidden" });
  }

  async screenshot(name: string) {
    await waitForRouteReady(this.page);
    await expect(
      this.page.getByText(/^(加载菜单中\.\.\.|Loading menus\.\.\.)$/),
    ).toHaveCount(0);
    const now = new Date();
    const day = new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Shanghai" })
      .format(now)
      .replaceAll("-", "");
    const dir = path.join(root, "temp", day);
    mkdirSync(dir, { recursive: true });
    await this.page.screenshot({
      path: path.join(dir, `${Date.now()}-permission-help-${name}.png`),
      fullPage: false,
    });
  }

  async appearance() {
    return this.button.evaluate((element) => {
      const style = getComputedStyle(element);
      return {
        background: style.backgroundColor,
        radius: style.borderRadius,
        shadow: style.boxShadow,
        position: style.position,
      };
    });
  }

  async menuTree() {
    const responsePromise = this.page.waitForResponse((response) =>
      new URL(response.url()).pathname.endsWith("/api/v1/menus/all"),
    );
    await this.goto("dept");
    const response = await responsePromise;
    if (!response.ok())
      throw new Error(`Menu request failed: ${response.status()}`);
    return response.json();
  }
}

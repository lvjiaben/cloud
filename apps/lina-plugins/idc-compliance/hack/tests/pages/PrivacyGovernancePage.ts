import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class PrivacyGovernancePage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("privacy-governance-page");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/compliance/privacy"), {
      waitUntil: "domcontentloaded",
    });
    await waitForRouteReady(this.page, 15_000);
    await this.root.waitFor({ state: "visible" });
  }

  async openTab(name: string | RegExp) {
    await this.page.getByRole("tab", { name }).click();
  }

  async confirmLastDialog() {
    const dialog = this.page.getByRole("dialog").last();
    await dialog.locator(".ant-btn-primary").last().click();
    await dialog.waitFor({ state: "hidden" });
  }

  async horizontalOverflowDetails() {
    return this.root.evaluate((root) => {
      const candidates = [
        root,
        ...root.querySelectorAll(
          ".privacy-admin, .privacy-admin > header, .privacy-admin > .ant-alert, .privacy-admin .ant-tabs, .privacy-admin .ant-card",
        ),
      ];
      return candidates.flatMap((element) => {
        const rect = element.getBoundingClientRect();
        if (rect.left >= -1 && rect.right <= window.innerWidth + 1) {
          return [];
        }
        return [
          {
            className: element.className,
            computedMaxWidth: getComputedStyle(element).maxWidth,
            dataTestId: element.getAttribute("data-testid"),
            left: Math.round(rect.left),
            matchesPrivacyAdmin: element.matches(".privacy-admin"),
            right: Math.round(rect.right),
            tagName: element.tagName,
            width: Math.round(rect.width),
          },
        ];
      });
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

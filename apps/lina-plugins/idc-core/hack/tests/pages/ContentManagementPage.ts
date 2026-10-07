import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class ContentManagementPage {
  readonly fileInput: Locator;
  readonly inspectionUnavailable: Locator;
  readonly page: Page;
  readonly pageRoot: Locator;
  readonly policySummary: Locator;
  readonly startUpload: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("content-management-page");
    this.fileInput = page.getByTestId("content-upload-file");
    this.inspectionUnavailable = page.getByTestId(
      "content-upload-inspection-unavailable",
    );
    this.policySummary = page.getByTestId("content-upload-policy");
    this.startUpload = page.getByTestId("content-upload-start");
  }

  async chooseFile(fileName: string, sizeBytes: number) {
    await this.fileInput.setInputFiles({
      buffer: Buffer.alloc(sizeBytes, "a"),
      mimeType: "application/octet-stream",
      name: fileName,
    });
  }

  async goto(contentNo: string) {
    await this.page.goto(workspacePath("/idc/content"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.page.getByTestId(`content-item-${contentNo}`).click();
    await this.fileInput.waitFor({ state: "visible" });
  }

  async screenshot(description: string) {
    await this.page.waitForTimeout(300);
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

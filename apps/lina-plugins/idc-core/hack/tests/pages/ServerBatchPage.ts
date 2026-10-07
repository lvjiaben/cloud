import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class ServerBatchPage {
  readonly pageRoot: Locator;

  constructor(readonly page: Page) {
    this.pageRoot = page.getByTestId("server-batch-page");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/resource/server-batches"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
  }

  async preview(serviceLines: string) {
    await this.page.getByTestId("server-batch-services").fill(serviceLines);
    await this.page.getByTestId("server-batch-preview").click();
    await this.page.getByTestId("server-batch-preview-panel").waitFor({
      state: "visible",
    });
  }

  async execute() {
    await this.page.getByTestId("server-batch-acknowledge").check();
    await this.page.getByTestId("server-batch-execute").click();
  }

  async screenshot(description: string, target?: Locator) {
    await this.page.waitForTimeout(300);
    const now = new Date();
    const directory = resolve(
      process.cwd(),
      "../../temp",
      now.toISOString().slice(0, 10).replaceAll("-", ""),
    );
    await mkdir(directory, { recursive: true });
    const path = resolve(
      directory,
      `${now.toISOString().slice(11, 19).replaceAll(":", "")}-${description}.png`,
    );
    if (target) {
      await target.screenshot({ path });
    } else {
      await this.page.screenshot({ fullPage: false, path });
    }
    return path;
  }
}

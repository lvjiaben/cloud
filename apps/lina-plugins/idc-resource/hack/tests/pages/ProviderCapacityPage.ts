import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import {
  waitForDialogReady,
  waitForRouteReady,
  waitForTableReady,
} from "../../../../../../hack/tests/support/ui";

export class ProviderCapacityPage {
  readonly nextButton: Locator;
  readonly page: Page;
  readonly pageIndicator: Locator;
  readonly pageRoot: Locator;
  readonly previousButton: Locator;
  readonly syncModal: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("provider-capacity-operations-page");
    this.pageIndicator = this.pageRoot.locator(".vxe-pager--num-btn.is--active");
    this.previousButton = this.pageRoot.locator(".vxe-pager--prev-btn");
    this.nextButton = this.pageRoot.locator(".vxe-pager--next-btn");
    this.syncModal = page.getByTestId("provider-capacity-sync-modal");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/resource/provider-capacity"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.page
      .getByText(/加载菜单中|Loading menu/iu)
      .waitFor({ state: "hidden", timeout: 15_000 });
    await waitForTableReady(this.page);
  }

  operationRow(visibleLabel: string) {
    return this.pageRoot
      .locator("tr")
      .filter({ hasText: visibleLabel })
      .first();
  }

  operationStatus(observationNo: string) {
    return this.page.getByTestId(`provider-capacity-status-${observationNo}`);
  }

  async nextPage() {
    await this.nextButton.click();
    await waitForTableReady(this.page);
  }

  async previousPage() {
    await this.previousButton.click();
    await waitForTableReady(this.page);
  }

  async selectFilter(testID: string, optionName: RegExp | string) {
    await this.page.getByTestId(testID).click();
    await this.page
      .locator(".ant-select-dropdown:visible .ant-select-item-option-content")
      .getByText(optionName, { exact: true })
      .click();
    await this.page.getByRole("button", { name: /搜\s*索|Search/iu }).click();
    await waitForTableReady(this.page);
  }

  async openSync(channelNo?: string) {
    if (channelNo) {
      await this.page
        .getByTestId(`provider-capacity-sync-channel-${channelNo}`)
        .click();
    } else {
      await this.page.getByTestId("provider-capacity-sync-open").click();
    }
    await waitForDialogReady(this.page.getByRole("dialog"));
    await this.syncModal.waitFor({ state: "visible" });
  }

  async selectSyncKind(name: RegExp | string) {
    await this.syncModal.getByText(name, { exact: true }).click();
  }

  async selectSyncChannel(channelName: string) {
    await this.page.getByTestId("provider-capacity-sync-channel").click();
    await this.page
      .locator(".ant-select-dropdown:visible .ant-select-item-option-content")
      .getByText(channelName, { exact: true })
      .click();
  }

  async confirmSync() {
    await this.page
      .getByRole("dialog")
      .getByRole("button", { name: /开始更新|Start Update/iu })
      .click();
    await this.syncModal.waitFor({ state: "hidden" });
    await waitForTableReady(this.page);
  }

  async screenshot(name: string) {
    const directory = resolve(
      process.cwd(),
      "temp",
      "e2e-artifacts",
      new Date().toISOString().slice(0, 10),
      "idc-resource",
    );
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({
      fullPage: true,
      path: resolve(directory, `${name}.png`),
    });
  }
}

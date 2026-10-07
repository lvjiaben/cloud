import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class ServerAccessoriesPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("server-accessory-management-page");
  }

  async goto(serviceNo = "svc-10001") {
    await this.page.goto(workspacePath("/idc/resource/server-accessories"), {
      waitUntil: "domcontentloaded",
    });
    await this.root.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.page
      .getByTestId(`server-service-manage-${serviceNo}`)
      .waitFor({ state: "visible" });
    await this.page.getByTestId(`server-service-manage-${serviceNo}`).click();
    await this.page.getByTestId("server-accessory-service-no").waitFor({
      state: "visible",
    });
  }

  action(action: string) {
    return this.page.getByTestId(`staff-accessory-action-${action}`);
  }

  get trafficPackages() {
    return this.page.getByTestId("staff-traffic-package-purchase");
  }

  async selectTrafficPackage() {
    await this.action("traffic_add").click();
    await this.trafficPackages.waitFor({ state: "visible" });
    await this.trafficPackages.locator("tr", { hasText: "固定100GB" }).getByRole("button", { name: "为客户购买", exact: true }).click();
  }

  async gotoManagement() {
    await this.page.goto(workspacePath("/idc/resource/server-accessories"), {
      waitUntil: "domcontentloaded",
    });
    await this.root.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.page.getByTestId("server-service-manage-svc-10001").waitFor({
      state: "visible",
    });
  }

  async selectService(serviceNo: string) {
    await this.root
      .locator("tr", { hasText: serviceNo })
      .locator(".ant-checkbox-input")
      .click();
  }

  async openBatch() {
    await this.page.getByTestId("server-batch-open").click();
    await this.page
      .getByRole("dialog")
      .filter({ hasText: "服务器批量操作" })
      .waitFor({ state: "visible" });
  }

  async screenshot(name: string) {
    const now = new Date();
    const directory = resolve(
      process.cwd(),
      "../../temp",
      now.toISOString().slice(0, 10).replaceAll("-", ""),
    );
    await mkdir(directory, { recursive: true });
    await this.page.screenshot({
      fullPage: false,
      path: resolve(
        directory,
        `${now.toISOString().slice(11, 19).replaceAll(":", "")}-${name}.png`,
      ),
    });
  }
}

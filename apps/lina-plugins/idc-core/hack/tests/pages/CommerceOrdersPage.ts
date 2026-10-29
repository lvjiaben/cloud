import {fileURLToPath} from 'node:url';
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class CommerceOrdersPage {
  readonly page: Page;
  readonly pageRoot: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("commerce-order-page");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/orders"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
  }

  row(orderNo: string) {
    return this.pageRoot.locator(`.vxe-table--main-wrapper tr[rowid="${orderNo}"]`);
  }

  async openDetail(orderNo: string) {
    await this.pageRoot.locator(`tr[rowid="${orderNo}"]`).getByRole("button", { name: /详情|Details/u }).click();
    await this.page.getByRole("dialog").waitFor({ state: "visible" });
  }

  async resize(width: number, height = 900) {
    await this.page.setViewportSize({ width, height });
  }

  async layoutMetrics() {
    return this.pageRoot.evaluate((root) => {
      const grid = root.querySelector<HTMLElement>(".vxe-grid")!;
      const body = root.querySelector<HTMLElement>(".vxe-table--main-wrapper .vxe-table--body-wrapper")!;
      const horizontalScroll = root.querySelector<HTMLElement>(".vxe-table--scroll-x-handle")!;
      const fixedAction = root.querySelector<HTMLElement>(".vxe-table--fixed-right-wrapper")!;
      const pager = root.querySelector<HTMLElement>(".vxe-pager")!;
      const bodyRect = body.getBoundingClientRect();
      return {
        viewport: window.innerWidth,
        viewportHeight: window.innerHeight,
        rootBottom: root.getBoundingClientRect().bottom,
        gridBottom: grid.getBoundingClientRect().bottom,
        pagerBottom: pager.getBoundingClientRect().bottom,
        rootRight: root.getBoundingClientRect().right,
        gridRight: grid.getBoundingClientRect().right,
        gridWidth: grid.getBoundingClientRect().width,
        tableWidth: horizontalScroll.scrollWidth,
        tableViewportWidth: horizontalScroll.clientWidth,
        actionRight: fixedAction?.getBoundingClientRect().right,
        pagerRight: pager.getBoundingClientRect().right,
        bodyHeight: bodyRect.height,
        visibleRows: Array.from(body.querySelectorAll(".vxe-body--row")).filter((row) => {
          const rect = row.getBoundingClientRect();
          return rect.top >= bodyRect.top && rect.bottom <= bodyRect.bottom;
        }).length,
      };
    });
  }

  async scrollTableToEnd() {
    await this.pageRoot.locator(".vxe-table--scroll-x-handle").evaluate((body) => {
      body.scrollLeft = body.scrollWidth;
    });
  }

  async scrollTableToStart() {
    await this.pageRoot.locator(".vxe-table--scroll-x-handle").evaluate((body) => {
      body.scrollLeft = 0;
    });
  }

  async showPagination() {
    await this.pageRoot.locator(".vxe-pager").scrollIntoViewIfNeeded();
  }

  get nextPageAction() {
    return this.pageRoot.locator(".vxe-pager--next-btn");
  }

  get previousPageAction() {
    return this.pageRoot.locator(".vxe-pager--prev-btn");
  }

  get fixedDetailAction() {
    return this.pageRoot.locator(".vxe-table--fixed-right-wrapper").getByRole("button", { name: "详情" }).first();
  }

  async nextPage() {
    await this.nextPageAction.click();
  }

  async pageSize(size: number) {
    await this.pageRoot.locator(".vxe-pager .vxe-select").click();
    await this.page.locator(".vxe-select-option").filter({
      hasText: new RegExp(`^\\s*${size}\\s*(条/页|/page)\\s*$`, "u"),
    }).click();
  }

  async lastPage() {
    await this.pageRoot.locator(".vxe-pager--end-btn").click();
  }

  async previousPage() {
    await this.previousPageAction.click();
  }

  async search() {
    await this.pageRoot.getByRole("button", { name: /搜\s*索/u }).click();
  }

  get optionalContractSection() {
    return this.page.getByTestId("optional-contract-section");
  }

  async screenshot(description: string) {
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(fileURLToPath(new URL("../../../../../../temp/",import.meta.url)), day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ animations: "disabled", fullPage: false, path });
    return path;
  }
}

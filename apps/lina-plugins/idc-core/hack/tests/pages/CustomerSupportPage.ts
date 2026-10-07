import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { MainLayout } from "@host-tests/pages/MainLayout";

export class CustomerSupportPage {
  readonly page: Page;
  readonly root: Locator;
  readonly rows: Locator;
  readonly pager: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("customer-support-page");
    this.rows = this.root.locator(".vxe-table--main-wrapper .vxe-body--row");
    this.pager = this.root.locator(".vxe-pager");
  }

  async goto() {
    const response = this.listResponse();
    await this.page.goto("http://127.0.0.1:6032/account/support", {
      waitUntil: "domcontentloaded",
    });
    await this.root.waitFor({ state: "visible" });
    return (await response).json();
  }

  private listResponse() {
    return this.page.waitForResponse(
      (response) =>
        /\/account\/support\/tickets(?:\?|$)/u.test(response.url()) &&
        response.request().method() === "GET",
    );
  }

  async search(query: string) {
    await this.page.getByTestId("ticket-search-query").fill(query);
    const response = this.listResponse();
    await this.root
      .getByRole("button", { name: /搜\s*索|Search/u, exact: true })
      .first()
      .click();
    return response;
  }

  async reset() {
    const response = this.listResponse();
    await this.root
      .getByRole("button", { name: /重\s*置|Reset/u, exact: true })
      .click();
    return response;
  }

  async nextPage() {
    const response = this.listResponse();
    await this.pager.locator(".vxe-pager--next-btn").click();
    return response;
  }

  async previousPage() {
    const response = this.listResponse();
    await this.pager.locator(".vxe-pager--prev-btn").click();
    return response;
  }

  async retry() {
    const response = this.listResponse();
    await this.root.getByRole("button", { name: /重\s*试/u }).click();
    return response;
  }

  async goToPage(number: number) {
    const response = this.listResponse();
    await this.pager.locator(".vxe-pager--num-btn").filter({ hasText: new RegExp(`^${number}$`, "u") }).click();
    return response;
  }

  async changePageSize(size: number) {
    await this.pager.locator(".vxe-select").click();
    const response = this.listResponse();
    await this.page.locator(".vxe-select-option").filter({ hasText: new RegExp(`^${size}\\s*(条/页|/page)$`, "u") }).click();
    return response;
  }

  activePage() {
    return this.pager.locator(".vxe-pager--num-btn.is--active");
  }

  async openDetail(ticketNo: string) {
    await this.root.getByTestId(`ticket-detail-${ticketNo}`).last().click();
    await this.page.waitForURL(`**/account/support/${ticketNo}`);
  }

  async openCreate() {
    await this.page.getByTestId("new-ticket-action").click();
    const drawer = this.page.getByRole("dialog", { name: "提交新工单" });
    await drawer.waitFor({ state: "visible" });
    await drawer.evaluate(async (element) => {
      await Promise.all(
        element
          .getAnimations({ subtree: true })
          .map((animation) => animation.finished.catch(() => undefined)),
      );
    });
    return drawer;
  }

  async closeCreate() {
    await this.page
      .getByRole("dialog", { name: "提交新工单" })
      .getByRole("button", { name: "返回工单列表" })
      .click();
    await this.page
      .getByRole("dialog", { name: "提交新工单" })
      .waitFor({ state: "hidden" });
  }

  async useEnglish() {
    await new MainLayout(this.page).switchLanguage("English");
  }

  async resizeMobile() {
    await this.page.setViewportSize({ width: 390, height: 844 });
  }

  async geometry() {
    return this.root.evaluate((element) => {
      const body = element.querySelector(
        ".vxe-table--main-wrapper .vxe-table--body-wrapper",
      );
      const pager = element.querySelector('.vxe-pager');
      return {
        bodyHeight: body?.getBoundingClientRect().height ?? 0,
        pageOverflow: document.documentElement.scrollWidth - window.innerWidth,
        pagerBottom: pager?.getBoundingClientRect().bottom ?? 0,
        viewportHeight: window.innerHeight,
      };
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

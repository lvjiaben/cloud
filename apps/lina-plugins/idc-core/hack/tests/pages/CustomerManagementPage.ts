import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";

export class CustomerManagementPage {
  readonly nextButton: Locator;
  readonly overviewDrawer: Locator;
  readonly page: Page;
  readonly pageIndicator: Locator;
  readonly pageRoot: Locator;
  readonly previousButton: Locator;

  constructor(page: Page) {
    this.page = page;
    this.overviewDrawer = page.getByTestId("customer-overview-drawer");
    this.pageRoot = page.getByTestId("customer-management-page");
    this.pageIndicator = this.pageRoot.locator(".vxe-pager--num-btn.is--active");
    this.previousButton = this.pageRoot.locator(".vxe-pager--prev-btn");
    this.nextButton = this.pageRoot.locator(".vxe-pager--next-btn");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/customers"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
  }

  customerRow(customerNo: string) {
    return this.pageRoot.locator(
      `.vxe-table--main-wrapper tr.vxe-body--row[rowid="${customerNo}"]`,
    );
  }

  async searchNamePrefix(prefix: string) {
    await this.page.getByTestId("customer-search-name").fill(prefix);
    await this.page.getByRole("button", { name: /搜\s*索/u }).click();
  }

  async selectSegment(kind: "group" | "tag", name: string) {
    await this.page.getByTestId(`customer-search-${kind}`).click();
    const option = this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(name, { exact: true });
    await option.waitFor({ state: "visible" });
    await option.click();
    await this.page.getByRole("button", { name: /搜\s*索/u }).click();
  }

  async nextPage() {
    await this.nextButton.click();
  }

  async previousPage() {
    await this.previousButton.click();
  }

  async openOverview(customerNo: string) {
    await this.page
      .locator(`[data-testid="customer-overview-${customerNo}"]:visible`)
      .click();
    await this.overviewDrawer.waitFor({
      state: "visible",
    });
  }

  async openOverviewByPublicId(publicId: string) {
    const rows = this.pageRoot.locator(
      ".vxe-table--main-wrapper .vxe-table--body-wrapper tr.vxe-body--row",
    );
    await rows.first().waitFor({ state: "visible" });
    const rowIndex = await rows.evaluateAll(
      (elements, expected) =>
        elements.findIndex((element) =>
          element.textContent?.includes(String(expected)),
        ),
      publicId,
    );
    if (rowIndex < 0) {
      throw new Error(
        `customer ${publicId} is not visible in the current page`,
      );
    }
    await this.pageRoot
      .locator('[data-testid^="customer-overview-"]:visible')
      .nth(rowIndex)
      .click();
    await this.overviewDrawer.waitFor({ state: "visible" });
  }

  async reopenStatusEditor() {
    const button = this.overviewDrawer.getByTestId("customer-status-open");
    for (let attempt = 0; attempt < 2; attempt += 1) {
      await button.click();
      await this.page.waitForFunction(
        () =>
          document.querySelectorAll('[role="dialog"][data-state="open"]')
            .length >= 2,
      );
      await this.page.keyboard.press("Escape");
      await this.page.waitForFunction(
        () =>
          document.querySelectorAll('[role="dialog"][data-state="open"]')
            .length === 1,
      );
    }
  }

  async reopenActivityDialog(label: string) {
    const button = this.overviewDrawer.getByRole("button", {
      exact: true,
      name: label,
    });
    for (let attempt = 0; attempt < 2; attempt += 1) {
      await button.click();
      await this.page.waitForFunction(
        () =>
          document.querySelectorAll('[role="dialog"][data-state="open"]')
            .length >= 2,
      );
      await this.page.keyboard.press("Escape");
      await this.page.waitForFunction(
        () =>
          document.querySelectorAll('[role="dialog"][data-state="open"]')
            .length === 1,
      );
    }
  }

  private blacklistColumn() {
    return this.overviewDrawer.locator(".history-column").nth(2);
  }

  async submitBlacklist() {
    await this.overviewDrawer
      .getByRole("button", { exact: true, name: "提交黑名单" })
      .click();
    const dialog = this.page.getByRole("dialog").last();
    const responsePromise = this.page.waitForResponse(
      (response) =>
        /\/blacklist$/u.test(new URL(response.url()).pathname) &&
        response.request().method() === "POST",
    );
    await dialog.getByRole("button", { name: /确\s*(?:定|认)/u }).click();
    const response = await responsePromise;
    if (!response.ok()) {
      throw new Error(`blacklist submission failed: ${response.status()}`);
    }
    await this.page.waitForFunction(
      () =>
        document.querySelectorAll('[role="dialog"][data-state="open"]')
          .length === 1,
    );
    await this.blacklistColumn()
      .locator(".history-item")
      .first()
      .getByText("待审批", { exact: true })
      .waitFor({ state: "visible" });
  }

  async decideLatestBlacklist(
    actionLabel: "批准" | "撤销" | "驳回",
    expectedStatus: "已撤销" | "已生效" | "已驳回",
  ) {
    const latest = this.blacklistColumn().locator(".history-item").first();
    await latest
      .getByRole("button", { exact: true, name: actionLabel })
      .click();
    const dialog = this.page.getByRole("dialog").last();
    const responsePromise = this.page.waitForResponse(
      (response) =>
        response.url().includes("/blacklist/") &&
        response.url().endsWith("/decision") &&
        response.request().method() === "POST",
    );
    await dialog.getByRole("button", { name: /确\s*(?:定|认)/u }).click();
    const response = await responsePromise;
    if (!response.ok()) {
      throw new Error(`blacklist decision failed: ${response.status()}`);
    }
    await latest
      .getByText(expectedStatus, { exact: true })
      .waitFor({ state: "visible" });
  }

  async showActivity() {
    await this.overviewDrawer.getByRole("tab", { name: /运营记录/u }).click();
  }

  async startAssistance(reason: string) {
    await this.overviewDrawer.getByTestId("customer-assistance-open").click();
    const dialog = this.page.getByRole("dialog").last();
    await dialog.getByTestId("customer-assistance-reason").fill(reason);
    await dialog.getByTestId("customer-assistance-confirm").check();
    const popupPromise = this.page.context().waitForEvent("page");
    const responsePromise = this.page.waitForResponse(
      (response) =>
        response.url().includes("/assistance-sessions") &&
        response.request().method() === "POST",
    );
    await dialog.getByRole("button", { name: /确\s*认/u }).click();
    const response = await responsePromise;
    if (!response.ok()) {
      throw new Error(
        `customer assistance request failed: ${response.status()}`,
      );
    }
    return popupPromise;
  }

  async selectCustomer(customerNo: string) {
    await this.customerRow(customerNo)
      .locator("td.col--checkbox .vxe-cell--checkbox")
      .click();
  }

  async openTagBatch() {
    await this.page.getByTestId("customer-tag-batch-open").click();
    await this.page.getByRole("dialog").waitFor({ state: "visible" });
  }

  async openCustomerActivation() {
    await this.page.getByTestId("customer-activation-open").click();
    await this.page.getByRole("dialog").waitFor({ state: "visible" });
  }

  async openDirectCustomer() {
    await this.page.getByTestId("customer-direct-open").click();
    await this.page.getByRole("dialog").waitFor({ state: "visible" });
  }

  async openPasswordReset(customerNo: string) {
    await this.page
      .getByTestId(`customer-password-reset-${customerNo}`)
      .click();
    await this.page.getByRole("dialog").waitFor({ state: "visible" });
  }

  async resetCustomerPassword(input: { password: string; reason: string }) {
    const dialog = this.page.getByRole("dialog");
    await dialog
      .getByTestId("customer-password-reset-password")
      .fill(input.password);
    await dialog
      .getByTestId("customer-password-reset-reason")
      .fill(input.reason);
    await dialog.getByRole("button", { name: /确\s*(?:定|认)/u }).click();
  }

  async createDirectCustomer(input: {
    displayName: string;
    email: string;
    password: string;
    reason: string;
  }) {
    const dialog = this.page.getByRole("dialog");
    await dialog.getByTestId("customer-direct-name").fill(input.displayName);
    await dialog.getByTestId("customer-direct-email").fill(input.email);
    await dialog.getByTestId("customer-direct-password").fill(input.password);
    await dialog.getByTestId("customer-direct-reason").fill(input.reason);
    await dialog.getByRole("button", { name: /确\s*(?:定|认)/u }).click();
  }

  async fillCustomerActivation(input: {
    bindExisting?: boolean;
    customerType: "enterprise" | "individual";
    destination: string;
    destinationKind?: "email" | "sms";
    displayName: string;
    reason: string;
  }) {
    const dialog = this.page.getByRole("dialog");
    await dialog.getByTestId("customer-activation-type").click();
    await this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(input.customerType === "enterprise" ? "企业" : "个人", {
        exact: true,
      })
      .click();
    await dialog
      .getByTestId("customer-activation-name")
      .fill(input.displayName);
    if (input.destinationKind === "sms") {
      await dialog
        .getByTestId("customer-activation-destination-kind")
        .getByText("短信", { exact: true })
        .click();
      await dialog
        .getByTestId("customer-activation-phone")
        .fill(input.destination);
    } else {
      await dialog
        .getByTestId("customer-activation-email")
        .fill(input.destination);
    }
    await dialog.getByTestId("customer-activation-reason").fill(input.reason);
    if (input.bindExisting) {
      await dialog.getByTestId("customer-activation-bind-existing").click();
    }
  }

  async submitCustomerActivation() {
    await this.page
      .getByRole("dialog")
      .getByRole("button", { name: /确\s*(?:定|认)/u })
      .click();
  }

  async screenshot(description: string) {
    await this.page.getByText("加载菜单中...").waitFor({ state: "hidden" });
    await this.pageRoot.evaluate(async (root) => {
      const animations = root
        .getAnimations({ subtree: true })
        .filter((animation) => {
          const endTime = animation.effect?.getComputedTiming().endTime;
          return typeof endTime === "number" && Number.isFinite(endTime);
        });
      await Promise.all(
        animations.map((animation) =>
          animation.finished.catch(() => undefined),
        ),
      );
    });
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

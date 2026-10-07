import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class SupportDeskPage {
  readonly page: Page;
  readonly pageRoot: Locator;
  readonly settingsRoot: Locator;

  constructor(page: Page) {
    this.page = page;
    this.pageRoot = page.getByTestId("support-desk-page");
    this.settingsRoot = page.getByTestId("support-settings-page");
  }

  async gotoSettings() {
    await this.page.goto(workspacePath("/idc/support/settings"), {
      waitUntil: "domcontentloaded",
    });
    await this.settingsRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.page
      .getByText(/加载菜单中|Loading menu/iu)
      .waitFor({ state: "hidden", timeout: 15_000 });
    await this.settingsRoot
      .getByText(/正在加载工单设置|Loading support settings/iu)
      .waitFor({ state: "hidden", timeout: 15_000 });
  }

  async openSettingsSection(section: "automation" | "fields" | "routing" | "sla" | "writing") {
    await this.page.getByTestId(`support-settings-tab-${section}`).click();
    await this.page.getByTestId(`support-settings-${section}`).waitFor({
      state: "visible",
    });
  }

  async openInternalTicketForm() {
    await this.page.getByTestId("support-create").click();
    await this.page.getByTestId("support-create-modal").waitFor({
      state: "visible",
    });
    await this.page
      .getByText(/正在加载工单工作台|Loading support desk/iu)
      .waitFor({ state: "hidden", timeout: 15_000 });
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/support"), {
      waitUntil: "domcontentloaded",
    });
    await this.pageRoot.waitFor({ state: "visible" });
    await waitForRouteReady(this.page, 15_000);
    await this.page
      .getByText(/加载菜单中|Loading menu/iu)
      .waitFor({ state: "hidden", timeout: 15_000 });
  }

  async openTicket(ticketNo: string) {
    await this.page.getByTestId(`support-ticket-${ticketNo}`).click();
    await this.pageRoot
      .locator(".ticket-detail .detail-heading")
      .getByText(ticketNo, { exact: true })
      .waitFor({ state: "visible" });
  }

  async merge(candidateTicketNo: string, reason: string, global: boolean) {
    await this.page
      .getByTestId(global ? "support-action-global-merge" : "support-action-merge")
      .click();
    await this.page
      .getByTestId(`support-merge-candidate-${candidateTicketNo}`)
      .check();
    await this.page
      .getByTestId("support-action-composer")
      .getByRole("textbox", { name: /原因|Reason/iu })
      .fill(reason);
    await this.page.getByTestId("support-action-submit").click();
  }

  async openStatusTransition() {
    await this.page.getByTestId("support-action-status-transition").click();
    await this.page
      .getByTestId("support-status-transition-target")
      .waitFor({ state: "visible" });
  }

  async selectStatusTransition(statusNo: string) {
    await this.page
      .getByTestId("support-status-transition-target")
      .selectOption(statusNo);
  }

  async fillActionReason(reason: string) {
    await this.page
      .getByTestId("support-action-composer")
      .getByRole("textbox", { name: /原因|Reason/iu })
      .fill(reason);
  }

  async submitAction() {
    await this.page.getByTestId("support-action-submit").click();
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

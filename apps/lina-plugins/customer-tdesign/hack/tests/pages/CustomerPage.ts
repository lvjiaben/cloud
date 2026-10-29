import type { Page } from "../support/fixture";
import { mkdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
export class CustomerPage {
  constructor(readonly page: Page) {}
  open(path = "/account") {
    return this.page.goto(path);
  }
  get main() {
    return this.page.locator("main,.main-content").first();
  }
  async openAnnouncements() {
    await this.button("公告").click();
  }
  get announcements() {
    return this.page.getByTestId("announcement-list");
  }
  get announcementDetail() {
    return this.page.getByTestId("announcement-detail");
  }
  async openAnnouncement(title: string) {
    await this.announcements
      .getByRole("button")
      .filter({ hasText: title })
      .click();
  }
  async closeAnnouncement() {
    await this.announcementDetail
      .locator('xpath=ancestor::*[@role="dialog"]')
      .getByRole("button", { name: "关闭弹窗" })
      .click();
  }
  async nextAnnouncementsPage() {
    await this.page.locator(".t-pagination__btn-next").click();
  }
  async reload() {
    await this.page.reload();
  }
  button(name: string) {
    return this.page.getByRole("button", { name, exact: true });
  }
  async pluginButton(pluginId: string, name: string) {
    await this.pluginBody(pluginId)
      .getByRole("button", { name, exact: true })
      .click();
  }
  async navigateMenu(name: string) {
    await this.page
      .getByRole("navigation")
      .getByRole("link", { name, exact: true })
      .click();
  }
  get serviceSearch() {
    return this.serviceBody.getByRole("button", { name: /搜\s*索/ });
  }
  get serviceNoteDialog() {
    return this.serviceBody.getByRole("dialog");
  }
  async searchServices(keyword: string) {
    await this.serviceBody
      .getByRole("textbox", { name: "搜索服务", exact: true })
      .fill(keyword);
    await this.serviceBody.getByRole("button", { name: /搜\s*索/ }).click();
  }
  async openServiceNote(name: string) {
    await this.serviceBody
      .getByRole("button", { name: `编辑备注 ${name}`, exact: true })
      .click();
  }
  async saveServiceNote() {
    await this.serviceNoteDialog
      .getByRole("button", { name: /确\s*认/ })
      .click();
  }
  pluginBody(pluginId: string) {
    return this.page
      .frameLocator(`section[data-plugin="${pluginId}"] iframe`)
      .locator("body");
  }
  serviceActionRows(id: string) {
    return this.serviceBody.getByRole("row").filter({ hasText: id });
  }
  get serviceRows() {
    return this.serviceBody
      .locator(".vxe-table--main-wrapper")
      .getByRole("row");
  }
  get serviceBody() {
    return this.pluginBody("idc-resource");
  }
  async reloadServices() {
    await this.serviceBody.getByRole("button", { name: /搜\s*索/ }).click();
  }
  async nextServicesPage() {
    await this.serviceBody.locator(".vxe-pager--next-btn").click();
  }
  get serviceTabs() {
    return this.main.getByRole("tablist", { name: "服务分类" });
  }
  async selectServiceCategory(name: string) {
    await this.serviceTabs.getByRole("tab", { name, exact: true }).click();
  }
  async openServiceChoices() {
    await this.page.locator(".form-card .t-select").nth(1).click();
  }
  async searchServiceChoices(keyword: string) {
    await this.page.locator(".form-card .t-select input").nth(1).fill(keyword);
  }
  async scrollChoicesToBottom() {
    const popup = this.page.locator(".t-select__list:visible");
    await popup.locator(".t-select-option").last().scrollIntoViewIfNeeded();
  }
  async attachTicketFiles(names: string[]) {
    await this.page
      .locator('.ticket-attachments input[type="file"]')
      .setInputFiles(
        names.map((name) => ({
          name,
          mimeType: "text/plain",
          buffer: Buffer.from("diagnostic data"),
        })),
      );
  }
  async removeTicketFile(name: string) {
    await this.page
      .locator(".t-upload__single-display-text")
      .filter({ hasText: name })
      .locator(".t-upload__icon-delete")
      .click();
  }
  async screenshot(
    step: string,
    evidenceDate = new Date().toISOString().slice(0, 10).replaceAll("-", ""),
  ) {
    const dir = resolve(
      fileURLToPath(new URL("../../../../../../temp/", import.meta.url)),
      evidenceDate,
    );
    await mkdir(dir, { recursive: true });
    const path = resolve(
      dir,
      `${new Date().toISOString().slice(11, 23).replace(/[:.]/g, "")}-tdesign-${step}.png`,
    );
    await this.page.screenshot({
      path,
      fullPage: false,
      animations: "disabled",
    });
    return path;
  }
}

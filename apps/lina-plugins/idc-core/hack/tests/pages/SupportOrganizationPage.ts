import type { Page } from "@host-tests/support/playwright";
import { SupportDeskPage } from "./SupportDeskPage";
export class SupportOrganizationPage extends SupportDeskPage {
  constructor(page: Page) {
    super(page);
  }
  form() {
    return this.page.getByTestId("support-settings-department-form");
  }
  async showDepartmentForm() {
    await this.form()
      .getByRole("button", { name: "保存设置", exact: true })
      .scrollIntoViewIfNeeded();
  }
  async showFollowUpHistory(reason: string) {
    await this.pageRoot
      .getByText(reason, { exact: true })
      .scrollIntoViewIfNeeded();
  }
  async edit(number: string) {
    await this.page
      .getByTestId(`support-settings-edit-department-${number}`)
      .click();
  }
  async save() {
    await this.form()
      .getByRole("button", { name: "保存设置", exact: true })
      .click();
  }
  async openOrganizations() {
    await this.form().getByTestId("support-organization-select").click();
  }
  dropdown() {
    return this.page.locator(".ant-select-dropdown:visible");
  }
  async search(value: string) {
    await this.form().getByRole("combobox").first().fill(value);
  }
  async choose(name: string) {
    await this.dropdown()
      .locator(".ant-select-item-option-content")
      .getByText(name, { exact: true })
      .click();
  }
  async nextPage() {
    await this.dropdown().locator(".ant-pagination-next").click();
  }
  async retryCandidates() {
    await this.dropdown()
      .getByRole("button", { name: /^重\s*试$/u })
      .click();
  }
  async names(zh: string, en: string) {
    await this.form()
      .getByRole("textbox", { name: "中文名称", exact: true })
      .fill(zh);
    await this.form()
      .getByRole("textbox", { name: "英文名称", exact: true })
      .fill(en);
  }
  async transfer() {
    await this.page.getByTestId("support-action-transfer").click();
  }
  async followUp() {
    await this.page.getByTestId("support-action-escalate").click();
  }
  async submitFollowUp(reason: string) {
    await this.page
      .getByTestId("support-action-composer")
      .getByRole("textbox", { name: "原因" })
      .fill(reason);
    await this.page.getByTestId("support-action-submit").click();
  }
  async filterFollowUp() {
    await this.page.getByRole("button", { name: /本页 · 待跟进/u }).click();
  }
  async transferTo(department: string, reason: string) {
    const form = this.page.getByTestId("support-action-composer");
    await form
      .getByLabel("部门", { exact: true })
      .selectOption(department, { timeout: 10000 });
    await form.getByRole("textbox", { name: "原因" }).fill(reason);
    await this.page.getByTestId("support-action-submit").click();
  }
}

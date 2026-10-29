import type { Page } from "@host-tests/support/playwright";
export class CustomerAnnouncementsPage {
  constructor(readonly page: Page) {}
  get list() { return this.page.getByTestId("announcement-list"); }
  get detail() { return this.page.getByTestId("announcement-detail"); }
  open(name = "公告") { return this.page.getByRole("button", { name, exact: true }).click(); }
  select(title: string) { return this.list.getByRole("button").filter({ hasText: title }).click(); }
  closeDetail() { return this.detail.locator('xpath=ancestor::*[@role="dialog"]').getByRole("button").last().click(); }
  closeList() { return this.list.locator('xpath=ancestor::*[@role="dialog"]').getByRole("button").last().click(); }
  nextPage() { return this.page.locator(".ant-pagination-next").click(); }
  retry() { return this.page.getByRole("button", { name: /重\s*试/, exact: true }).click(); }
}

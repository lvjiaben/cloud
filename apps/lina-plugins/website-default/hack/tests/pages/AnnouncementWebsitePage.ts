import type { Page } from '@host-tests/support/playwright';
export class AnnouncementWebsitePage {
  constructor(readonly page: Page, readonly origin: string) {}
  open() { return this.page.goto(this.origin + '/pages/announcements'); }
  select(title: string) { return this.page.getByTestId('public-content-list').getByRole('link').filter({ hasText: title }).click(); }
  next() { return this.page.getByRole('button', { name: '下一页', exact: true }).click(); }
}

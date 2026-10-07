import type { Locator, Page } from '@playwright/test';

export class PublicContentPageObject {
  readonly detail: Locator;
  readonly list: Locator;
  readonly page: Page;
  readonly shell: Locator;

  constructor(page: Page) {
    this.page = page;
    this.shell = page.getByTestId('public-content-shell');
    this.list = page.getByTestId('public-content-list');
    this.detail = page.getByTestId('public-content-detail');
  }

  async goto(path: string) {
    await this.page.goto(path, { waitUntil: 'domcontentloaded' });
    await this.shell.waitFor({ state: 'visible' });
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
  }
}


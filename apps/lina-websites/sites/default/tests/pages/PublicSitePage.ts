import type { Locator, Page } from '@playwright/test';

import { testRuntime } from '../support/runtime-config';

export class PublicSitePage {
  readonly customerCenter: Locator;
  readonly document: Locator;
  readonly home: Locator;
  readonly localeSwitch: Locator;
  readonly page: Page;
  readonly shell: Locator;
  readonly signIn: Locator;

  constructor(page: Page) {
    this.page = page;
    this.shell = page.getByTestId('public-site-shell');
    this.home = page.getByTestId('public-home');
    this.document = page.locator('html');
    this.localeSwitch = page.getByTestId('public-locale-switch');
    this.signIn = page.getByTestId('public-sign-in');
    this.customerCenter = page.getByTestId('public-customer-center');
  }

  async goto(path = '/') {
    await this.page.goto(new URL(path, testRuntime.baseURL).toString(), {
      waitUntil: 'domcontentloaded',
    });
    await this.shell.waitFor({ state: 'visible' });
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
  }
}

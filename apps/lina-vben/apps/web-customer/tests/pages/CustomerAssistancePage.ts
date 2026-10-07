import type { Page } from '@playwright/test';

export class CustomerAssistancePage {
  constructor(readonly page: Page) {}

  get root() {
    return this.page.getByTestId('customer-assistance-page');
  }

  get countdown() {
    return this.page.getByTestId('customer-assistance-countdown');
  }

  async open(token = 'A'.repeat(43)) {
    await this.page.goto(`/account/assistance?token=${token}`);
    await this.page.locator('.ant-spin-spinning').waitFor({ state: 'detached' });
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () => document.documentElement.scrollWidth > window.innerWidth + 1,
    );
  }

  async hasClippedContent() {
    return this.root.evaluate((root) =>
      [...root.querySelectorAll<HTMLElement>('*')].some(
        (element) => element.scrollWidth > element.clientWidth + 1,
      ),
    );
  }
}

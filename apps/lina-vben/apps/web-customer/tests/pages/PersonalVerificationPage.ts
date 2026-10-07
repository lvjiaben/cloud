import type { Page } from '@playwright/test';

export class PersonalVerificationPage {
  constructor(readonly page: Page) {}

  get consent() {
    return this.page.getByTestId('identity-consent');
  }

  get name() {
    return this.page.getByTestId('identity-name');
  }

  get number() {
    return this.page.getByTestId('identity-number');
  }

  get root() {
    return this.page.getByTestId('identity-center');
  }

  get startButton() {
    return this.page.getByTestId('identity-start');
  }

  get status() {
    return this.page.getByTestId('identity-status');
  }

  async login() {
    await this.page.goto('/auth/login');
    const inputs = this.page.locator('input');
    await inputs.nth(0).fill('customer');
    await inputs.nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login' }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async open(path = '/account/identity') {
    await this.page.goto(path);
    await this.root.waitFor({ state: 'visible' });
  }

  async start(name: string, identityNumber: string) {
    await this.name.fill(name);
    await this.number.fill(identityNumber);
    await this.consent.check();
    await this.startButton.click();
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () =>
        document.documentElement.scrollWidth >
        document.documentElement.clientWidth,
    );
  }
}

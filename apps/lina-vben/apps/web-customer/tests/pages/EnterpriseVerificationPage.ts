import type { Page } from '@playwright/test';

export class EnterpriseVerificationPage {
  constructor(readonly page: Page) {}

  get beginButton() {
    return this.page.getByTestId('enterprise-begin');
  }

  get consent() {
    return this.page.getByTestId('enterprise-consent-checkbox');
  }

  get root() {
    return this.page.getByTestId('enterprise-verification-center');
  }

  get status() {
    return this.page.getByTestId('enterprise-status');
  }

  get submitButton() {
    return this.page.getByTestId('enterprise-submit');
  }

  async begin() {
    await this.consent.check();
    await this.beginButton.click();
    await this.page.getByTestId('enterprise-legal-form').waitFor();
  }

  async fillLegalDetails(
    values = {
      legalName: '测试科技有限公司',
      registrationIdentifier: '91350211M000100Y46',
      representativeIdentityNumber: '110101199001010015',
      representativeName: '测试法人',
    },
  ) {
    await this.page.getByTestId('enterprise-legal-name').fill(values.legalName);
    await this.page
      .getByTestId('enterprise-registration-id')
      .fill(values.registrationIdentifier);
    await this.page
      .getByTestId('enterprise-representative-name')
      .fill(values.representativeName);
    await this.page
      .getByTestId('enterprise-representative-id')
      .fill(values.representativeIdentityNumber);
  }

  async hasHorizontalOverflow() {
    return this.page.evaluate(
      () =>
        document.documentElement.scrollWidth >
        document.documentElement.clientWidth,
    );
  }

  async login() {
    await this.page.goto('/auth/login');
    const inputs = this.page.locator('input');
    await inputs.nth(0).fill('customer');
    await inputs.nth(1).fill('customer-password');
    await this.page.getByRole('button', { name: 'login' }).click();
    await this.page.waitForURL(/\/(?:account|profile)(?:$|\?)/u);
  }

  async open() {
    await this.page.goto('/account/enterprise-verification');
    await this.root.waitFor({ state: 'visible' });
  }

  async upload(type: string, body: string, name: string) {
    await this.page
      .getByTestId(`enterprise-upload-${type}`)
      .locator('input[type="file"]')
      .setInputFiles({
        buffer: Buffer.from(body),
        mimeType: 'application/pdf',
        name,
      });
    await this.page
      .getByTestId(`enterprise-evidence-${type}`)
      .getByText(/加密保存|Encrypted and ready/u)
      .waitFor();
  }
}

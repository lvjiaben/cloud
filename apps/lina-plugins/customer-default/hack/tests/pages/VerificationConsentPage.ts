import type { Page } from "@host-tests/support/playwright";

export class VerificationConsentPage {
  constructor(readonly page: Page) {}
  get openButton() { return this.page.getByTestId("verification-consent-open"); }
  get panel() { return this.page.getByTestId("verification-consent-panel"); }
  open() { return this.openButton.click(); }
  withdraw() { return this.panel.getByRole("button", { name: "撤回未来授权" }).click(); }
  confirmWithdrawal() { return this.page.getByRole("button", { name: "确认撤回", exact: true }).click(); }
}

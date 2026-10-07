import type { Locator, Page } from "@playwright/test";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class PaymentExceptionFundsPage {
  readonly page: Page;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("payment-exception-funds-page");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/payment-exception-funds"), {
      waitUntil: "domcontentloaded",
    });
    await this.root.waitFor({ state: "visible", timeout: 15_000 });
    await waitForRouteReady(this.page, 15_000);
  }

  row(exceptionNo: string) {
    return this.root
      .getByTestId(`payment-exception-fund-${exceptionNo}`)
      .locator("xpath=ancestor::tr");
  }

  action(exceptionNo: string) {
    return this.root.getByTestId(
      `payment-exception-fund-action-${exceptionNo}`,
    );
  }

  async allocate(exceptionNo: string, orderNo: string) {
    const action = this.action(exceptionNo);
    await action.getByPlaceholder("输入同一客户的未付订单号").fill(orderNo);
    await action.getByRole("button", { name: "校验并分配" }).click();
  }

  async requestRefund(exceptionNo: string) {
    await this.action(exceptionNo)
      .getByRole("button", { name: "原路退款" })
      .click();
  }

  async confirmManualRefund(exceptionNo: string, content: string) {
    const action = this.action(exceptionNo);
    await action.locator('input[type="file"]').setInputFiles({
      buffer: Buffer.from(content),
      mimeType: "application/pdf",
      name: "refund-proof.pdf",
    });
    const confirmation = this.page.waitForResponse((response) =>
      response.url().endsWith(`/${exceptionNo}/manual-refund-confirmation`),
    );
    await action.getByRole("button", { name: "确认已原路退款" }).click();
    await confirmation;
  }
}

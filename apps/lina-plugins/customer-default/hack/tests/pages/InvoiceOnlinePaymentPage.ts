import type { Page } from "@playwright/test";
import { resolve } from "node:path";
export class InvoiceOnlinePaymentPage {
  constructor(readonly page: Page) {}
  openInvoice(invoiceNo: string) {
    return this.page.goto(
      (process.env.CUSTOMER_E2E_BASE_URL || "http://127.0.0.1:6032") +
        "/account/billing/invoices/" +
        encodeURIComponent(invoiceNo) +
        "?pay=1",
    );
  }
  pay() {
    return this.page.getByTestId("invoice-payment-submit").click();
  }
  async screenshot(step: string) {
    await this.page.locator('#__app-loading__').waitFor({ state: 'hidden' });
    const stamp = new Date(Date.now() + 8 * 3600000).toISOString();
    return this.page.screenshot({
      path: resolve(
        "../../temp/" +
          stamp.slice(0, 10).replaceAll("-", "") +
          "/" +
          stamp.slice(11, 19).replaceAll(":", "") +
          "-invoice-" +
          step +
          ".png",
      ),
      fullPage: true,
    });
  }
}

import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

import type { Locator, Page } from "@host-tests/support/playwright";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { waitForRouteReady } from "../../../../../../hack/tests/support/ui";

export class TaxInvoicePage {
  readonly buyerProfile: Locator;
  readonly invoiceDetail: Locator;
  readonly page: Page;
  readonly redLetterDetail: Locator;
  readonly root: Locator;

  constructor(page: Page) {
    this.page = page;
    this.root = page.getByTestId("tax-invoice-management-page");
    this.invoiceDetail = page.getByTestId("tax-invoice-detail");
    this.buyerProfile = page.getByTestId("tax-invoice-buyer-profile");
    this.redLetterDetail = page.getByTestId("red-letter-detail");
  }

  async goto() {
    await this.page.goto(workspacePath("/idc/compliance/tax-invoices"), {
      waitUntil: "domcontentloaded",
    });
    await waitForRouteReady(this.page, 15_000);
    await this.root.waitFor({ state: "visible" });
  }

  async openInvoice(taxInvoiceNo: string) {
    await this.root.getByTestId(`tax-invoice-view-${taxInvoiceNo}`).click();
    await this.invoiceDetail.waitFor({ state: "visible" });
  }

  async closeInvoice() {
    await this.page.keyboard.press("Escape");
    await this.invoiceDetail.waitFor({ state: "hidden" });
  }

  async openRedLetter(taskNo: string) {
    await this.root.getByRole("tab", { name: /红字任务|Red-letter tasks/u }).click();
    await this.root.getByTestId(`red-letter-view-${taskNo}`).click();
    await this.redLetterDetail.waitFor({ state: "visible" });
  }

  async closeRedLetter() {
    await this.page.keyboard.press("Escape");
    await this.redLetterDetail.waitFor({ state: "hidden" });
  }

  async screenshot(description: string) {
    const now = new Date();
    const day = now.toISOString().slice(0, 10).replaceAll("-", "");
    const stamp = now.toISOString().slice(11, 19).replaceAll(":", "");
    const directory = resolve(process.cwd(), "../../temp", day);
    await mkdir(directory, { recursive: true });
    const path = resolve(directory, `${stamp}-${description}.png`);
    await this.page.screenshot({ fullPage: false, path });
    return path;
  }
}

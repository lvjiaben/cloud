// Actual website/customer template controls; this page object never mocks APIs.
import type { Page } from "@host-tests/support/playwright";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
export class LiveCommercePage {
  constructor(
    readonly page: Page,
    readonly website: string,
  ) {}
  get configurator() {
    return this.page.getByTestId("public-product-configurator");
  }
  get total() {
    return this.page.getByTestId("public-quote-total");
  }
  get cart() {
    return this.page.getByTestId("website-cart");
  }
  get finalQuote() {
    return this.page.getByTestId("website-final-quote");
  }
  get payment() {
    return this.page.getByTestId("invoice-online-pay");
  }
  open(slug: string) {
    return this.page.goto(
      this.website + "/products/" + encodeURIComponent(slug),
    );
  }
  selectOS() {
    return this.page
      .getByRole("radio", { name: "Debian13-x86", exact: true })
      .check();
  }
  quote() {
    return this.page
      .getByRole("button", { name: "查看准确价格", exact: true })
      .click();
  }
  async add() {
    await this.page.getByTestId("public-add-to-cart").click();
    await this.page.waitForURL(this.website + "/cart");
  }
  signIn() {
    return this.cart
      .getByRole("button", { name: "登录并继续结算", exact: true })
      .first()
      .click();
  }
  async login(email: string, password: string) {
    await this.page
      .getByRole("textbox", { name: "请输入用户名", exact: true })
      .fill(email);
    await this.page
      .getByRole("textbox", { name: "请输入密码", exact: true })
      .fill(password);
    await this.page.getByRole("button", { name: "login", exact: true }).click();
  }
  review() {
    return this.page
      .getByRole("button", { name: "核对配置和价格", exact: true })
      .click();
  }
  submitOrder() {
    return this.page.getByTestId("website-submit-order").click();
  }
  pay() {
    return this.page.getByTestId("invoice-payment-submit").click();
  }
  async screenshot(step: string) {
    const stamp = new Date().toISOString(),
      dir = resolve(
        fileURLToPath(new URL("../../../../../../temp/", import.meta.url)),
        stamp.slice(0, 10).replaceAll("-", ""),
      );
    await mkdir(dir, { recursive: true });
    return this.page.screenshot({
      path: resolve(
        dir,
        stamp.slice(11, 19).replaceAll(":", "") +
          "-live-commerce-" +
          step +
          ".png",
      ),
      fullPage: false,
    });
  }
}

import type { Page } from "@host-tests/support/playwright";
import { expect } from "@host-tests/support/playwright";
import { ProductFulfillmentPage } from "./ProductFulfillmentPage";

// 实际本地后台页面，不拦截或伪造交易响应。
export class IsolatedFulfillmentPage {
  constructor(
    readonly page: Page,
    readonly origin: string,
    readonly orderNo: string,
  ) {}
  get detail() {
    return this.page
      .locator('[role="dialog"]:visible')
      .filter({ hasText: this.orderNo })
      .last();
  }
  get action() {
    return this.page
      .locator('[role="dialog"]:visible')
      .filter({ has: this.page.getByTestId("recovery-paid-specifications") });
  }
  async login() {
    await this.page.goto(this.origin + "/#/auth/login");
    await this.page
      .getByRole("textbox", { name: "请输入用户名", exact: true })
      .fill("admin");
    await this.page
      .getByRole("textbox", { name: "请输入密码", exact: true })
      .fill("admin123");
    const loggedIn = this.page.waitForResponse(
      (r) =>
        r.url() === this.origin + "/api/v1/auth/login" &&
        r.request().method() === "POST",
    );
    await this.page.getByRole("button", { name: "login", exact: true }).click();
    expect((await (await loggedIn).json()).code).toBe(0);
    await this.page.waitForURL((url) => !url.hash.includes("/auth/"), {timeout:30_000});
  }
  async open() {
    await this.page.goto(
      this.origin +
        "/#/idc/product-fulfillment/tasks?orderNo=" +
        encodeURIComponent(this.orderNo),
    );
    const root = this.page.getByTestId("product-fulfillment-page");
    const row = root
      .locator(".vxe-body--row")
      .filter({ hasText: this.orderNo });
    await expect(row).toHaveCount(1);
    // 列表已精确限定一笔订单；Vxe 的可见操作按钮位于固定列副本中。
    await root.getByRole("button", { name: /详\s*情/ }).click();
    await expect(
      this.detail.getByText("资金结清", { exact: true }),
    ).toBeVisible();
  }
  async recover() {
    const start = Date.now();
    const loaded = this.page.waitForResponse(response => response.request().method() === 'GET' && response.url().includes('includeRecoveryConfiguration=true'));
    await this.detail
      .getByRole("button", { name: "重新开通", exact: true })
      .click();
    await expect(this.action).toBeVisible();
    expect((await (await loaded).json()).code).toBe(0);
    expect(Date.now()-start).toBeLessThan(3000);
  }
  async choose(label: string, value: string) {
    const refreshed = label === "处理原因" ? undefined : this.page.waitForResponse(response =>
      response.request().method() === "GET" && response.url().includes("includeRecoveryConfiguration=true"));
    const control = this.action.getByRole("combobox", { name: label });
    await expect(control).toBeEnabled();
    await control.focus();
    await control.press("ArrowDown");
    await this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(value, { exact: true })
      .click();
    if (refreshed) {
      expect((await (await refreshed).json()).code).toBe(0);
      await expect(this.action.getByRole("combobox", { name: label })).toBeEnabled();
    }
  }
  async passwordRules(length: number) {
    const control = this.action.getByTestId("recovery-password-length");
    await control.click();
    await this.page.locator(".ant-select-dropdown:visible").getByText(String(length), { exact: true }).click();
    await expect(this.action.getByText("新密码包含字符", { exact: true })).toBeVisible();
    await this.action.getByRole("checkbox", { name: "小写字母", exact: true }).uncheck();
    await this.action.getByRole("checkbox", { name: "特殊字符", exact: true }).uncheck();
  }
  async submit(note: string, reason: string) {
    await this.choose("处理原因", reason);
    await this.action.getByLabel("处理说明").fill(note);
    await this.action.getByRole("button", { name: /确\s*(认|定)/ }).click();
  }
  async showTimeline() {
    const timeline = this.detail.locator(".ant-timeline");
    await timeline.scrollIntoViewIfNeeded();
    return timeline;
  }
  screenshot(name: string) {
    return new ProductFulfillmentPage(this.page).screenshot(name);
  }
}

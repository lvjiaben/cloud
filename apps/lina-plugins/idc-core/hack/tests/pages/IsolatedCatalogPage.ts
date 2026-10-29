// 真实隔离后台商品表单操作，复用已有商品页对象，不伪造业务接口。
import { resolve } from "node:path";
import type { Locator, Page } from "@host-tests/support/playwright";
import { expect } from "@host-tests/support/playwright";
import { LoginPage } from "@host-tests/pages/LoginPage";
import { CatalogWorkspacePage } from "./CatalogWorkspacePage";
import { MerchantOfferPage } from "./MerchantOfferPage";

export class IsolatedCatalogPage extends CatalogWorkspacePage {
  constructor(
    page: Page,
    readonly origin: string,
  ) {
    super(page);
  }
  async login() {
    const isolatedEntry = process.env.IDC_ISOLATED_ADMIN_ENTRY;
    if (isolatedEntry) {
      const path = resolve(isolatedEntry);
      if (
        !path.startsWith(resolve("temp") + "/") ||
        ["6030", "6031", "6032", "6680"].includes(new URL(this.origin).port)
      )
        throw new Error("Admin entry override requires isolated runtime");
      await this.page.route(this.origin + "/", (route) =>
        route.fulfill({ path, contentType: "text/html" }),
      );
    }
    const login = new LoginPage(this.page);
    await login.gotoPath(this.origin + "/#/auth/login");
    const response = this.page.waitForResponse(
      (r) =>
        r.url() === this.origin + "/api/v1/auth/login" &&
        r.request().method() === "POST",
    );
    await login.login("admin", "admin123");
    expect((await (await response).json()).code).toBe(0);
    await this.page.waitForURL((url) => !url.hash.includes("/auth/"), {
      timeout: 30_000,
    });
  }
  async goto() {
    await this.page.goto(this.origin + "/#/idc/catalog");
    await expect(this.pageRoot).toBeVisible();
    await this.waitForDataSettled();
  }
  async openSaleStateConfirmation(productNo: string) {
    await this.page.getByTestId(`catalog-product-more-${productNo}`).click();
    await this.page.getByTestId(`catalog-product-sale-state-${productNo}`).click();
    await expect(this.page.locator(".ant-popover:visible")).toBeVisible();
  }
  async choose(control: Locator, label: string) {
    await expect(control).toBeEnabled();
    await control.focus();
    await control.press("ArrowDown");
    // Search materializes a matching option even when the select virtualizes
    // older channels outside the currently rendered dropdown viewport.
    if (await control.isEditable()) await control.fill(label);
    await this.page
      .locator(".ant-select-dropdown:visible")
      .getByText(label, { exact: true })
      .click();
  }
  async writeThrough(
    action: () => Promise<unknown>,
    suffix: string,
    method: string,
  ) {
    const response = this.page.waitForResponse(
      (r) =>
        new URL(r.url()).pathname.endsWith(suffix) &&
        r.request().method() === method,
    );
    await action();
    const data = await (await response).json();
    expect(data.code, data.message).toBe(0);
    return data.data;
  }
  async create(slug: string, name: string) {
    await this.openProductCreate(false);
    await this.page.getByTestId("catalog-product-create-name-zh").fill(name);
    await this.page
      .getByTestId("catalog-product-create-name-en")
      .fill("Merchant UI cloud server");
    await this.page.getByTestId("catalog-product-create-slug").fill(slug);
    await this.page
      .getByTestId("catalog-product-create-summary-zh")
      .fill("仅用于独立测试库的云服务器");
    await this.page
      .getByTestId("catalog-product-create-summary-en")
      .fill("Cloud server in the isolated test database only");
    await this.choose(
      this.productCreateModal.getByRole("combobox", { name: "所属分类" }),
      "V1隔离云服务器",
    );
    await this.screenshot("actual-merchant-product-create-required-copy");
    return this.writeThrough(
      () =>
        this.page
          .getByRole("dialog")
          .filter({ has: this.productCreateModal })
          .getByRole("button", { name: "创建商品", exact: true })
          .click(),
      "/catalog/products",
      "POST",
    );
  }
  async configureAutomation() {
    await this.selectEditSection("交付设置");
    await this.choose(
      this.fulfillmentPolicyModal.getByRole("combobox", {
        name: "对接上游／接口",
      }),
      "V1隔离模拟魔方云",
    );
    await this.screenshot("actual-merchant-upstream-selected");
    await this.choose(
      this.fulfillmentPolicyModal.getByRole("combobox", { name: "开通方式" }),
      "付款后自动开通",
    );
    await this.writeThrough(
      () => this.fulfillmentPolicyConfirm().click(),
      "/fulfillment-policy",
      "PUT",
    );
    await expect(this.page.getByTestId("catalog-settings-loading")).toHaveCount(
      0,
    );
  }
  async configureFixedParameters() {
    await this.selectEditSection("购买配置");
    await expect(this.page.getByTestId("catalog-package-defaults")).toHaveCount(
      0,
    );
    for (const [key, label, value] of [
      ["region", "地区", ""],
      ["line", "线路", ""],
      ["cpu", "处理器核心数", "2"],
      ["memory", "内存（MiB）", "2000"],
      ["system_disk", "系统盘（GiB）", "22"],
      ["data_disk", "数据盘（GiB）", "22"],
      ["bandwidth", "带宽（Mbps）", "1"],
    ]) {
      const parameter = this.page
        .getByTestId("catalog-customer-option-parameter")
        .getByRole("combobox");
      await this.choose(parameter, `${label} · ${key}`);
      await this.page.getByTestId("catalog-customer-option-add").click();
      const offer = new MerchantOfferPage(this.page);
      if (value) await offer.setChildren(value);
      else {
        const expected = key === "region" ? "隔离陕西地区" : "隔离电信节点";
        await expect
          .poll(
            async () => {
              if (!(await offer.children.innerText()).includes(expected))
                await this.page
                  .getByTestId("catalog-option-import-choices")
                  .click();
              return offer.children.innerText();
            },
            { timeout: 60000, intervals: [1000] },
          )
          .toContain(expected);
        const buttons = offer.children.getByRole("list").getByRole("button");
        await buttons.filter({ hasText: expected }).click();
        const selected = await offer.children
          .getByTestId("catalog-option-child-value")
          .inputValue();
        await offer.setChildren(selected + "|" + expected + "|" + expected);
      }
    }
    await this.writeThrough(
      () => this.page.getByTestId("catalog-customer-options-save").click(),
      "/configuration",
      "PUT",
    );
    await expect(
      this.page.getByTestId("catalog-customer-options-save"),
    ).toBeEnabled();
  }
  async configureTax() {
    await this.selectEditSection("价格与税务");
    await this.page.getByTestId("catalog-tax-add").click();
    const modal = this.page.getByTestId("catalog-tax-modal");
    await this.choose(
      modal.getByRole("combobox", { name: "税务类型" }),
      "免税",
    );
    await this.choose(
      modal.getByRole("combobox", { name: "税务分类" }),
      "云服务",
    );
    await this.writeThrough(
      () =>
        this.page
          .getByRole("dialog")
          .filter({ has: modal })
          .getByRole("button", { name: /确\s*认/ })
          .click(),
      "/tax-policies",
      "POST",
    );
    await expect(modal).toBeHidden();
  }
  async configureMonthlyPrice() {
    await this.selectEditSection("价格与税务");
    const form = this.page.getByTestId("catalog-priced-billing-form");
    if (!(await form.isVisible()))
      await this.page
        .getByTestId("catalog-priced-period-panel")
        .locator(".ant-collapse-header")
        .click();
    await expect(form).toBeVisible();
    await this.page.getByTestId("catalog-priced-amount").fill("1");
    await this.page.getByTestId("catalog-priced-renewal").fill("1");
    await this.page.getByTestId("catalog-priced-setup").fill("10");
    await expect(
      this.page.getByTestId("catalog-priced-currency"),
    ).toContainText("CNY");
    await this.writeThrough(
      () => this.page.getByTestId("catalog-priced-save").click(),
      "/priced-billing-rules",
      "POST",
    );
    await expect(
      this.page.getByTestId("catalog-detail-base-prices"),
    ).toContainText("1 月");
  }
}

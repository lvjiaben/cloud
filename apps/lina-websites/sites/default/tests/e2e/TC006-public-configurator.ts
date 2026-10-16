// This case proves a non-server plugin declaration drives public configuration without exposing keys.
import { expect, test } from "@playwright/test";

import { PublicCatalogPage } from "../pages/PublicCatalogPage";
import { mockPublicCatalog } from "../support/catalog-mock";
import { capturePublicScreenshot } from "../support/screenshot";

test.describe("TC-6 通用公开商品配置", () => {
  let mockState: Awaited<ReturnType<typeof mockPublicCatalog>>;

  test.beforeEach(async ({ page }) => {
    mockState = await mockPublicCatalog(page);
  });

  test("TC-6a: SSL 商品按插件声明展示字段", async ({ page }) => {
    const requestedPaths: string[] = [];
    page.on("request", (request) =>
      requestedPaths.push(new URL(request.url()).pathname),
    );
    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/en-US/products/managed-ssl");

    await expect(catalog.detail).toContainText("Managed SSL certificate");
    await expect(catalog.configurator).toContainText("Certificate domain");
    await expect(catalog.configurator).toContainText("Certificate type");
    await expect(catalog.detail).toContainText("CNY");
    await expect(catalog.detail).not.toContainText("CPU");
    await expect(catalog.detail).not.toContainText("Memory");
    await expect(catalog.detail).not.toContainText("opaque_");
    expect(requestedPaths).toContain(
      "/x/idc-core/api/v1/public/catalog/products/managed-ssl/purchase-configuration",
    );
    await expect
      .poll(() => mockState.configurationEvaluations.length)
      .toBeGreaterThan(0);
    const values = mockState.configurationEvaluations.at(-1)?.values as Array<
      Record<string, unknown>
    >;
    expect(
      values.some((value) => value.fieldToken === "opaque_region_field"),
    ).toBe(false);
    for (const value of values) {
      expect(
        [
          "addressOptionTokens",
          "booleanValue",
          "dateTimeValue",
          "dateValue",
          "decimalValue",
          "fileTokens",
          "integerValue",
          "optionTokens",
          "textValue",
          "treeOptionTokens",
        ].filter((key) => value[key] !== undefined),
      ).toHaveLength(1);
    }
  });

  test("TC-6b: 准确价格只由服务端报价返回", async ({ page }) => {
    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/en-US/products/managed-ssl");
    await expect(
      catalog.configurator.getByRole("button", { name: "Check exact price" }),
    ).toBeEnabled();
    await catalog.requestConfigurationQuote();
    await expect(catalog.configurator).toContainText("CNY 98.00");
    await capturePublicScreenshot(page, "public-ssl-configuration-en");
  });

  test("TC-6c: 移动端插件配置无横向溢出", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/en-US/products/managed-ssl");
    await expect(catalog.configurator).toBeVisible();
    await expect(catalog.configurator).toContainText("Supporting documents");
    expect(await catalog.hasHorizontalOverflow()).toBe(false);
    await capturePublicScreenshot(page, "public-generic-product-mobile");
  });

  test("TC-6d: 配置变化后迟到的旧报价不会回显", async ({ page }) => {
    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/en-US/products/managed-ssl");
    const quoteRequest = page.waitForRequest((request) =>
      new URL(request.url()).pathname.endsWith(
        "/managed-ssl/purchase-configuration/quote",
      ),
    );
    await catalog.configurator
      .getByRole("button", { name: "Check exact price" })
      .evaluate((button: HTMLButtonElement) => button.click());
    await quoteRequest;
    const domain = catalog.configurator
      .locator(".ant-form-item")
      .filter({ hasText: "Certificate domain" })
      .locator("input");
    await domain.fill("changed.example");
    await page.waitForTimeout(550);
    await expect(catalog.configurator).not.toContainText("CNY 98.00");
  });

  test("TC-6e: 动态候选由权威评估更新且匿名文件只展示安全上传要求", async ({
    page,
  }) => {
    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/en-US/products/managed-ssl");

    await expect(catalog.configurator).toContainText("Supporting documents");
    const policy = catalog.configurator.getByTestId("purchase-file-policy");
    await expect(policy).toContainText("Maximum files");
    await expect(policy).toContainText("3");
    await expect(policy).toContainText("5 MiB");
    await expect(policy).toContainText("10 MiB");
    await expect(policy).toContainText(".pdf, .png");
    await expect(policy).toContainText("application/pdf, image/png");
    await expect(policy).toContainText("Required");
    await expect(catalog.configurator).toContainText(
      "Files are uploaded in the secure signed-in purchase flow",
    );
    await expect(catalog.configurator).not.toContainText(/https?:\/\/|token/iu);

    await catalog.configurator
      .getByRole("radio", { name: "Wildcard domain" })
      .click();
    await expect
      .poll(() => mockState.configurationEvaluations.length)
      .toBeGreaterThan(1);
    const region = catalog.configurator
      .locator(".ant-form-item")
      .filter({ hasText: "Validation region" });
    await region.locator(".ant-select-selector").click();
    await expect(page.getByText("Europe", { exact: true })).toBeVisible();
    await expect(page.getByText("Asia Pacific", { exact: true })).toHaveCount(
      0,
    );
  });

  test("TC-6f: 中文文件上传要求完整且不泄露上传地址或令牌", async ({
    page,
  }) => {
    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/products/managed-ssl");
    const policy = catalog.configurator.getByTestId("purchase-file-policy");
    await expect(catalog.configurator).toContainText("登录后的安全购买流程");
    await expect(policy).toContainText("最多文件数");
    await expect(policy).toContainText("单个文件上限");
    await expect(policy).toContainText("全部文件合计上限");
    await expect(policy).toContainText("允许扩展名");
    await expect(policy).toContainText("允许文件类型");
    await expect(policy).toContainText("安全扫描");
    await expect(policy).toContainText("需要");
    await expect(catalog.configurator).not.toContainText(/https?:\/\/|令牌|token/iu);
  });

  test("TC-6g: 数量100和0原样提交且价格采用服务端结果", async ({ page }) => {
    const schema = {
      configurationRevision: "quantity-revision",
      fields: [{ fieldToken: "quantity-field", type: "integer", labelZhCN: "数据盘容量", labelEnUS: "Data disk", required: true, visible: true, disabled: false, minimumInteger: "0", maximumInteger: "200" }],
      values: [{ fieldToken: "quantity-field", type: "integer", integerValue: "100" }],
    };
    const quotedValues: Array<Record<string, unknown>> = [];
    await page.route("**/managed-ssl/purchase-configuration**", async (route) => {
      const path = new URL(route.request().url()).pathname;
      if (path.endsWith("/evaluate")) {
        const body = route.request().postDataJSON();
        const allowed = Number(body.values[0]?.integerValue) <= 200;
        await route.fulfill({ json: { code: 0, data: { ...schema, values: body.values, valid: allowed, issues: allowed ? [] : [{ fieldToken: "quantity-field", issueCode: "out_of_range" }] } } });
      } else if (path.endsWith("/quote")) {
        const body = route.request().postDataJSON();
        const value = body.values[0] as Record<string, unknown>;
        quotedValues.push(value);
        await route.fulfill({ json: { code: 0, data: {
          configurationRevision: schema.configurationRevision,
          currencyCode: "CNY",
          terms: [{ billingTermToken: "quantity-term", labelZhCN: "月付", labelEnUS: "Monthly", minimumPurchaseCycles: 1, setupAmount: "0", status: "final", subtotalAmount: value.integerValue === "0" ? "9800" : "11800", taxAmount: "0", totalAmount: value.integerValue === "0" ? "9800" : "11800" }],
        } } });
      } else {
        await route.fulfill({ json: { code: 0, data: schema } });
      }
    });
    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/products/managed-ssl");
    await expect(catalog.configurator).toContainText("数据盘容量");
    await catalog.requestConfigurationQuote();
    await expect(catalog.configurator).toContainText("CNY 118.00");
    expect(quotedValues[0]).toEqual({ fieldToken: "quantity-field", type: "integer", integerValue: "100" });
    await catalog.setConfigurationQuantity("数据盘容量", "0");
    await expect(catalog.configurator).not.toContainText("CNY 118.00");
    await catalog.requestConfigurationQuote();
    await expect(catalog.configurator).toContainText("CNY 98.00");
    expect(quotedValues.at(-1)).toEqual({ fieldToken: "quantity-field", type: "integer", integerValue: "0" });
    await capturePublicScreenshot(page, "quantity-zero-quote");
    await catalog.setConfigurationQuantity("数据盘容量", "201");
    await expect(catalog.configurator.getByRole("button", { name: "查看准确价格" })).toBeDisabled();
    await expect(catalog.configurator).toContainText("请检查标出的配置后再查看价格。");
    await expect(catalog.configurator).not.toContainText("CNY 98.00");
    expect(quotedValues).toHaveLength(2);
    await capturePublicScreenshot(page, "quantity-out-of-range");
  });
});

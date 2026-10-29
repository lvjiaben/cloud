// 从真实管理员表单新建可售套餐，再经支付拥有方和实际冻结连接器交付。
import { randomUUID, createHash } from "node:crypto";
import { test } from "@host-tests/fixtures/auth";
import { expect, request } from "@host-tests/support/playwright";
import {
  api,
  descriptor,
  fixture,
  get,
  saved,
  serversPath,
} from "../support/isolated-commerce";
import { createCustomer, paidOrder } from "../support/isolated-paid-order";
import { IsolatedCatalogPage } from "../pages/IsolatedCatalogPage";
import { MerchantOfferPage } from "../pages/MerchantOfferPage";

test.describe("TC-70 实际商家表单到云服务器交付", () => {
  test.skip(!descriptor, "Requires the explicit local fake-commerce database");
  test.setTimeout(240_000);
  test("TC-70b: 统一配置保存CPU选配及单价，客户选择8核付款后按8核开通", async ({
    browser,
  }) => {
    const f = (await fixture()) as Awaited<ReturnType<typeof fixture>> & {
      adminOrigin: string;
    };
    const admin = await api(f, f.adminAuth),
      slug = "offer-ui-" + randomUUID();
    const context = await browser.newContext({ baseURL: f.adminOrigin });
    context.setDefaultTimeout(20000);
    const allowed = new Set([f.adminOrigin, f.baseURL, f.mockOrigin]);
    await context.route("**/*", (r) =>
      allowed.has(new URL(r.request().url()).origin) ? r.continue() : r.abort(),
    );
    let client: Awaited<ReturnType<typeof createCustomer>> | undefined;
    try {
      const ui = new IsolatedCatalogPage(
        await context.newPage(),
        f.adminOrigin,
      );
      await ui.login();
      await ui.goto();
      const product = await ui.create(slug, "实际后台用户选配套餐");
      await ui.configureAutomation();
      await ui.configureFixedParameters();
      await ui.configureMonthlyPrice();
      const offer = new MerchantOfferPage(ui.page);
      await offer.selectEditSection("购买配置");
      await offer.chooseField("local_product_cpu");
      await expect(offer.offers.getByTestId("catalog-option-remove")).toHaveCount(0);
      await expect(offer.offers.getByRole("switch", { name: "接口必填核心项", exact: true })).toBeDisabled();
      await offer.control("下拉选择规格");
      await offer.setChildren("2\n4\n8");
      await offer.addField("节点选择优先级 · node_priority");
      await offer.setChildren("3|内存占用最低|Lowest memory usage");
      await offer.addField("流量重置周期 · traffic_bill_type");
      await offer.setChildren("last_30days|按开通日每月重置|Service month");
      await offer.addField("系统盘性能 · system_disk_io_limit");
      await offer.setChildren("500,500,2000,2000|高性能|High performance");
      await offer.chooseField("operating_system");
      await ui.page.getByTestId("catalog-option-import-choices").click();
      await offer.setChildren("97|Debian长期支持版|Debian LTS");
      await offer.chooseField("local_product_cpu");
      // A valid configuration plus a syntactically valid stale price must
      // roll back the configuration, new dimension and price in one transaction.
      const beforePrice = await get(
        admin,
        "/x/idc-core/api/v1/admin/catalog/products/" + product.productNo,
      );
      expect(beforePrice.version.status).toBe("draft");
      const configPath =
        "/x/idc-core/api/v1/admin/catalog/product-versions/" +
        beforePrice.version.versionNo +
        "/configuration";
      const beforeConfiguration = (await get(admin, configPath)).configuration;
      const failedSelections = JSON.parse(
        JSON.stringify(beforeConfiguration.customerFieldSelections),
      );
      const failedCPU = failedSelections.find(
        (selection: any) => selection.fieldKey === "local_product_cpu",
      );
      expect(failedCPU).toBeTruthy();
      failedCPU.displayNameZhCN = "不应保存的临时名称";
      const failedResponse = await admin.put(configPath, {
        data: {
          expectedVersion: beforeConfiguration.version,
          merchantValues: beforeConfiguration.merchantValues,
          customerFieldSelections: failedSelections,
          priceRows: [
            {
              fieldKey: "local_product_cpu",
              candidateValue: "",
              quantity: true,
              pricingMode: "per_unit",
              active: true,
              previousPriceNo: "prc_" + "f".repeat(32),
              billingRuleNo: beforePrice.billingRules.find(
                (rule: any) => rule.status === "active",
              ).billingRuleNo,
              currencyCode: "CNY",
              tiers: [
                {
                  minimumQuantity: 0,
                  maximumQuantity: null,
                  amountMinor: "100",
                  setupFeeMinor: "0",
                  renewalAmountMinor: "100",
                },
              ],
            },
          ],
        },
      });
      const rejected = await failedResponse.json();
      expect(rejected.code).not.toBe(0);
      expect(rejected.errorCode).toBe("IDC_CATALOG_MANAGEMENT_CONFLICT");
      const afterFailure = (await get(admin, configPath)).configuration;
      expect(afterFailure.version).toBe(beforeConfiguration.version);
      expect(afterFailure.customerFieldSelections).toEqual(
        beforeConfiguration.customerFieldSelections,
      );
      expect(afterFailure.merchantPriceTargets || []).toEqual(
        beforeConfiguration.merchantPriceTargets || [],
      );
      const prices = ui.page.getByTestId("catalog-offer-field-prices");
      await prices.getByTestId("catalog-offer-price-amount").fill("1.00");
      await prices.getByTestId("catalog-offer-price-renewal").fill("1.00");
      const submission = ui.page.waitForRequest(
        (request) =>
          request.method() === "PUT" &&
          request.url().endsWith("/configuration") &&
          !!request.postDataJSON().priceRows?.length,
      );
      await ui.writeThrough(() => offer.save(), "/configuration", "PUT");
      expect((await submission).postDataJSON().priceRows).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            fieldKey: "local_product_cpu",
            quantity: true,
            pricingMode: "per_unit",
            tiers: [
              expect.objectContaining({
                amountMinor: "100",
                renewalAmountMinor: "100",
              }),
            ],
          }),
        ]),
      );
      await expect(
        ui.page.getByTestId("catalog-customer-options-save"),
      ).toBeEnabled();
      await expect(offer.offers).not.toContainText(
        "配置已被更新，请重新读取后再保存。",
      );
      await expect(
        prices.getByTestId("catalog-offer-price-amount"),
      ).toHaveValue("1.00");
      const detail = await get(
        admin,
        "/x/idc-core/api/v1/admin/catalog/products/" + product.productNo,
      );
      expect(detail.prices).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            configurationPricingMode: "per_unit",
            amountMinor: "100",
            renewalAmountMinor: "100",
            setupFeeMinor: "0",
          }),
        ]),
      );
      await ui.screenshot("actual-merchant-option-price-saved");
      await ui.configureTax();
      await ui.closeProductDetail();
      await ui.searchNamePrefix(slug);
      await ui.openSaleStateConfirmation(product.productNo);
      await ui.writeThrough(
        () =>
          ui.page
            .locator(".ant-popover:visible")
            .getByRole("button", { name: /确\s*定/ })
            .click(),
        "/sale-state",
        "PUT",
      );
      const key = "elastic-" + randomUUID();
      client = await createCustomer(f, admin, key);
      const { orderNo } = await paidOrder(f, client, key, slug, {
        fieldValues: { 处理器核心数: "8" },
        expectedTotal: "1900",
        imageLabelEnUS: "Debian LTS",
      });
      const tasks = "/x/idc-core/api/v1/admin/product-fulfillment/tasks";
      let task: any;
      await expect
        .poll(
          async () => {
            const rows = (await get(admin, tasks + "?orderNo=" + orderNo))
              .tasks;
            if (rows.length !== 1) return "";
            task = (await get(admin, tasks + "/" + rows[0].intentNo)).task;
            return task.statusCode;
          },
          { timeout: 120000, intervals: [1000] },
        )
        .toBe("completed");
      const digest = createHash("sha256")
        .update(
          "idc-resource/server/v1\0" +
            task.intentNo +
            "\0" +
            task.orderItemNo +
            "\0" +
            task.unitSequence,
        )
        .digest("hex");
      const forms = (await saved(f, f.mockState)).forms.filter(
        (row: any) =>
          row.hostname?.[0] === "srv-" + digest.slice(0, 16) + ".internal",
      );
      expect(forms).toHaveLength(1);
      expect(forms[0]).toMatchObject({
        cpu: ["8"],
        node_priority: ["3"],
        system_read_bytes_sec: ["500"],
        system_write_bytes_sec: ["500"],
        system_read_iops_sec: ["2000"],
        system_write_iops_sec: ["2000"],
        memory: ["2000"],
        system_disk_size: ["22"],
        node: ["5"],
        os: ["97"],
      });
      expect(forms[0].traffic_bill_type).toBeUndefined();
      expect(Number(forms[0].reset_flow_day[0])).toBeGreaterThanOrEqual(1);
      expect(Number(forms[0].reset_flow_day[0])).toBeLessThanOrEqual(31);
      const order = await get(
        client,
        "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
      );
      expect(order.order.invoice.paidAmountMinor).toBe("1900");
      expect(order.order.invoice.refundedAmountMinor).toBe("0");
    } finally {
      await client?.dispose();
      await admin.dispose();
      await context.close();
    }
  });
  test("TC-70a: 新建商品配置上游规格和月价，配置保存后上架，按原规格付款开通", async ({
    browser,
  }) => {
    const f = (await fixture()) as Awaited<ReturnType<typeof fixture>> & {
      adminOrigin: string;
    };
    const origin = new URL(f.adminOrigin);
    expect(origin.protocol).toBe("http:");
    expect(origin.hostname).toBe("127.0.0.1");
    expect(["6030", "6031", "6032", "6680"]).not.toContain(origin.port);
    const admin = await api(f, f.adminAuth),
      key = "merchant-" + randomUUID(),
      slug = "ui-" + randomUUID();
    const context = await browser.newContext({ baseURL: f.adminOrigin });
    context.setDefaultTimeout(20_000);
    const allowed = new Set([f.adminOrigin, f.baseURL, f.mockOrigin]);
    await context.route("**/*", (r) =>
      allowed.has(new URL(r.request().url()).origin) ? r.continue() : r.abort(),
    );
    const upstream = await request.newContext({
      baseURL: f.mockOrigin,
      ignoreHTTPSErrors: true,
    });
    let client: Awaited<ReturnType<typeof createCustomer>> | undefined;
    try {
      client = await createCustomer(f, admin, key);
      expect(
        (
          await upstream.post("/fixture/mode", { data: { mode: "success" } })
        ).status(),
      ).toBe(200);
      const page = new IsolatedCatalogPage(
        await context.newPage(),
        f.adminOrigin,
      );
      await page.login();
      await page.goto();
      await page.screenshot("actual-merchant-catalog-start");
      const product = await page.create(slug, "后台表单新建云服务器");
      expect(product.productNo).toMatch(/^prd_[0-9a-f]{32}$/);
      await expect(page.detailDrawer).toBeVisible();
      await page.configureAutomation();
      await page.screenshot("actual-merchant-automation-saved");
      await page.configureFixedParameters();
      await page.screenshot("actual-merchant-parameters-saved");
      await page.configureMonthlyPrice();
      await page.screenshot("actual-merchant-monthly-price-saved");
      const productPath =
        "/x/idc-core/api/v1/admin/catalog/products/" + product.productNo;
      await page.closeProductDetail();
      await page.searchNamePrefix(slug);
      const blockedSale = page.page.waitForResponse(
        (response) =>
          new URL(response.url()).pathname.endsWith("/sale-state") &&
          response.request().method() === "PUT",
      );
      await page.openSaleStateConfirmation(product.productNo);
      await page.page
        .locator(".ant-popover:visible")
        .getByRole("button", { name: /确\s*定/ })
        .click();
      const blocked = await (await blockedSale).json();
      expect(blocked.errorCode).toBe("IDC_CATALOG_SALE_TAX_REQUIRED");
      await expect(
        page.page.getByText(
          "请在“计费与售价”为每个售价币种设置税务规则后再上架",
          { exact: true },
        ),
      ).toBeVisible();
      expect((await get(admin, productPath)).product.status).toBe("suspended");
      await page.screenshot("actual-merchant-missing-tax-explained");
      await page.page
        .locator(".ant-popover:visible")
        .getByRole("button", { name: /取\s*消/ })
        .click();
      await page.openProductDetail(product.productNo);
      await page.configureTax();
      await page.closeProductDetail();
      await page.searchNamePrefix(slug);
      await page.openSaleStateConfirmation(product.productNo);
      await page.writeThrough(
        () =>
          page.page
            .locator(".ant-popover:visible")
            .getByRole("button", { name: /确\s*定/ })
            .click(),
        "/sale-state",
        "PUT",
      );
      await expect
        .poll(async () => (await get(admin, productPath)).product.status, {
          timeout: 30_000,
        })
        .toBe("active");
      const detail = await get(admin, productPath);
      expect(detail.fulfillmentPolicy.strategyCode).toBe(
        "payment_confirmed_plugin",
      );
      expect(
        detail.prices.filter((p: any) => !p.configurationPriceDimensionNo),
      ).toEqual(
        expect.arrayContaining([
          expect.objectContaining({
            amountMinor: "100",
            setupFeeMinor: "1000",
            renewalAmountMinor: "100",
          }),
        ]),
      );
      await page.searchNamePrefix(slug);
      await page.screenshot("actual-merchant-new-product-listed");
      const { orderNo } = await paidOrder(f, client, key, slug);
      const tasks = "/x/idc-core/api/v1/admin/product-fulfillment/tasks";
      let task: any;
      await expect
        .poll(
          async () => {
            const rows = (await get(admin, tasks + "?orderNo=" + orderNo))
              .tasks;
            if (rows.length !== 1) return "";
            task = (await get(admin, tasks + "/" + rows[0].intentNo)).task;
            return task.statusCode;
          },
          { timeout: 120_000, intervals: [1000] },
        )
        .toBe("completed");
      await expect
        .poll(
          async () =>
            (await get(client!, serversPath + "?page=1&pageSize=20")).total,
          { timeout: 30_000 },
        )
        .toBe(1);
      const servers = await get(client, serversPath + "?page=1&pageSize=20");
      expect(
        await get(client, serversPath + "/" + servers.items[0].serverNo),
      ).toMatchObject({
        orderNo,
        runtimeState: "active",
        entitlementStatus: "active",
        credentialAvailable: true,
      });
      const digest = createHash("sha256")
        .update(
          "idc-resource/server/v1\0" +
            task.intentNo +
            "\0" +
            task.orderItemNo +
            "\0" +
            task.unitSequence,
        )
        .digest("hex");
      const forms = (await saved(f, f.mockState)).forms.filter(
        (row: any) =>
          row.hostname?.[0] === "srv-" + digest.slice(0, 16) + ".internal",
      );
      expect(forms).toHaveLength(1);
      expect(forms[0]).toMatchObject({
        node: ["5"],
        os: ["97"],
        cpu: ["2"],
        memory: ["2000"],
        system_disk_size: ["22"],
        "other_data_disk[0][size]": ["22"],
        in_bw: ["1"],
        out_bw: ["1"],
      });
      const order = await get(
        client,
        "/x/idc-core/api/v1/account/commerce/orders/" + orderNo,
      );
      expect(order.order.invoice.paidAmountMinor).toBe("1100");
      expect(order.order.invoice.refundedAmountMinor).toBe("0");
    } finally {
      expect(
        (
          await upstream.post("/fixture/mode", { data: { mode: "success" } })
        ).status(),
      ).toBe(200);
      await upstream.dispose();
      await client?.dispose();
      await admin.dispose();
      await context.close();
    }
  });
});

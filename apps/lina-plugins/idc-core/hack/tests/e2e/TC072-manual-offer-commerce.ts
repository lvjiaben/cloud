// A product without any automation binding uses the same commerce contracts.
// Only the explicitly named disposable database and loopback payment are used.
import { randomUUID } from "node:crypto";
import { test } from "@host-tests/fixtures/auth";
import { expect } from "@host-tests/support/playwright";
import { api, descriptor, fixture, get, saved } from "../support/isolated-commerce";
import { createCustomer, paidConfiguredOrder } from "../support/isolated-paid-order";
import { IsolatedCatalogPage } from "../pages/IsolatedCatalogPage";
import { MerchantOfferPage } from "../pages/MerchantOfferPage";

test.describe("TC-72 通用商品配置不依赖自动化", () => {
  test.skip(!descriptor, "Requires the dedicated disposable commerce database");
  test("TC-72a: 无接口维护资料与可售档位，按选项计价付款并人工交付", async ({ browser }) => {
    test.setTimeout(180000);
    const f = await fixture() as Awaited<ReturnType<typeof fixture>> & { adminOrigin: string };
    const admin = await api(f, f.adminAuth);
    const context = await browser.newContext({ baseURL: f.adminOrigin });
    context.setDefaultTimeout(20000);
    const allowed = new Set([f.adminOrigin, f.baseURL, f.mockOrigin]);
    await context.route("**/*", (route) => allowed.has(new URL(route.request().url()).origin) ? route.continue() : route.abort());
    const slug = "manual-offer-" + randomUUID();
    let client: Awaited<ReturnType<typeof createCustomer>> | undefined;
    let productNo = "";
    try {
      const ui = new IsolatedCatalogPage(await context.newPage(), f.adminOrigin);
      await ui.login();
      await ui.goto();
      const created = await ui.create(slug, "人工交付通用存储套餐");
      productNo = created.productNo;
      await ui.selectEditSection("交付设置");
      await ui.writeThrough(() => ui.fulfillmentPolicyConfirm().click(), "/fulfillment-policy", "PUT");
      await ui.configureMonthlyPrice();
      const view = new MerchantOfferPage(ui.page);
      await view.selectEditSection("购买配置");
      await expect(ui.page.getByTestId("catalog-customer-option-parameter")).toHaveCount(0);
      await expect(view.offers).not.toContainText("操作系统");
      await ui.page.getByTestId("catalog-local-option-add").click();
      await view.offers.getByRole("checkbox", { name: "分组、排序与多语言设置", exact: true }).check();
      await view.input("配置名称（中文）").fill("存储等级");
      await view.input("配置名称（英文）").fill("Storage profile");
      await view.setChildren("standard|标准存储|Standard\narchive|归档存储|Archive");
      const prices = ui.page.getByTestId("catalog-offer-field-prices");
      await prices.getByTestId("catalog-offer-price-amount").nth(0).fill("0");
      await prices.getByTestId("catalog-offer-price-renewal").nth(0).fill("0");
      await prices.getByTestId("catalog-offer-price-amount").nth(1).fill("3");
      await prices.getByTestId("catalog-offer-price-renewal").nth(1).fill("3");
      await ui.choose(ui.page.getByTestId("catalog-local-option-type").getByRole("combobox"), "单行文本");
      await ui.page.getByTestId("catalog-local-option-add").click();
      await view.input("配置名称（中文）").fill("服务用途");
      await view.input("配置名称（英文）").fill("Service purpose");
      await expect(view.offers.getByRole("combobox", { name: "开通接口参数", exact: true })).toHaveCount(0);
      await view.save();
      await expect(ui.page.getByTestId("catalog-customer-options-save")).toBeEnabled();
      await expect(ui.page.getByTestId("catalog-customer-options-save")).not.toHaveClass(/ant-btn-loading/);
      await ui.screenshot("manual-offer-fields-and-prices-saved");
      const path = "/x/idc-core/api/v1/admin/catalog/products/" + productNo;
      let detail = await get(admin, path);
      expect(detail.fulfillmentBinding).toBeFalsy();
      const configPath = "/x/idc-core/api/v1/admin/catalog/product-versions/" + detail.version.versionNo + "/configuration";
      const configuration = (await get(admin, configPath)).configuration;
      expect(configuration.customerFieldSelections).toHaveLength(2);
      const bad = await admin.put(configPath, { data: {
        expectedVersion: configuration.version, merchantValues: [],
        customerFieldSelections: configuration.customerFieldSelections.map((row: any, index: number) => ({ ...row, ...(index === 0 ? { productFieldKey: "unverified_supplier_parameter" } : {}) })),
      } });
      expect((await bad.json()).errorCode).toBe("IDC_CATALOG_MANAGEMENT_INVALID");
      expect((await get(admin, configPath)).configuration.version).toBe(configuration.version);
      await ui.configureTax();
      await ui.closeProductDetail();
      await ui.searchNamePrefix(slug);
      await ui.openSaleStateConfirmation(productNo);
      await ui.writeThrough(() => ui.page.locator(".ant-popover:visible").getByRole("button", { name: /确\s*定/ }).click(), "/sale-state", "PUT");
      detail = await get(admin, path);
      expect(detail.fulfillmentBinding).toBeFalsy();
      client = await createCustomer(f, admin, "manual-" + randomUUID());
      const publicBase = "/x/idc-core/api/v1/public/catalog/products/" + slug + "/purchase-configuration";
      const visible = await get(client, publicBase);
      expect(visible.fields.map((field: any) => field.labelZhCN)).toEqual(["存储等级", "服务用途"]);
      const profile = visible.fields[0];
      const purpose = visible.fields[1];
      const values = [{ fieldToken: profile.fieldToken, type: profile.type, optionTokens: [profile.options.find((row: any) => row.labelZhCN === "归档存储").optionToken] },
        { fieldToken: purpose.fieldToken, type: purpose.type, textValue: "隔离人工商品配置冻结验收" }];
      const before = (await saved(f, f.mockState)).forms.filter((row: any) => row.hostname?.[0]?.startsWith("srv-"));
      const paid = await paidConfiguredOrder(f, client, "manual-payment-" + randomUUID(), { publicSlug: slug, configurationRevision: visible.configurationRevision, values }, "1400");
      await expect.poll(async () => (await get(client!, "/x/idc-core/api/v1/account/commerce/orders/" + paid.orderNo)).order.invoice.paidAmountMinor, { timeout: 45000 }).toBe("1400");
      const tasksPath = "/x/idc-core/api/v1/admin/product-fulfillment/tasks";
      let task: any;
      await expect.poll(async () => {
        const tasks = (await get(admin, tasksPath + "?orderNo=" + paid.orderNo)).tasks;
        if (tasks.length !== 1) return false;
        task = (await get(admin, tasksPath + "/" + tasks[0].intentNo)).task;
        return task.canManualConclude;
      }, { timeout: 45000 }).toBe(true);
      expect(task.financiallySettled).toBe(true);
      const result = await admin.post(tasksPath + "/" + task.intentNo + "/manual-conclusions", { data: {
        expectedVersion: task.version, outcomeCode: "completed", idempotencyKey: "manual-result-" + randomUUID(),
        reasonCode: task.manualCompletedReasons[0].code, note: "隔离人工商品已完成交付，仅用于本地业务流程验收。",
      } });
      const completed = await result.json();
      expect(completed.code, completed.message).toBe(0);
      expect(completed.data.task.statusCode).toBe("completed");
      const customerServices = await get(client, "/x/idc-core/api/v1/account/product-directories/all/services?page=1&pageSize=5");
      // Manual completion records delivery without inventing a managed service
      // or renewal contract. Such purchases remain visible in their order.
      expect(customerServices.total).toBe(0);
      expect(customerServices.items).toHaveLength(0);
      const customerOrder = (await get(client, "/x/idc-core/api/v1/account/commerce/orders/" + paid.orderNo)).order;
      expect(customerOrder.serviceEntitlements).toHaveLength(0);
      expect(customerOrder.deliveries).toEqual(expect.arrayContaining([expect.objectContaining({ statusCode: "completed", ongoingService: false })]));
      const after = (await saved(f, f.mockState)).forms.filter((row: any) => row.hostname?.[0]?.startsWith("srv-"));
      expect(after.length).toBe(before.length);
      // Adding automation later preserves commercial fields, prices and the
      // already paid manual delivery. Parameter admission happens on save.
      await ui.openProductDetail(productNo);
      await ui.configureAutomation();
      await view.selectEditSection("购买配置");
      await expect(view.offers).toContainText("存储等级");
      await expect(view.offers).toContainText("服务用途");
      await ui.configureFixedParameters();
      detail = await get(admin, path);
      const adopted = (await get(admin, "/x/idc-core/api/v1/admin/catalog/product-versions/" + detail.version.versionNo + "/configuration")).configuration;
      const profileKey = configuration.customerFieldSelections[0].fieldKey;
      expect(adopted.customerFieldSelections.find((row: any) => row.fieldKey === profileKey).localField.label.zhCN).toBe("存储等级");
      const archivePrice = adopted.merchantPriceTargets.find((row: any) => row.fieldKey === profileKey && row.candidateValue === "archive");
      expect(archivePrice).toBeTruthy();
      expect(detail.prices.find((row: any) => row.configurationPriceDimensionNo === archivePrice.dimensionNo && row.status === "active").amountMinor).toBe("300");
      const automated = await get(client, publicBase);
      expect(automated.fields.map((row: any) => row.labelZhCN)).toEqual(expect.arrayContaining(["存储等级", "服务用途", "操作系统"]));
      const historical = (await get(admin, tasksPath + "/" + task.intentNo)).task;
      expect(historical.statusCode).toBe("completed");
      expect(historical.fulfillmentModeCode).toBe("manual");
      await view.chooseField(profileKey);
      await view.field(profileKey).scrollIntoViewIfNeeded();
      await ui.screenshot("manual-offer-retained-after-automation-binding");
    } finally {
      if (productNo) {
        const detail = await get(admin, "/x/idc-core/api/v1/admin/catalog/products/" + productNo);
        if (detail.product.status === "active") {
          const stopped = await admin.put("/x/idc-core/api/v1/admin/catalog/products/" + productNo + "/sale-state", { data: { versionNo: detail.product.currentVersionNo, expectedVersion: detail.product.descriptionVersion, active: false } });
          const result = await stopped.json();
          expect(result.code, result.message).toBe(0);
        }
      }
      await client?.dispose();
      await context.close();
      await admin.dispose();
    }
  });
});

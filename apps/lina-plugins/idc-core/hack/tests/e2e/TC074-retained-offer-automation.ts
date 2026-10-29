// Real isolated UI conversion, quote and paid-history checks. Only the fenced
// fixture supplier is called, and every product belongs to this test.
import { createHash, randomUUID } from "node:crypto";
import { test } from "@host-tests/fixtures/auth";
import { expect } from "@host-tests/support/playwright";
import { api, descriptor, fixture, get, saved } from "../support/isolated-commerce";
import { createCustomer, paidConfiguredOrder } from "../support/isolated-paid-order";
import { IsolatedCatalogPage } from "../pages/IsolatedCatalogPage";
import { MerchantOfferPage } from "../pages/MerchantOfferPage";

test.describe("TC-74 更换交付接口保留商业商品", () => {
  test.skip(!descriptor, "Requires the dedicated disposable commerce database");
  test("TC-74a: 自动交付转人工再重新对应接口，价格及旧单保持原事实", async ({ browser }) => {
    test.setTimeout(300000);
    const f = await fixture() as Awaited<ReturnType<typeof fixture>> & { adminOrigin: string };
    const admin = await api(f, f.adminAuth);
    const context = await browser.newContext({ baseURL: f.adminOrigin });
    const allowed = new Set([f.adminOrigin, f.baseURL, f.mockOrigin]);
    await context.route("**/*", route => allowed.has(new URL(route.request().url()).origin) ? route.continue() : route.abort());
    const slug = "retained-offer-" + randomUUID();
    let productNo = "";
    let client: Awaited<ReturnType<typeof createCustomer>> | undefined;
    try {
      const ui = new IsolatedCatalogPage(await context.newPage(), f.adminOrigin);
      await ui.login(); await ui.goto();
      productNo = (await ui.create(slug, "可更换接口的商业套餐")).productNo;
      await ui.configureAutomation(); await ui.configureFixedParameters(); await ui.configureMonthlyPrice();
      const offer = new MerchantOfferPage(ui.page);
      await offer.selectEditSection("购买配置");
      await ui.page.getByTestId("catalog-local-option-add").click();
      await offer.offers.getByRole("checkbox", { name: "分组、排序与多语言设置", exact: true }).check();
      await offer.input("配置名称（中文）").fill("存储等级");
      await offer.input("配置名称（英文）").fill("Storage profile");
      await offer.setChildren("standard|标准存储|Standard\narchive|归档存储|Archive");
      const prices = ui.page.getByTestId("catalog-offer-field-prices");
      for (const [index, amount] of ["0", "3"].entries()) {
        await prices.getByTestId("catalog-offer-price-amount").nth(index).fill(amount);
        await prices.getByTestId("catalog-offer-price-renewal").nth(index).fill(amount);
      }
      await ui.selectEditSection("交付设置");
      await ui.choose(ui.fulfillmentPolicyModal.getByRole("combobox", { name: "开通方式" }), "人工开通");
      await offer.selectEditSection("购买配置");
      // A saved write and a failed refresh are different outcomes. The
      // workspace must retain inputs, protect stale targets and offer retry.
      const failedRead = "**/x/idc-core/api/v1/admin/catalog/products/" + productNo;
      await ui.page.route(failedRead, route => route.request().method() === "GET"
        ? route.fulfill({ status: 503, contentType: "application/json", body: JSON.stringify({ code: 55, message: "隔离验收：暂时无法读取最新状态" }) })
        : route.continue());
      await ui.writeThrough(() => offer.save(), "/configuration", "PUT");
      await expect(ui.page.getByTestId("catalog-detail-refresh-failed")).toContainText("设置已保存，但暂时无法读取最新状态");
      await expect(ui.page.getByTestId("catalog-detail-refresh-failed")).toBeInViewport();
      await ui.screenshot("retained-offer-saved-refresh-failure");
      await ui.page.unroute(failedRead);
      await ui.page.getByTestId("catalog-detail-refresh-retry").click();
      await expect(ui.page.getByTestId("catalog-detail-refresh-failed")).toHaveCount(0);
      await expect(offer.input("配置名称（中文）")).toHaveValue("存储等级");
      await expect(prices.getByTestId("catalog-offer-price-amount").nth(1)).toHaveValue("3.00");
      await expect(prices.getByRole("button", { name: "设为零元", exact: true })).toHaveCount(2);
      await expect(prices.getByRole("switch", { name: "启用此价格", exact: true }).nth(1)).toContainText("启用");
      await ui.selectEditSection("交付设置");
      await expect(ui.page.getByTestId("catalog-fulfillment-policy-strategy")).toContainText("人工开通");
      await ui.choose(ui.fulfillmentPolicyModal.getByRole("combobox", { name: "开通方式" }), "付款后自动开通");
      await ui.writeThrough(() => ui.fulfillmentPolicyConfirm().click(), "/fulfillment-policy", "PUT");
      await ui.configureTax(); await ui.closeProductDetail(); await ui.searchNamePrefix(slug);
      await ui.openSaleStateConfirmation(productNo);
      await ui.writeThrough(() => ui.page.locator(".ant-popover:visible").getByRole("button", { name: /确\s*定/ }).click(), "/sale-state", "PUT");
      const productPath = "/x/idc-core/api/v1/admin/catalog/products/" + productNo;
      const config = async () => {
        const detail = await get(admin, productPath);
        return { detail, configuration: (await get(admin, "/x/idc-core/api/v1/admin/catalog/product-versions/" + detail.version.versionNo + "/configuration")).configuration };
      };
      const original = await config();
      const profileKey = original.configuration.customerFieldSelections.find((row: any) => row.localField?.label.zhCN === "存储等级").fieldKey;
      client = await createCustomer(f, admin, "retained-" + randomUUID());
      const publicPath = "/x/idc-core/api/v1/public/catalog/products/" + slug + "/purchase-configuration";
      const selected = async () => {
        const view = await get(client!, publicPath);
        const profile = view.fields.find((field: any) => field.labelZhCN === "存储等级");
        const values = structuredClone(view.values);
        values.find((value: any) => value.fieldToken === profile.fieldToken).optionTokens = [profile.options.find((option: any) => option.labelZhCN === "归档存储").optionToken];
        return { publicSlug: slug, configurationRevision: view.configurationRevision, values };
      };
      const paid = await paidConfiguredOrder(f, client, "retained-payment-" + randomUUID(), await selected(), "1400");
      const tasksPath = "/x/idc-core/api/v1/admin/product-fulfillment/tasks";
      let task: any;
      await expect.poll(async () => {
        const rows = (await get(admin, tasksPath + "?orderNo=" + paid.orderNo)).tasks;
        if (rows.length !== 1) return "";
        task = (await get(admin, tasksPath + "/" + rows[0].intentNo)).task;
        return task.statusCode;
      }, { timeout: 120000 }).toBe("completed");
      const historicalIntent = task.intentNo;
      const hostname = "srv-" + createHash("sha256").update("idc-resource/server/v1\0" + task.intentNo + "\0" + task.orderItemNo + "\0" + task.unitSequence).digest("hex").slice(0, 16) + ".internal";
      expect((await saved(f, f.mockState)).forms.filter((row: any) => row.hostname?.[0] === hostname)).toHaveLength(1);
      const assertPrice = async () => {
        const current = await config();
        const archive = current.configuration.merchantPriceTargets.find((row: any) => row.fieldKey === profileKey && row.candidateValue === "archive");
        expect(archive).toBeTruthy();
        expect(current.detail.prices.find((row: any) => row.status === "active" && row.configurationPriceDimensionNo === archive.dimensionNo).amountMinor).toBe("300");
        return current;
      };
      await ui.openProductDetail(productNo);
      await ui.selectEditSection("交付设置");
      await ui.choose(ui.fulfillmentPolicyModal.getByRole("combobox", { name: "对接上游／接口" }), "不使用自动化接口");
      await ui.choose(ui.fulfillmentPolicyModal.getByRole("combobox", { name: "开通方式" }), "人工开通");
      await ui.writeThrough(() => ui.fulfillmentPolicyConfirm().click(), "/fulfillment-policy", "PUT");
      const manual = await assertPrice();
      expect(manual.detail.fulfillmentBinding).toBeFalsy();
      expect(manual.configuration.customerDeclaration.fields.map((field: any) => field.label.zhCN).sort()).toEqual(original.configuration.customerDeclaration.fields.map((field: any) => field.label.zhCN).sort());
      const oldImage = manual.configuration.customerFieldSelections.find((row: any) => row.localField?.label.zhCN === "操作系统");
      expect(oldImage.localField.staticCandidates).toHaveLength(2);
      const manualQuote = await client.post(publicPath + "/quote", { data: { ...(await selected()), currencyCode: "CNY" } });
      expect((await manualQuote.json()).data.terms[0].totalAmount).toBe("1400");
      await offer.selectEditSection("购买配置"); await offer.chooseField(profileKey);
      await expect(prices.getByTestId("catalog-offer-price-amount").nth(1)).toHaveValue("3.00");
      await prices.scrollIntoViewIfNeeded();
      await ui.screenshot("retained-offer-manual-fields-prices");

      await ui.configureAutomation(); await offer.selectEditSection("购买配置");
      await expect(ui.page.getByTestId("catalog-retained-mapping-hint")).toContainText("原可售配置和价格已保留");
      const pendingBeforeSave = await config();
      await ui.page.getByTestId("catalog-retained-mapping-hint").scrollIntoViewIfNeeded();
      await ui.screenshot("retained-offer-pending-mapping");
      await offer.save();
      await expect(offer.offers).toContainText("请选择新接口对应参数");
      const pending = (await config()).configuration;
      expect(pending.version).toBe(pendingBeforeSave.configuration.version);
      expect(pending.pendingAutomationFields.length).toBeGreaterThan(1);
      for (const selection of original.configuration.customerFieldSelections.filter((row: any) => row.productFieldKey)) {
        await offer.chooseField(selection.fieldKey);
        const target = original.configuration.productDeclaration.fields.find((field: any) => field.key === selection.productFieldKey);
        const binding = offer.offers.getByRole("combobox", { name: "开通接口参数", exact: true });
        await expect(binding).toBeEnabled();
        await ui.choose(binding, `${target.label.zhCN} · ${target.key}`);
      }
      // The merchant deliberately replaces the retained information field
      // with this interface's mandatory OS selector. Nothing is auto-forwarded.
      await offer.chooseField(oldImage.fieldKey); await offer.enable();
      await ui.writeThrough(() => offer.save(), "/configuration", "PUT");
      const rebound = await assertPrice();
      expect(rebound.configuration.pendingAutomationFields || []).toHaveLength(0);
      expect(rebound.configuration.customerDeclaration.fields.filter((field: any) => field.label.zhCN === "操作系统")).toHaveLength(1);
      const finalQuote = await client.post(publicPath + "/quote", { data: { ...(await selected()), currencyCode: "CNY" } });
      expect((await finalQuote.json()).data.terms[0].totalAmount).toBe("1400");
      const historical = (await get(admin, tasksPath + "/" + historicalIntent)).task;
      expect(historical.statusCode).toBe("completed");
      const order = (await get(client, "/x/idc-core/api/v1/account/commerce/orders/" + paid.orderNo)).order;
      expect(order.invoice.paidAmountMinor).toBe("1400"); expect(order.invoice.refundedAmountMinor).toBe("0");
      expect((await saved(f, f.mockState)).forms.filter((row: any) => row.hostname?.[0] === hostname)).toHaveLength(1);
      await offer.chooseField(profileKey);
      await expect(prices.getByTestId("catalog-offer-price-amount").nth(1)).toHaveValue("3.00");
      await prices.scrollIntoViewIfNeeded();
      await ui.screenshot("retained-offer-explicit-new-mapping");
    } finally {
      if (productNo) {
        const detail = await get(admin, "/x/idc-core/api/v1/admin/catalog/products/" + productNo);
        if (detail.product.status === "active") {
          const response = await admin.put("/x/idc-core/api/v1/admin/catalog/products/" + productNo + "/sale-state", { data: { versionNo: detail.product.currentVersionNo, expectedVersion: detail.product.descriptionVersion, active: false } });
          const body = await response.json(); expect(body.code, body.message).toBe(0);
        }
      }
      await client?.dispose(); await context.close(); await admin.dispose();
    }
  });
});

// Real isolated UI deletion closes new checkout while preserving old orders.
// All requests stay in the disposable database and loopback provider fence.
import { randomUUID } from "node:crypto";
import { test } from "@host-tests/fixtures/auth";
import { expect, type APIRequestContext } from "@host-tests/support/playwright";
import { api, descriptor, fixture, get } from "../support/isolated-commerce";
import { createCustomer } from "../support/isolated-paid-order";
import { IsolatedCatalogPage } from "../pages/IsolatedCatalogPage";

const base = "/x/idc-core/api/v1/";

async function post(client: APIRequestContext, path: string, data: object) {
  const response = await client.post(base + path, { data });
  expect(response.status()).toBe(200);
  const body = await response.json();
  expect(body.code, body.message).toBe(0);
  return body.data;
}

async function finalQuote(client: APIRequestContext, slug: string) {
  const config = await get(client, base + `public/catalog/products/${slug}/purchase-configuration`);
  const values = structuredClone(config.values || []);
  for (const field of config.fields) {
    if (!field.visible || field.disabled || values.some((row: any) => row.fieldToken === field.fieldToken)) continue;
    const value: any = { fieldToken: field.fieldToken, type: field.type };
    if (field.type === "integer") value.integerValue = field.numericChoices?.[0] ?? field.minimumInteger;
    else if (field.type === "single_select") value.optionTokens = [field.options[0].optionToken];
    else continue;
    values.push(value);
  }
  const purchase = { publicSlug: slug, configurationRevision: config.configurationRevision, values };
  const publicQuote = await post(client, `public/catalog/products/${slug}/purchase-configuration/quote`, { ...purchase, currencyCode: "CNY" });
  const original = (await post(client, "account/commerce/carts", { currencyCode: "CNY" })).cart;
  const cart = (await post(client, "account/commerce/cart-items", {
    ...purchase, expectedCartVersion: original.version, quantity: 1,
    billingTermToken: publicQuote.terms[0].billingTermToken, currencyCode: "CNY", purchaseCycles: 1,
  })).cart;
  const quote = (await post(client, "account/commerce/quotes", { expectedCartVersion: cart.version, assetNumbers: [] })).quote;
  for (const item of quote.items) {
    if (item.configurationRequired && !item.configurationFrozen)
      await post(client, `account/commerce/quote-items/${item.quoteItemNo}/purchase-configuration`, purchase);
  }
  expect(quote.totalAmountMinor).toBe("1100");
  return quote;
}

test.describe("TC-80 删除商品后的结账边界", () => {
  test.skip(!descriptor, "Requires the dedicated disposable commerce database");
  test("TC-80a: 删除阻止旧报价新购，已有订单可重放且其他客户不可读取", async ({ browser }) => {
    test.setTimeout(240000);
    const f = await fixture() as Awaited<ReturnType<typeof fixture>> & { adminOrigin: string };
    const admin = await api(f, f.adminAuth);
    const context = await browser.newContext({ baseURL: f.adminOrigin });
    const allowed = new Set([f.adminOrigin, f.baseURL, f.mockOrigin]);
    await context.route("**/*", route => allowed.has(new URL(route.request().url()).origin) ? route.continue() : route.abort());
    const clients: APIRequestContext[] = [];
    const slug = "delete-checkout-" + randomUUID();
    try {
      const ui = new IsolatedCatalogPage(await context.newPage(), f.adminOrigin);
      await ui.login(); await ui.goto();
      const product = await ui.create(slug, "删除与历史订单隔离验收");
      await ui.configureAutomation(); await ui.configureFixedParameters();
      await ui.configureMonthlyPrice(); await ui.configureTax();
      await ui.closeProductDetail(); await ui.searchNamePrefix(slug);
      await ui.openSaleStateConfirmation(product.productNo);
      await ui.writeThrough(() => ui.page.locator(".ant-popover:visible").getByRole("button", { name: /确\s*定/ }).click(), "/sale-state", "PUT");
      for (let i = 0; i < 2; i++) clients.push(await createCustomer(f, admin, "delete-quote-" + randomUUID()));
      const [owner, other] = clients;
      const ownerQuote = await finalQuote(owner!, slug);
      const otherQuote = await finalQuote(other!, slug);
      const checkoutInput = { quoteNo: ownerQuote.quoteNo, idempotencyKey: "delete-preserve-" + randomUUID() };
      const oldOrder = (await post(owner!, "account/commerce/orders", checkoutInput)).checkout;
      await ui.page.getByTestId(`catalog-product-more-${product.productNo}`).click();
      await ui.page.getByTestId(`catalog-product-delete-${product.productNo}`).click();
      await expect(ui.page.locator(".ant-popover:visible")).toBeVisible();
      await ui.screenshot("delete-quoted-product-confirmation");
      await ui.writeThrough(() => ui.page.locator(".ant-popover:visible").getByRole("button", { name: /确\s*定/ }).click(), "/products/" + product.productNo, "DELETE");
      await expect(ui.page.getByTestId(`catalog-product-detail-${product.productNo}`)).toHaveCount(0);
      await ui.screenshot("delete-quoted-product-removed");
      const denied = await other!.post(base + "account/commerce/orders", { data: { quoteNo: otherQuote.quoteNo, idempotencyKey: "delete-reject-" + randomUUID() } });
      const failure = await denied.json();
      expect(failure.code).not.toBe(0);
      expect(failure.errorCode).toBe("IDC_COMMERCE_QUOTE_NOT_AVAILABLE");
      const replay = (await post(owner!, "account/commerce/orders", checkoutInput)).checkout;
      expect(replay).toMatchObject({ orderNo: oldOrder.orderNo, totalAmountMinor: "1100", reused: true });
      const detail = (await get(owner!, base + "account/commerce/orders/" + oldOrder.orderNo)).order;
      expect(detail.invoice.totalAmountMinor).toBe("1100");
      const foreign = await other!.get(base + "account/commerce/orders/" + oldOrder.orderNo);
      expect((await foreign.json()).code).not.toBe(0);
      const preparation = (await post(owner!, `account/commerce/orders/${oldOrder.orderNo}/payment-preparations`, {})).preparation;
      expect(preparation.ready).toBe(true);
      const tasks = await get(admin, base + "admin/product-fulfillment/tasks?orderNo=" + oldOrder.orderNo);
      expect(tasks.tasks).toHaveLength(0);
    } finally {
      for (const client of clients) await client.dispose();
      await context.close(); await admin.dispose();
    }
  });
});

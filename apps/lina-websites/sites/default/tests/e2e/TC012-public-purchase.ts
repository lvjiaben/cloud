import { expect, test } from "@playwright/test";

import { PublicCatalogPage } from "../pages/PublicCatalogPage";
import { mockPublicCatalog } from "../support/catalog-mock";
import { testRuntime } from "../support/runtime-config";

test.describe("TC012 官网购买交接", () => {
  test("TC012a: 配置商品只把服务端购物车凭据安全交给客户后台", async ({
    context,
    page,
  }) => {
    await mockPublicCatalog(page);
    const submittedItems: Array<Record<string, unknown>> = [];
    let opened = false;
    let credentialReachedAdd = false;

    await page.route("**/x/idc-core/api/v1/public/commerce/carts", async (route) => {
      opened = true;
      await route.fulfill({
        body: JSON.stringify({
          code: 0,
          data: { cart: { reference: "CRT_public_purchase" } },
          message: "OK",
        }),
        contentType: "application/json",
        headers: {
          "set-cookie":
            "lina_idc_guest_cart=gcart_browser_only; Path=/; HttpOnly; SameSite=Lax",
        },
        status: 200,
      });
    });
    await page.route(
      "**/x/idc-core/api/v1/public/commerce/cart-items",
      async (route) => {
        submittedItems.push(route.request().postDataJSON() as Record<string, unknown>);
        credentialReachedAdd = (route.request().headers().cookie ?? "").includes(
          "lina_idc_guest_cart=gcart_browser_only",
        );
        await route.fulfill({
          body: JSON.stringify({
            code: 0,
            data: { cart: { reference: "CRT_public_purchase" } },
            message: "OK",
          }),
          contentType: "application/json",
          status: 200,
        });
      },
    );
    await page.route(`${testRuntime.customerURL.replace(/\/$/u, "")}/**`, (route) =>
      route.fulfill({ body: "<main>customer cart</main>", contentType: "text/html" }),
    );

    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/en-US/products/managed-ssl");
    await catalog.requestConfigurationQuote();
    await catalog.addQuotedProductToCart();
    await page.waitForURL(
      new URL("/account/cart", testRuntime.customerURL).toString(),
    );

    expect(opened).toBe(true);
    expect(credentialReachedAdd).toBe(true);
    expect(submittedItems).toHaveLength(1);
    expect(submittedItems[0]).toMatchObject({
      billingTermToken: "opaque_yearly_term",
      currencyCode: "CNY",
      publicSlug: "managed-ssl",
      purchaseCycles: 1,
      quantity: 1,
    });
    expect(page.url()).not.toContain("gcart_");
    expect(page.url()).not.toContain("CRT_public_purchase");
    const cookie = (await context.cookies()).find(
      (item) => item.name === "lina_idc_guest_cart",
    );
    expect(cookie?.httpOnly).toBe(true);
    expect(await page.evaluate(() => document.cookie)).not.toContain(
      "lina_idc_guest_cart",
    );
    expect(
      await page.evaluate(() =>
        JSON.stringify({
          local: { ...localStorage },
          session: { ...sessionStorage },
        }),
      ),
    ).not.toMatch(/gcart_|CRT_public_purchase/u);
  });

  test("TC012b: 中文移动端完成配置、核价和加入购物车且无横向溢出", async ({
    page,
  }) => {
    await page.setViewportSize({ height: 844, width: 390 });
    await mockPublicCatalog(page);
    await page.route("**/x/idc-core/api/v1/public/commerce/carts", (route) =>
      route.fulfill({
        body: JSON.stringify({
          code: 0,
          data: { cart: { reference: "CRT_mobile_purchase" } },
        }),
        contentType: "application/json",
        headers: {
          "set-cookie":
            "lina_idc_guest_cart=gcart_mobile; Path=/; HttpOnly; SameSite=Lax",
        },
        status: 200,
      }),
    );
    await page.route(
      "**/x/idc-core/api/v1/public/commerce/cart-items",
      (route) =>
        route.fulfill({
          body: JSON.stringify({
            code: 0,
            data: { cart: { reference: "CRT_mobile_purchase" } },
          }),
          contentType: "application/json",
          status: 200,
        }),
    );
    await page.route(`${testRuntime.customerURL.replace(/\/$/u, "")}/**`, (route) =>
      route.fulfill({ body: "<main>customer cart</main>", contentType: "text/html" }),
    );

    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/products/managed-ssl");
    await catalog.requestConfigurationQuote();
    expect(await catalog.hasHorizontalOverflow()).toBe(false);
    await catalog.addQuotedProductToCart();
    await page.waitForURL(
      new URL("/account/cart", testRuntime.customerURL).toString(),
    );
    expect(page.url()).not.toMatch(/gcart_|CRT_mobile_purchase/u);
  });
});

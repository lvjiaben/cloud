import { test, expect } from "../support/fixtures";

test.describe("TC-9 选配分类商品侧栏", () => {
  test("TC-9a: 目录延迟不阻塞核价，分类筛选和分页使用真实返回值", async ({
    site,
    page,
  }) => {
    let release!: () => void;
    const gate = new Promise<void>((r) => (release = r));
    const categories = [
      { categoryNo: "A", nameZhCN: "云计算", nameEnUS: "Compute" },
      { categoryNo: "B", nameZhCN: "云安全", nameEnUS: "Security" },
    ];
    await page.route("**/__website/session", (r) =>
      r.fulfill({ json: { checked: true, accessToken: "test-session" } }),
    );
    await page.route("**/catalog/products**", async (route) => {
      const url = new URL(route.request().url()),
        path = url.pathname;
      let data: any;
      if (path.endsWith("/catalog/products")) {
        if (url.searchParams.get("limit") !== "20") return route.fallback();
        await gate;
        const category = url.searchParams.get("categoryNo");
        data = {
          categories,
          products:
            category === "B"
              ? [
                  {
                    publicSlug: "secure",
                    nameZhCN: "防护主机",
                    availability: "available",
                  },
                ]
              : [
                  {
                    publicSlug: url.searchParams.has("cursor") ? "two" : "one",
                    nameZhCN: url.searchParams.has("cursor")
                      ? "第二台云主机"
                      : "当前云主机",
                    availability: "available",
                  },
                ],
          nextCursor:
            category === "A" && !url.searchParams.has("cursor") ? "NEXT" : "",
        };
      } else if (path.endsWith("/one"))
        data = {
          product: {
            publicSlug: "one",
            nameZhCN: "当前云主机",
            categoryNo: "A",
            currencies: ["CNY"],
          },
        };
      else if (path.endsWith("/quote"))
        data = {
          configurationRevision: "R",
          currencyCode: "CNY",
          terms: [
            {
              billingTermToken: "M",
              labelZhCN: "月付",
              status: "final",
              totalAmount: "1000",
              minimumPurchaseCycles: 1,
            },
          ],
        };
      else
        data = {
          ready: true,
          valid: true,
          configurationRevision: "R",
          fields: [],
          values: [],
        };
      await route.fulfill({ json: { code: 0, data } });
    });
    await site.open("/configure.html?product=one");
    try {
      await expect(page.locator("#purchase-price")).toHaveText("CNY 10.00");
    } finally {
      release();
    }
    await expect(
      page.locator('.purchase-catalog [aria-current="page"]'),
    ).toHaveText("当前云主机");
    await site.moreConfigureProducts();
    await expect(page.locator(".purchase-catalog-products")).toContainText(
      "第二台云主机",
    );
    await site.configureCategory("云安全");
    await expect(page.locator(".purchase-catalog-products")).toHaveText(
      "防护主机",
    );
    await expect(page.locator(".purchase-catalog-products a")).toHaveAttribute(
      "href",
      "/configure.html?product=secure&lang=zh-CN",
    );
    await expect(page.locator("#purchase-price")).toHaveText("CNY 10.00");
    await site.screenshot("configure-sidebar-desktop");
    await site.mobile();
    await site.screenshot("configure-sidebar-mobile");
    const box = await page.locator(".purchase-catalog").boundingBox();
    expect(box!.x + box!.width).toBeLessThanOrEqual(390);
    const checkout = await page.locator(".purchase-checkout").boundingBox();
    const utility = await page.locator(".floating-tools").boundingBox();
    // Global utility actions must never sit underneath the fixed purchase bar.
    if (utility)
      expect(utility.y + utility.height).toBeLessThanOrEqual(checkout!.y);
    await expect(page.locator(".purchase-add")).toBeEnabled();
  });

  test("TC-9b: 英文目录失败可重试，空分类不伪造商品", async ({
    site,
    page,
  }) => {
    let failed = true;
    await page.route("**/__website/session", (r) =>
      r.fulfill({ json: { checked: true, accessToken: "test-session" } }),
    );
    await page.route("**/catalog/products**", async (route) => {
      const url = new URL(route.request().url());
      let data: any;
      if (url.pathname.endsWith("/catalog/products")) {
        if (url.searchParams.get("limit") !== "20") return route.fallback();
        if (failed) return route.fulfill({ status: 503, json: { code: 503 } });
        data = {
          categories: [{ categoryNo: "A", nameEnUS: "Compute" }],
          products: [],
          nextCursor: "",
        };
      } else if (url.pathname.endsWith("/one"))
        data = {
          product: {
            nameEnUS: "Cloud server",
            categoryNo: "A",
            currencies: ["CNY"],
          },
        };
      else if (url.pathname.endsWith("/quote"))
        data = {
          currencyCode: "CNY",
          terms: [
            {
              billingTermToken: "M",
              labelEnUS: "Monthly",
              status: "final",
              totalAmount: "1000",
              minimumPurchaseCycles: 1,
            },
          ],
        };
      else
        data = {
          ready: true,
          valid: true,
          configurationRevision: "R",
          fields: [],
          values: [],
        };
      await route.fulfill({ json: { code: 0, data } });
    });
    await site.open("/configure.html?product=one&lang=en-US");
    await expect(page.locator(".purchase-catalog h2")).toHaveText(
      "Product categories",
    );
    await expect(page.locator(".catalog-status")).toContainText(
      "Unable to load. Please retry.",
    );
    await expect(page.locator(".purchase-add")).toBeEnabled();
    await site.screenshot("configure-sidebar-error");
    failed = false;
    await site.retryConfigureCatalog("Retry");
    await expect(page.locator(".catalog-status")).toHaveText(
      "No products found.",
    );
    await expect(page.locator(".purchase-categories")).toContainText("Compute");
  });
  test("TC-9c: 手机返回顶部不误加购，安全区变高后保持可操作", async ({
    site,
    page,
  }) => {
    let additions = 0;
    const existing = {
      cartItemNo: "existing",
      productNameZhCN: "已有商品",
      quantity: 1,
      purchaseCycles: 1,
      version: 1,
    };
    const cart = { version: 1, currencyCode: "CNY", items: [existing] };
    await page.route("**/__website/session", (r) =>
      r.fulfill({ json: { checked: true, accessToken: "test-session" } }),
    );
    await page.route("**/x/idc-core/api/v1/**", async (route) => {
      const url = new URL(route.request().url());
      let data: any;
      if (url.pathname.endsWith("/commerce/cart") || url.pathname.endsWith("/commerce/carts")) data = { cart };
      else if (url.pathname.endsWith("/commerce/cart-items")) {
        additions++;
        cart.items.push({
          cartItemNo: "new",
          productNameZhCN: "当前云主机",
          quantity: 1,
          purchaseCycles: 1,
          version: 1,
        });
        data = { cart };
      } else if (url.pathname.endsWith("/catalog/products"))
        data = {
          categories: [{ categoryNo: "A", nameZhCN: "云计算" }],
          products: [
            {
              publicSlug: "one",
              nameZhCN: "当前云主机",
              availability: "available",
            },
          ],
        };
      else if (url.pathname.endsWith("/one"))
        data = {
          product: {
            publicSlug: "one",
            nameZhCN: "当前云主机",
            categoryNo: "A",
            currencies: ["CNY"],
          },
        };
      else if (url.pathname.endsWith("/quote"))
        data = {
          configurationRevision: "R",
          currencyCode: "CNY",
          terms: [
            {
              billingTermToken: "M",
              labelZhCN: "月付",
              status: "final",
              totalAmount: "1000",
              minimumPurchaseCycles: 1,
            },
          ],
        };
      else
        data = {
          ready: true,
          valid: true,
          configurationRevision: "R",
          fields: [],
          values: [],
        };
      await route.fulfill({ json: { code: 0, data } });
    });
    for (const width of [390, 320]) {
      await page.setViewportSize({ width, height: 844 });
      await site.open("/configure.html?product=one");
      await expect(page.locator("#purchase-price")).toHaveText("CNY 10.00");
      // Model a larger device safe area; observe the resulting checkout size.
      await page.addStyleTag({
        content: ".purchase-checkout { padding-bottom: 46px; }",
      });
      await site.key("Control+End");
      await expect
        .poll(() => page.evaluate(() => scrollY))
        .toBeGreaterThan(200);
      await expect
        .poll(async () => {
          const utility = await page.locator(".floating-tools").boundingBox();
          const checkout = await page
            .locator(".purchase-checkout")
            .boundingBox();
          return Boolean(
            utility && checkout && utility.y + utility.height <= checkout.y,
          );
        })
        .toBe(true);
      await site.backToTop();
      await expect.poll(() => page.evaluate(() => scrollY)).toBe(0);
      expect(additions).toBe(0);
      expect(cart.items).toEqual([existing]);
      await expect(page.locator(".purchase-add")).toBeEnabled();
    }
    await site.open("/cart.html");
    await expect(page.locator(".cart-items article")).toHaveCount(1);
    await expect(page.locator(".cart-items")).toContainText("已有商品");
    await site.open("/configure.html?product=one");
    await expect(page.locator(".purchase-add")).toBeEnabled();
    await site.addToCart();
    await expect(page.locator(".cart-items article")).toHaveCount(2);
    await expect(page.locator(".cart-items")).toContainText("当前云主机");
    expect(additions).toBe(1);
    await site.screenshot("mobile-back-top-cart");
  });
});

import { test, expect } from "../support/fixtures";
test.describe("TC-5 官网配置与购物车", () => {
  test("TC-5a: 官网配置以服务端报价加入已登录购物车并移除", async ({
    site,
    page,
  }) => {
    let added = false;
    const actions: any[] = [];
    const view = {
      ready: true,
      configurationRevision: "R".repeat(43),
      fields: [
        {
          fieldToken: "cpu",
          type: "integer",
          labelZhCN: "CPU 核数",
          labelEnUS: "CPU cores",
          visible: true,
          required: true,
          minimumInteger: "1",
          maximumInteger: "8",
        },
      ],
      values: [{ fieldToken: "cpu", type: "integer", integerValue: "2" }],
    };
    await page.route("**/__website/session", (route) =>
      route.fulfill({ json: { checked: false } }),
    );
    await page.route(
      "http://127.0.0.1:6032/auth/website-session?**",
      (route) => {
        const nonce = new URL(route.request().url()).searchParams.get("nonce");
        return route.fulfill({
          contentType: "text/html",
          body: `<script>parent.postMessage({type:'lina-website-session',nonce:${JSON.stringify(nonce)},accessToken:'test-session'},'http://127.0.0.1:16881')</script>`,
        });
      },
    );
    await page.route("**/x/idc-core/api/v1/**", (route) => {
      const req = route.request(),
        path = new URL(req.url()).pathname;
      let data: any;
      if (path.endsWith("/catalog/products/test"))
        data = {
          product: {
            publicSlug: "test",
            nameZhCN: "测试云主机",
            currencies: ["CNY"],
          },
        };
      else if (path.endsWith("/purchase-configuration")) data = view;
      else if (path.endsWith("/evaluate")) {
        actions.push(req.postDataJSON());
        data = {
          ...view,
          values: req.postDataJSON().values,
          valid: true,
          issues: [],
        };
      } else if (path.endsWith("/quote"))
        data = {
          configurationRevision: view.configurationRevision,
          currencyCode: "CNY",
          terms: [
            {
              billingTermToken: "MONTH",
              labelZhCN: "月付",
              totalAmount: "12345",
              minimumPurchaseCycles: 1,
              status: "final",
            },
            {
              billingTermToken: "YEAR",
              labelZhCN: "年付",
              totalAmount: "99900",
              minimumPurchaseCycles: 1,
              status: "final",
            },
          ],
        };
      else if (path.endsWith("/carts")) data = { cart: { cartNo:"CART", version:1, currencyCode:"CNY", items:[] } };
      else if (path.endsWith("/guest-cart-merge")) data = { merged: false };
      else if (path.endsWith("/cart-items")) {
        actions.push(req.postDataJSON());
        added = true;
        data = { cart: { cartNo:"CART",version:2,currencyCode:"CNY",items:[{cartItemNo:"ITEM",configurationReference:"CONFIG",publicSlug:"test",quantity:1,purchaseCycles:1,version:1}] } };
      } else if (path.endsWith("/cart-items/ITEM")) {
        expect(req.method()).toBe("DELETE");
        expect(req.url()).toContain("expectedCartVersion=2");
        added = false;
        data = { cart: { version: 3, currencyCode: "CNY", items: [] } };
      } else if (path.endsWith("/cart"))
        data = {
          cart: {
            reference: "guest",
            version: added ? 2 : 1,
            currencyCode: "CNY",
            items: added
              ? [
                  {
                    cartItemNo: "ITEM",
                    configurationReference: "CONFIG",
                    publicSlug: "test",
                    productNameZhCN: "测试云主机",
                    quantity: 1,
                    purchaseCycles: 1,
                    version: 1,
                  },
                ]
              : [],
          },
        };
      else return route.fallback();
      return route.fulfill({ json: { code: 0, data } });
    });
    await site.open("/configure.html?product=test");
    await expect(page.locator(".purchase-terms")).toContainText("CNY 123.45");
    await expect(page.getByLabel("CPU 核数")).toHaveValue("2");
    await site.configureNumber("CPU 核数", "4");
    await expect(page.getByLabel("CPU 核数")).toHaveValue("4");
    await expect(page.locator(".purchase-terms")).toContainText("CNY 123.45");
    await page
      .getByRole("radio", { name: "年付 CNY 999.00", exact: true })
      .check();
    await expect(page.locator("#purchase-price")).toHaveText("CNY 999.00");
    await page.getByRole("button", { name: "加入购物车", exact: true }).click();
    await expect(page).toHaveURL(/\/cart.html/);
    await expect(page.locator(".cart-items")).toContainText("测试云主机");
    expect(actions.at(-1).values[0].integerValue).toBe("4");
    expect(actions.at(-1).billingTermToken).toBe("YEAR");
    await site.screenshot("account-cart");
    await page.getByRole("button", { name: "移除", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "购物车还是空的", exact: true }),
    ).toBeVisible();
  });
  test("TC-5b: 受信任会话、合并、核价与幂等订单，错误保留购物车", async ({
    site,
    page,
  }) => {
    let failOrder = true;
    const submitted: string[] = [];
    let merged = 0;
    const quote = {
      quoteNo: "QUOTE",
      cartVersion: 2,
      currencyCode: "CNY",
      subtotalMinor: "100",
      discountMinor: "0",
      taxAmountMinor: "0",
      totalAmountMinor: "100",
      expiresAt: Date.now() + 60000,
      items: [],
    };
    const cart = {
      cartNo: "CART",
      version: 2,
      currencyCode: "CNY",
      items: [
        {
          cartItemNo: "ITEM",
          productNameZhCN: "真实配置商品",
          quantity: 1,
          purchaseCycles: 1,
          version: 1,
        },
      ],
    };
    await page.route("**/__website/session", (route) =>
      route.fulfill({ json: { checked: false } }),
    );
    await page.route(
      "http://127.0.0.1:6032/auth/website-session?**",
      (route) => {
        const nonce = new URL(route.request().url()).searchParams.get("nonce");
        return route.fulfill({
          contentType: "text/html",
          body: `<script>parent.postMessage({type:'lina-website-session',nonce:${JSON.stringify(nonce)},accessToken:'test-session'},'http://127.0.0.1:16881')</script>`,
        });
      },
    );
    await page.route("**/account/commerce/**", (route) => {
      const req = route.request();
      expect(req.headers().authorization).toBe("Bearer test-session");
      let data: any;
      if (req.url().endsWith("guest-cart-merge")) {
        merged++;
        data = { merged: true };
      } else if (req.url().endsWith("/cart")) data = { cart };
      else if (req.url().endsWith("/quotes")) data = { quote };
      else if (req.url().endsWith("/orders")) {
        submitted.push(req.postDataJSON().idempotencyKey);
        if (failOrder)
          return route.fulfill({ status: 503, json: { code: 503 } });
        data = { checkout: { orderNo: "ORDER", invoiceNo: "INVOICE" } };
      } else return route.fallback();
      return route.fulfill({ json: { code: 0, data } });
    });
    await page.route(
      "http://127.0.0.1:6032/account/billing/invoices/**",
      (route) =>
        route.fulfill({
          contentType: "text/html",
          body: "<h1>Invoice INVOICE</h1>",
        }),
    );
    await site.open("/cart.html");
    await page.getByRole("button", { name: "去结算", exact: true }).click();
    await expect(page.locator(".cart-total")).toContainText("CNY 1.00");
    await page.getByRole("button", { name: "立即结账", exact: true }).click();
    await expect(page.locator("#commerce-page .status")).toContainText(
      "操作未完成",
    );
    await expect(page.locator(".cart-items")).toContainText("真实配置商品");
    failOrder = false;
    await page.getByRole("button", { name: "立即结账", exact: true }).click();
    await expect(page).toHaveURL(
      "http://127.0.0.1:6032/account/billing/invoices/INVOICE?pay=1",
    );
    expect(submitted).toHaveLength(2);
    expect(submitted[0]).toBe(submitted[1]);
    expect(merged).toBeGreaterThan(0);
  });
});

test("TC-5c: 错误 nonce 被忽略，账号变化阻止沿用旧报价下单", async ({
  site,
  page,
}) => {
  let session = "account-a",
    orders = 0;
  const cart = {
    version: 1,
    currencyCode: "CNY",
    items: [
      {
        cartItemNo: "ITEM",
        productNameZhCN: "会话隔离商品",
        quantity: 1,
        purchaseCycles: 1,
        version: 1,
      },
    ],
  };
  const quote = {
    quoteNo: "QUOTE-A",
    currencyCode: "CNY",
    subtotalMinor: "100",
    discountMinor: "0",
    taxAmountMinor: "0",
    totalAmountMinor: "100",
    expiresAt: Date.now() + 60000,
    items: [],
  };
  await page.route("**/__website/session", (route) =>
    route.fulfill({ json: { checked: false } }),
  );
  await page.route("http://127.0.0.1:6032/auth/website-session?**", (route) => {
    const nonce = new URL(route.request().url()).searchParams.get("nonce");
    return route.fulfill({
      contentType: "text/html",
      body: `<script>parent.postMessage({type:'lina-website-session',nonce:'wrong-nonce',accessToken:'untrusted'},'http://127.0.0.1:16881');setTimeout(()=>parent.postMessage({type:'lina-website-session',nonce:${JSON.stringify(nonce)},accessToken:${JSON.stringify(session)}},'http://127.0.0.1:16881'),50)</script>`,
    });
  });
  await page.route("**/account/commerce/**", (route) => {
    expect(route.request().headers().authorization).not.toContain("untrusted");
    const path = new URL(route.request().url()).pathname;
    if (path.endsWith("/orders")) orders++;
    return route.fulfill({
      json: {
        code: 0,
        data: path.endsWith("/quotes")
          ? { quote }
          : path.endsWith("guest-cart-merge")
            ? { merged: false }
            : { cart },
      },
    });
  });
  await site.open("/cart.html");
  await page.getByRole("button", { name: "去结算", exact: true }).click();
  await expect(page.locator(".cart-total")).toContainText("CNY 1.00");
  await expect(
    page.getByRole("heading", { name: "结算详情", exact: true }),
  ).toBeVisible();
  session = "account-b";
  await page.getByRole("button", { name: "立即结账", exact: true }).click();
  await expect(page.locator("#commerce-page .status")).toContainText(
    "登录状态已改变",
  );
  await expect(page.locator(".cart-total")).toBeEmpty();
  expect(orders).toBe(0);
});

test("TC-5d: 过期报价可重新核价，不能提交旧订单", async ({ site, page }) => {
  let quoteCount = 0,
    orderCount = 0;
  const cart = {
    version: 1,
    currencyCode: "CNY",
    items: [
      {
        cartItemNo: "ITEM",
        productNameZhCN: "过期报价商品",
        quantity: 1,
        purchaseCycles: 1,
        version: 1,
      },
    ],
  };
  await page.clock.install();
  await page.route("**/__website/session", (r) =>
    r.fulfill({ json: { checked: true, accessToken: "test-session" } }),
  );
  await page.route("**/account/commerce/**", (r) => {
    const path = new URL(r.request().url()).pathname;
    let data: any = { cart };
    if (path.endsWith("/quotes")) {
      quoteCount++;
      data = {
        quote: {
          quoteNo: "QUOTE-" + quoteCount,
          currencyCode: "CNY",
          subtotalMinor: "100",
          discountMinor: "0",
          taxAmountMinor: "0",
          totalAmountMinor: "100",
          expiresAt: Date.now() + (quoteCount === 1 ? 60000 : 600000),
          items: [],
        },
      };
    }
    if (path.endsWith("/orders")) orderCount++;
    return r.fulfill({ json: { code: 0, data } });
  });
  await site.open("/cart.html");
  await page.getByRole("button", { name: "去结算", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "结算详情", exact: true }),
  ).toBeVisible();
  await page.clock.fastForward(120000);
  await page.getByRole("button", { name: "立即结账", exact: true }).click();
  await expect(page.locator("#commerce-page .status")).toContainText(
    "报价已失效",
  );
  await page.getByRole("button", { name: "重新核价", exact: true }).click();
  await expect(page.locator(".cart-total")).toContainText("CNY 1.00");
  expect(quoteCount).toBe(2);
  expect(orderCount).toBe(0);
});

test("TC-5e: 文件配置遵循上传限额，上传成功后使用服务端令牌核价", async ({
  site,
  page,
}) => {
  const uploadToken = "CFU_" + "A".repeat(32);
  let uploaded = false,
    reserved = 0,
    evaluated: any;
  const field = {
    fieldToken: "proof",
    type: "file",
    labelZhCN: "证明文件",
    visible: true,
    required: true,
    uploadPolicy: {
      maximumFiles: 1,
      maximumFileBytes: 4,
      maximumTotalBytes: 4,
      allowedExtensions: [".txt"],
    },
  };
  const view = {
    ready: true,
    configurationRevision: "R".repeat(43),
    fields: [field],
    values: [],
  };
  await page.route("**/__website/session", (route) =>
    route.fulfill({ json: { checked: true, accessToken: "upload-session" } }),
  );
  await page.route("**/x/idc-core/api/v1/**", (route) => {
    const req = route.request(),
      path = new URL(req.url()).pathname;
    let data: any;
    if (path.endsWith("/catalog/products/upload"))
      data = { product: { nameZhCN: "文件配置商品", currencies: ["CNY"] } };
    else if (path.endsWith("/purchase-configuration")) data = view;
    else if (path.endsWith("/file-uploads")) {
      reserved++;
      expect(req.headers().authorization).toBe("Bearer upload-session");
      expect(req.postDataJSON().sizeBytes).toBe(3);
      data = { uploadToken };
    } else if (path.endsWith("/content")) {
      expect(req.method()).toBe("PUT");
      expect(req.postDataBuffer()?.toString()).toBe("abc");
      uploaded = true;
      data = { status: "ready", fileToken: uploadToken };
    } else if (path.endsWith("/evaluate")) {
      evaluated = req.postDataJSON();
      data = { ...view, values: evaluated.values, valid: true };
    } else if (path.endsWith("/quote"))
      data = {
        configurationRevision: view.configurationRevision,
        currencyCode: "CNY",
        terms: [],
      };
    else return route.fallback();
    return route.fulfill({ json: { code: 0, data } });
  });
  await site.open("/configure.html?product=upload");
  await page.getByLabel("证明文件", { exact: false }).setInputFiles({
    name: "too-large.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("abcde"),
  });
  await expect(page.locator("#commerce-page .status")).toContainText(
    "文件大小或数量超出允许范围。",
  );
  expect(reserved).toBe(0);
  await page.getByLabel("证明文件", { exact: false }).setInputFiles({
    name: "proof.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("abc"),
  });
  await expect.poll(() => uploaded).toBe(true);
  expect(uploaded).toBe(true);
  await expect(page.locator(".price-status")).toContainText(
    "暂无可购买的计费周期。",
  );
  expect(evaluated.values[0].fileTokens).toEqual([uploadToken]);
});

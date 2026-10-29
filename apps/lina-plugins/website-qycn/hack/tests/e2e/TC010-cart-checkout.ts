import { test, expect } from "../support/fixtures";

const revision = "R".repeat(43);
const schema = {
  ready: true,
  configurationRevision: revision,
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
const terms = [
  {
    billingTermToken: "MONTH",
    labelZhCN: "月付",
    labelEnUS: "Monthly",
    totalAmount: "2100",
    minimumPurchaseCycles: 1,
    status: "final",
  },
  {
    billingTermToken: "YEAR",
    labelZhCN: "年付",
    labelEnUS: "Yearly",
    totalAmount: "20000",
    minimumPurchaseCycles: 1,
    status: "final",
  },
];

test("TC-10a 未登录加购无写入，登录取消及回跳保留配置周期且不自动加购", async ({
  site,
  page,
}) => {
  let signedIn = false,
    writes: any[] = [];
  let cart: any = { version: 1, currencyCode: "CNY", items: [] };
  await page.route("**/__website/session", (r) =>
    r.fulfill({
      json: { checked: true, accessToken: signedIn ? "customer-session" : "" },
    }),
  );
  await page.route("http://127.0.0.1:6032/auth/website-session?**", (r) =>
    r.fulfill({ contentType: "text/html", body: "<h1>请登录</h1>" }),
  );
  await page.route("**/x/idc-core/api/v1/**", (r) => {
    const req = r.request(),
      path = new URL(req.url()).pathname;
    let data: any;
    if (path.endsWith("/catalog/products/test"))
      data = {
        product: {
          publicSlug: "test",
          nameZhCN: "弹性云服务器",
          currencies: ["CNY"],
        },
      };
    else if (path.endsWith("/purchase-configuration")) data = schema;
    else if (path.endsWith("/evaluate"))
      data = { ...schema, valid: true, values: req.postDataJSON().values };
    else if (path.endsWith("/quote"))
      data = { configurationRevision: revision, currencyCode: "CNY", terms };
    else if (path.endsWith("/commerce/carts")) {
      writes.push(path);
      data = { cart };
    } else if (path.endsWith("/commerce/cart-items")) {
      writes.push(req.postDataJSON());
      cart = {
        ...cart,
        version: 2,
        items: [
          {
            cartItemNo: "ITEM",
            configurationReference: "CFG",
            publicSlug: "test",
            productNameZhCN: "弹性云服务器",
            quantity: 1,
            purchaseCycles: 1,
            version: 1,
          },
        ],
      };
      data = { cart };
    } else if (path.endsWith("/commerce/cart")) data = { cart };
    else if (path.endsWith("/guest-cart-merge")) data = { merged: false };
    else return r.fallback();
    return r.fulfill({ json: { code: 0, data } });
  });
  await site.open("/configure.html?product=test");
  await site.configureNumber("CPU 核数", "4");
  await page
    .getByRole("radio", { name: "年付 CNY 200.00", exact: true })
    .check();
  await page.getByRole("button", { name: "加入购物车", exact: true }).click();
  await expect(page).toHaveURL(/6032\/auth\/website-session\?login=1/);
  expect(writes).toEqual([]);
  // Cancelling login and returning to the same origin retains the draft.
  await site.open("/configure.html?product=test");
  await expect(page.getByRole("spinbutton", { name: "CPU 核数 *", exact: true })).toHaveValue("4");
  await expect(
    page.getByRole("radio", { name: "年付 CNY 200.00", exact: true }),
  ).toBeChecked();
  signedIn = true;
  await site.open("/configure.html?product=test");
  await expect(page.getByRole("spinbutton", { name: "CPU 核数 *", exact: true })).toHaveValue("4");
  await expect(page.locator("#purchase-price")).toHaveText("CNY 200.00");
  expect(writes).toEqual([]);
  await page.getByRole("button", { name: "加入购物车", exact: true }).click();
  await expect(page).toHaveURL(/\/cart.html/);
  expect(writes).toHaveLength(2);
  expect(writes[1]).toMatchObject({
    billingTermToken: "YEAR",
    values: [{ fieldToken: "cpu", type: "integer", integerValue: "4" }],
  });
  await site.open("/cart.html?lang=en-US");
  await expect(page.locator(".cart-configuration")).toContainText("CPU cores");
  await expect(page.locator(".cart-configuration")).toContainText("4");
});

test("TC-10b 配置卡、数量失败重试、真实优惠和结算支付交接", async ({
  site,
  page,
}) => {
  let failUpdate = true,
    version = 2,
    quantity = 1,
    quoteNo = 0,
    discount = false,
    quoted: any;
  const mutations: any[] = [];
  const item = () => ({
    cartItemNo: "ITEM",
    configurationReference: "CFG",
    publicSlug: "test",
    productNameZhCN: "弹性云服务器 · 华东",
    productNameEnUS: "Elastic cloud · East China",
    quantity,
    purchaseCycles: 1,
    version,
    statusCode: "active",
    chargeTypeCode: "recurring",
    periodUnitCode: "month",
    periodCount: 1,
    configuration: [
      {
        labelZhCN: "CPU",
        labelEnUS: "CPU",
        valueZhCN: "4 核",
        valueEnUS: "4 cores",
      },
      {
        labelZhCN: "内存",
        labelEnUS: "Memory",
        valueZhCN: "8 GB",
        valueEnUS: "8 GB",
      },
      {
        labelZhCN: "地区",
        labelEnUS: "Region",
        valueZhCN: "上海一区",
        valueEnUS: "Shanghai 1",
      },
      {
        labelZhCN: "操作系统",
        labelEnUS: "Operating system",
        valueZhCN: "Ubuntu 24.04",
        valueEnUS: "Ubuntu 24.04",
      },
    ],
  });
  const cart = () => ({
    version,
    currencyCode: "CNY",
    items: [item()],
    latestQuote: quoted,
  });
  await page.route("**/__website/session", (r) =>
    r.fulfill({ json: { checked: true, accessToken: "customer-session" } }),
  );
  await page.route("**/x/idc-payment/api/v1/account/payment-channels?**", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          channels: [
            { channelNo: "ALI", displayName: "支付宝", modes: ["page"] },
            { channelNo: "APP_ONLY", displayName: "应用支付", modes: ["app"] },
          ],
        },
      },
    }),
  );
  await page.route("**/x/idc-core/api/v1/**", (r) => {
    const req = r.request(),
      path = new URL(req.url()).pathname;
    let data: any;
    if (!path.includes("/public/"))
      expect(req.headers().authorization).toBe("Bearer customer-session");
    if (path.endsWith("/commerce/cart")) data = { cart: cart() };
    else if (path.endsWith("/guest-cart-merge")) data = { merged: false };
    else if (path.endsWith("/commerce/cart-items/ITEM")) {
      expect(req.method()).toBe("PUT");
      mutations.push(req.postDataJSON());
      if (failUpdate) return r.fulfill({ status: 503, json: { code: 503 } });
      expect(req.postDataJSON()).toMatchObject({
        expectedCartVersion: version,
        expectedItemVersion: version,
        quantity: 2,
      });
      version++;
      quantity = 2;
      quoted = undefined;
      data = { cart: cart() };
    } else if (path.endsWith("/commerce/quotes")) {
      quoteNo++;
      discount = req.postDataJSON().assetNumbers.includes("COUPON");
      quoted = {
        quoteNo: "QUOTE-" + quoteNo,
        cartVersion: version,
        currencyCode: "CNY",
        subtotalMinor: quantity === 1 ? "7900" : "15800",
        discountMinor: discount ? "800" : "0",
        taxAmountMinor: "0",
        totalAmountMinor: discount
          ? "15000"
          : quantity === 1
            ? "7900"
            : "15800",
        expiresAt: Date.now() + 60000,
        items: [
          {
            cartItemNo: "ITEM",
            quoteItemNo: "QI",
            totalAmountMinor: discount
              ? "15000"
              : quantity === 1
                ? "7900"
                : "15800",
          },
        ],
      };
      data = { quote: quoted };
    } else if (path.endsWith("/marketing/coupons/claims")) {
      expect(req.postDataJSON().code).toBe("TEST2026");
      data = { item: { assetNo: "COUPON" } };
    } else if (path.endsWith("/marketing/coupons")) data = { items: [] };
    else if (path.endsWith("/billing/overview"))
      data = {
        overview: {
          balances: [
            {
              accountTypeCode: "customer_balance",
              currencyCode: "CNY",
              statusCode: "active",
              availableBalanceMinor: "20000",
            },
          ],
        },
      };
    else if (path.endsWith("/commerce/orders")) {
      expect(req.postDataJSON().quoteNo).toBe(quoted.quoteNo);
      data = { checkout: { invoiceNo: "INV-UI" } };
    } else return r.fallback();
    return r.fulfill({ json: { code: 0, data } });
  });
  await page.route("http://127.0.0.1:6032/account/billing/invoices/**", (r) =>
    r.fulfill({ body: "Invoice payment" }),
  );
  await site.open("/cart.html");
  await expect(page.locator(".cart-total")).toContainText("CNY 79.00");
  await expect(page.locator(".cart-configuration")).toContainText("上海一区");
  await page.getByRole("button", { name: "增加数量", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "去结算", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "更新", exact: true }).click();
  await expect(page.locator(".cart-status")).toContainText("操作未完成");
  await expect(page.getByLabel("数量", { exact: true })).toHaveValue("2");
  failUpdate = false;
  await page.getByRole("button", { name: "更新", exact: true }).click();
  await expect(page.locator(".cart-total")).toContainText("CNY 158.00");
  expect(mutations).toHaveLength(2);
  await page.getByLabel("优惠码", { exact: true }).fill("TEST2026");
  await page.getByRole("button", { name: "应用", exact: true }).click();
  await expect(page.locator(".cart-total")).toContainText("CNY 150.00");
  await site.screenshot("cart-reference-desktop");
  await page.getByRole("button", { name: "去结算", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "结算详情", exact: true }),
  ).toBeVisible();
  await expect(page.locator(".cart-detail-configuration")).toContainText(
    "上海一区",
  );
  await expect(page.getByText("应用支付", { exact: true })).toHaveCount(0);
  await page.getByRole("radio", { name: "支付宝", exact: true }).check();
  await site.screenshot("checkout-reference-desktop");
  await page.getByRole("button", { name: "立即结账", exact: true }).click();
  await expect(page).toHaveURL(
    "http://127.0.0.1:6032/account/billing/invoices/INV-UI?pay=1&paymentChannel=ALI",
  );
});

test("TC-10c 英文手机购物车配置与固定结算栏无溢出遮挡", async ({
  site,
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  const item = {
    cartItemNo: "ITEM",
    publicSlug: "test",
    productNameEnUS: "Elastic cloud server",
    quantity: 1,
    purchaseCycles: 1,
    version: 1,
    statusCode: "active",
    periodUnitCode: "month",
    periodCount: 1,
    configuration: [
      { labelEnUS: "Region", valueEnUS: "Shanghai" },
      { labelEnUS: "Memory", valueEnUS: "8 GB" },
    ],
  };
  const quote = {
    quoteNo: "Q",
    currencyCode: "CNY",
    subtotalMinor: "7900",
    discountMinor: "0",
    taxAmountMinor: "0",
    totalAmountMinor: "7900",
    expiresAt: Date.now() + 60000,
    items: [{ cartItemNo: "ITEM", totalAmountMinor: "7900" }],
  };
  await page.route("**/__website/session", (r) =>
    r.fulfill({ json: { checked: true, accessToken: "customer-session" } }),
  );
  await page.route("**/account/commerce/**", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: r.request().url().endsWith("/quotes")
          ? { quote }
          : {
              cart: {
                version: 1,
                currencyCode: "CNY",
                items: [item],
                latestQuote: quote,
              },
            },
      },
    }),
  );
  await site.open("/cart.html?lang=en-US");
  await expect(
    page.getByRole("heading", { name: "Shopping cart checkout" }),
  ).toBeVisible();
  await expect(page.locator(".cart-configuration")).toContainText("Shanghai");
  await expect(
    page.getByRole("link", { name: "✎ Edit configuration" }),
  ).toHaveAttribute("href", /cartItemNo=ITEM/);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= innerWidth,
    ),
  ).toBe(true);
  await site.screenshot("cart-reference-mobile-en");
  await site.key("Control+End");
  const bar = await page.locator(".cart-checkout").boundingBox(),
    tools = await page.locator(".floating-tools").boundingBox();
  expect(tools!.y + tools!.height).toBeLessThanOrEqual(bar!.y);
  await expect(
    page.getByRole("button", { name: "Proceed to checkout", exact: true }),
  ).toBeInViewport();
});

test("TC-10d 加购响应丢失后仍重试原版本，不能重复加入", async ({
  site,
  page,
}) => {
  let version = 1,
    commits = 0;
  const versions: number[] = [];
  await page.route("**/__website/session", (r) =>
    r.fulfill({ json: { checked: true, accessToken: "customer-session" } }),
  );
  await page.route("**/x/idc-core/api/v1/**", (r) => {
    const req = r.request(),
      path = new URL(req.url()).pathname;
    let data: any;
    if (path.endsWith("/catalog/products/test"))
      data = { product: { nameZhCN: "响应丢失商品", currencies: ["CNY"] } };
    else if (path.endsWith("/purchase-configuration")) data = schema;
    else if (path.endsWith("/evaluate"))
      data = { ...schema, values: req.postDataJSON().values, valid: true };
    else if (path.endsWith("/quote"))
      data = { configurationRevision: revision, currencyCode: "CNY", terms };
    else if (path.endsWith("/commerce/carts"))
      data = {
        cart: { cartNo: "CART", version, currencyCode: "CNY", items: [] },
      };
    else if (path.endsWith("/commerce/cart"))
      data = {
        cart: { cartNo: "CART", version, currencyCode: "CNY", items: [] },
      };
    else if (path.endsWith("/commerce/cart-items")) {
      versions.push(req.postDataJSON().expectedCartVersion);
      if (req.postDataJSON().expectedCartVersion !== version)
        return r.fulfill({ status: 409, json: { code: 409 } });
      commits++;
      version++;
      return r.fulfill({ status: 503, json: { code: 503 } });
    } else return r.fallback();
    return r.fulfill({ json: { code: 0, data } });
  });
  await site.open("/configure.html?product=test");
  await expect(page.locator(".purchase-add")).toBeEnabled();
  await page.locator(".purchase-add").click();
  await expect(page.locator(".purchase-form .status")).toContainText(
    "再次加购前请先检查购物车",
  );
  await page.locator(".purchase-add").click();
  await expect.poll(() => versions.length).toBe(2);
  expect(versions).toEqual([1, 1]);
  expect(commits).toBe(1);
});

test("TC-10e 编辑恢复原配置周期，双版本PUT替换原条目且不再加购", async ({
  site,
  page,
}) => {
  const saved = {
    publicSlug: "test",
    configurationRevision: revision,
    values: [{ fieldToken: "cpu", type: "integer", integerValue: "4" }],
    billingTermToken: "YEAR",
    currencyCode: "CNY",
    purchaseCycles: 1,
  };
  await page.addInitScript(
    (draft) =>
      sessionStorage.setItem(
        "website-qycn-purchase-drafts",
        JSON.stringify({ CFG: draft }),
      ),
    saved,
  );
  const mutations: any[] = [];
  let ownedCartReads = 0;
  await page.route("**/__website/session", (r) =>
    r.fulfill({ json: { checked: true, accessToken: "customer-session" } }),
  );
  await page.route("**/x/idc-core/api/v1/**", (r) => {
    const req = r.request(),
      path = new URL(req.url()).pathname;
    let data: any;
    if (path.endsWith("/catalog/products/test"))
      data = { product: { nameZhCN: "编辑商品", currencies: ["CNY"] } };
    else if (path.endsWith("/purchase-configuration")) data = schema;
    else if (path.endsWith("/evaluate"))
      data = { ...schema, values: req.postDataJSON().values, valid: true };
    else if (path.endsWith("/quote"))
      data = { configurationRevision: revision, currencyCode: "CNY", terms };
    else if (path.endsWith("/commerce/carts"))
      throw new Error("Editing must not open the cart again");
    else if (path.endsWith("/commerce/cart")) {
      ownedCartReads++;
      data = {
        cart: {
          version: 7,
          currencyCode: "CNY",
          items: [
            {
              cartItemNo: "ITEM",
              configurationReference: "CFG",
              publicSlug: "test",
              quantity: 3,
              version: 4,
            },
          ],
        },
      };
    } else if (path.endsWith("/commerce/cart-items/ITEM")) {
      expect(ownedCartReads).toBe(1);
      expect(req.method()).toBe("PUT");
      mutations.push(req.postDataJSON());
      data = {
        cart: {
          items: [{ cartItemNo: "ITEM", configurationReference: "CFG2" }],
        },
      };
    } else if (path.endsWith("/commerce/cart-items"))
      throw new Error("Editing must not append a new item");
    else return r.fallback();
    return r.fulfill({ json: { code: 0, data } });
  });
  await site.open("/configure.html?product=test&cartItemNo=ITEM");
  await expect(page.getByRole("spinbutton", { name: "CPU 核数 *", exact: true })).toHaveValue("4");
  await expect(
    page.getByRole("radio", { name: "年付 CNY 200.00", exact: true }),
  ).toBeChecked();
  await site.configureNumber("CPU 核数", "6");
  await expect(
    page.getByRole("button", { name: "保存配置", exact: true }),
  ).toBeEnabled();
  await page.getByRole("button", { name: "保存配置", exact: true }).click();
  await expect(page).toHaveURL(/\/cart.html/);
  expect(mutations).toHaveLength(1);
  expect(mutations[0]).toMatchObject({
    expectedCartVersion: 7,
    expectedItemVersion: 4,
    quantity: 3,
    purchase: { billingTermToken: "YEAR", values: [{ integerValue: "6" }] },
  });
});

for (const setting of [
  {
    width: 320,
    lang: "zh-CN",
    next: "去结算",
    title: "结算详情",
    submit: "立即结账",
    apply: "应用",
  },
  {
    width: 390,
    lang: "en-US",
    next: "Proceed to checkout",
    title: "Checkout details",
    submit: "Place order",
    apply: "Apply",
  },
]) {
  test(`TC-10f-${setting.lang} ${setting.width}px结算金额与按钮完整显示，加载期间不闪空购物车`, async ({
    site,
    page,
  }) => {
    await page.setViewportSize({ width: setting.width, height: 844 });
    let release: () => void = () => {};
    const held = new Promise<void>((resolve) => {
      release = resolve;
    });
    const quote = {
      quoteNo: "Q",
      currencyCode: "CNY",
      subtotalMinor: "7900",
      discountMinor: "0",
      taxAmountMinor: "0",
      totalAmountMinor: "7900",
      expiresAt: Date.now() + 60000,
      items: [{ cartItemNo: "ITEM", totalAmountMinor: "7900" }],
    };
    const cart = {
      version: 1,
      currencyCode: "CNY",
      items: [
        {
          cartItemNo: "ITEM",
          publicSlug: "test",
          productNameZhCN: "弹性云主机",
          productNameEnUS: "Elastic cloud server",
          quantity: 1,
          purchaseCycles: 1,
          periodUnitCode: "month",
          periodCount: 1,
          version: 1,
        },
      ],
      latestQuote: quote,
    };
    await page.route("**/__website/session", (r) =>
      r.fulfill({ json: { checked: true, accessToken: "customer-session" } }),
    );
    await page.route("**/account/commerce/**", async (r) => {
      if (r.request().url().endsWith("guest-cart-merge")) await held;
      return r.fulfill({ json: { code: 0, data: { cart } } });
    });
    await page.route("**/account/billing/overview", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            overview: {
              balances: [
                {
                  accountTypeCode: "customer_balance",
                  currencyCode: "CNY",
                  statusCode: "active",
                  availableBalanceMinor: "100",
                },
              ],
            },
          },
        },
      }),
    );
    await page.route("**/account/payment-channels?**", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            channels: [
              {
                channelNo: "BANK",
                displayName:
                  setting.lang === "en-US" ? "Bank payment" : "银行支付",
                modes: ["page"],
              },
            ],
          },
        },
      }),
    );
    await site.open("/cart.html?lang=" + setting.lang);
    await expect(page.locator(".cart-loading")).toBeVisible();
    await expect(page.locator(".cart-empty")).toHaveCount(0);
    release();
    await page.getByRole("button", { name: setting.next, exact: true }).click();
    await expect(
      page.getByRole("heading", { name: setting.title, exact: true }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: setting.submit, exact: true }),
    ).toBeInViewport();
    await expect(
      page.getByRole("textbox", {
        name: setting.lang === "en-US" ? "Promotion code" : "优惠码",
        exact: true,
      }),
    ).toBeVisible();
    const apply = await page
      .getByRole("button", { name: setting.apply, exact: true })
      .boundingBox();
    expect(apply!.height).toBeLessThanOrEqual(44);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await expect(
      page.getByRole("radio", {
        name: setting.lang === "en-US" ? /Account balance/ : /账户余额/,
      }),
    ).toBeDisabled();
    await site.screenshot("checkout-reference-mobile-" + setting.lang);
  });
}

test("TC-10g 加购直接复用两次写响应，报价版本推进不重复读取购物车", async ({
  site,
  page,
}) => {
  const calls = { open: 0, add: 0, get: 0, merge: 0, quote: 0 };
  let cart: any = {
    cartNo: "CART",
    version: 1,
    currencyCode: "CNY",
    items: [],
  };
  await page.route("**/__website/session", (r) =>
    r.fulfill({ json: { checked: true, accessToken: "customer-session" } }),
  );
  await page.route("**/x/idc-core/api/v1/**", (r) => {
    const req = r.request(),
      path = new URL(req.url()).pathname;
    let data: any;
    if (path.endsWith("/catalog/products/test"))
      data = { product: { nameZhCN: "快速加购商品", currencies: ["CNY"] } };
    else if (path.endsWith("/purchase-configuration")) data = schema;
    else if (path.endsWith("/evaluate"))
      data = { ...schema, valid: true, values: req.postDataJSON().values };
    else if (path.endsWith("/quote"))
      data = { configurationRevision: revision, currencyCode: "CNY", terms };
    else if (path.endsWith("/commerce/carts")) {
      calls.open++;
      data = { cart };
    } else if (path.endsWith("/commerce/cart-items")) {
      calls.add++;
      expect(req.postDataJSON().expectedCartVersion).toBe(1);
      cart = {
        ...cart,
        version: 2,
        items: [
          {
            cartItemNo: "ITEM",
            configurationReference: "CFG",
            publicSlug: "test",
            productNameZhCN: "快速加购商品",
            quantity: 1,
            purchaseCycles: 1,
            version: 1,
          },
        ],
      };
      data = { cart };
    } else if (path.endsWith("/guest-cart-merge")) {
      calls.merge++;
      data = { merged: false, cart };
    } else if (path.endsWith("/commerce/cart")) {
      calls.get++;
      data = { cart };
    } else if (path.endsWith("/commerce/quotes")) {
      calls.quote++;
      expect(req.postDataJSON().expectedCartVersion).toBe(2);
      const quote = {
        quoteNo: "Q",
        cartVersion: 3,
        currencyCode: "CNY",
        totalAmountMinor: "2100",
        subtotalMinor: "2100",
        discountMinor: "0",
        taxAmountMinor: "0",
        expiresAt: Date.now() + 60000,
        items: [
          {
            cartItemNo: "ITEM",
            quoteItemNo: "QI",
            totalAmountMinor: "2100",
            configurationRequired: true,
            configurationFrozen: true,
          },
        ],
      };
      cart = { ...cart, version: 3, latestQuote: quote };
      data = { quote };
    } else if (path.endsWith("/marketing/coupons")) data = { items: [] };
    else if (path.endsWith("/commerce/cart-items/ITEM")) {
      expect(req.postDataJSON()).toMatchObject({
        expectedCartVersion: 3,
        expectedItemVersion: 1,
        quantity: 2,
      });
      return r.fulfill({ status: 503, json: { code: 503 } });
    } else return r.fallback();
    return r.fulfill({ json: { code: 0, data } });
  });
  await site.open("/configure.html?product=test");
  await expect(page.locator(".purchase-add")).toBeEnabled();
  await site.configureNumber("CPU 核数", "4");
  await expect(page.locator(".purchase-add")).toBeEnabled();
  await site.addToCart();
  await expect(page).toHaveURL(/\/cart.html/);
  await expect(page.locator(".cart-total")).toContainText("CNY 21.00");
  await expect(page.locator(".cart-configuration")).toContainText("CPU 核数");
  expect(calls).toEqual({ open: 1, add: 1, get: 0, merge: 1, quote: 1 });
  await site.cartAction("增加数量");
  await site.cartAction("更新");
  await expect(page.locator(".cart-status")).toContainText("操作未完成");
  await site.screenshot("cart-request-count-version");
});

for (const language of ["zh-CN", "en-US"]) {
  test(`TC-10h-${language} 优惠券和付款资料缓慢不阻塞结算，付款读取失败可重试`, async ({
    site,
    page,
  }) => {
    const zh = language === "zh-CN";
    let releaseAssets!: () => void, releasePayments!: () => void;
    const assetGate = new Promise<void>((r) => (releaseAssets = r)),
      paymentGate = new Promise<void>((r) => (releasePayments = r));
    const calls = {
      cartGet: 0,
      quote: 0,
      coupons: 0,
      balance: 0,
      channels: 0,
      order: 0,
    };
    const quote = {
      quoteNo: "Q",
      cartVersion: 2,
      currencyCode: "CNY",
      totalAmountMinor: "2100",
      subtotalMinor: "2100",
      discountMinor: "0",
      taxAmountMinor: "0",
      expiresAt: Date.now() + 60000,
      items: [
        {
          cartItemNo: "ITEM",
          quoteItemNo: "QI",
          totalAmountMinor: "2100",
          configurationRequired: true,
          configurationFrozen: true,
        },
      ],
    };
    const cart = {
      cartNo: "CART",
      version: 2,
      currencyCode: "CNY",
      latestQuote: quote,
      items: [
        {
          cartItemNo: "ITEM",
          publicSlug: "test",
          productNameZhCN: "快速结算商品",
          productNameEnUS: "Fast checkout",
          quantity: 1,
          purchaseCycles: 1,
          chargeTypeCode: "recurring", periodUnitCode:"month", periodCount:1,
          version: 1,
        },
      ],
    };
    await page.route("**/__website/session", (r) =>
      r.fulfill({ json: { checked: true, accessToken: "customer-session" } }),
    );
    await page.route("**/x/idc-core/api/v1/**", async (r) => {
      const path = new URL(r.request().url()).pathname;
      let data: any;
      if (path.endsWith("/guest-cart-merge")) data = { merged: false, cart };
      else if (path.endsWith("/commerce/cart")) {
        calls.cartGet++;
        data = { cart };
      } else if (path.endsWith("/commerce/quotes")) {
        calls.quote++;
        data = { quote };
      } else if (path.endsWith("/marketing/coupons")) {
        calls.coupons++;
        await assetGate;
        data = { items: [] };
      } else if (path.endsWith("/billing/overview")) {
        calls.balance++;
        if (calls.balance === 1) {
          await paymentGate;
          return r.fulfill({ status: 503, json: { code: 503 } });
        }
        data = {
          overview: {
            balances: [
              {
                accountTypeCode: "customer_balance",
                currencyCode: "CNY",
                statusCode: "active",
                availableBalanceMinor: "0",
              },
            ],
          },
        };
      } else if (path.endsWith("/commerce/orders")) {
        calls.order++;
        expect(r.request().postDataJSON().quoteNo).toBe("Q");
        data = { checkout: { invoiceNo: "INV-FAST" } };
      } else return r.fallback();
      try {
        await r.fulfill({ json: { code: 0, data } });
      } catch {
        /* The page may leave for its invoice while optional reads finish. */
      }
    });
    await page.route(
      "**/x/idc-payment/api/v1/account/payment-channels?**",
      async (r) => {
        calls.channels++;
        if (calls.channels === 1) {
          await paymentGate;
          return r.fulfill({ status: 503, json: { code: 503 } });
        }
        await r.fulfill({
          json: {
            code: 0,
            data: {
              channels: [
                {
                  channelNo: "ALI",
                  displayName: zh ? "支付宝" : "Alipay",
                  modes: ["page"],
                },
              ],
            },
          },
        });
      },
    );
    await page.route("http://127.0.0.1:6032/account/billing/invoices/**", (r) =>
      r.fulfill({ body: "Invoice payment" }),
    );
    try {
      await site.open("/cart.html?lang=" + language);
      await expect(page.locator(".cart-total")).toContainText("CNY 21.00");
      await expect(
        page.getByRole("button", {
          name: zh ? "去结算" : "Proceed to checkout",
          exact: true,
        }),
      ).toBeEnabled();
      expect(calls.coupons).toBe(1);
      await site.cartAction(zh ? "去结算" : "Proceed to checkout");
      await expect(
        page.getByRole("heading", {
          name: zh ? "结算详情" : "Checkout details",
          exact: true,
        }),
      ).toBeVisible();
      await expect(
        page.getByRole("button", {
          name: zh ? "立即结账" : "Place order",
          exact: true,
        }),
      ).toBeEnabled();
      await expect.poll(() => [calls.balance, calls.channels]).toEqual([1, 1]);
      expect(calls.cartGet).toBe(0);
      expect(calls.quote).toBe(0);
      await site.mobile();
      await site.showPaymentChoices();
      await site.screenshot("checkout-optional-reads-pending-" + language);
      releasePayments();
      await expect(page.locator(".cart-payment-options")).toContainText(
        zh ? "支付方式加载失败" : "Could not load payment methods",
      );
      await expect(
        page.getByRole("button", {
          name: zh ? "立即结账" : "Place order",
          exact: true,
        }),
      ).toBeEnabled();
      await site.showPaymentChoices();
      await site.screenshot("checkout-optional-reads-failed-" + language);
      await site.retryPaymentChoices(zh ? "重试" : "Retry");
      await expect(
        page.getByRole("radio", {
          name: zh ? "支付宝" : "Alipay",
          exact: true,
        }),
      ).toBeVisible();
      await site.choosePayment(zh ? "支付宝" : "Alipay");
      await site.showPaymentChoices();
      await site.screenshot("checkout-optional-reads-ready-" + language);
      await site.cartAction(zh ? "立即结账" : "Place order");
      await expect(page).toHaveURL(
        "http://127.0.0.1:6032/account/billing/invoices/INV-FAST?pay=1&paymentChannel=ALI",
      );
      expect(calls.order).toBe(1);
    } finally {
      releaseAssets();
      releasePayments();
    }
  });
}

test("TC-10i 旧报价补配后保留一次权威购物车重读", async ({ site, page }) => {
  const calls = { get: 0, quote: 0, freeze: 0 };
  const quote: any = {
    quoteNo: "Q",
    cartVersion: 2,
    currencyCode: "CNY",
    totalAmountMinor: "2100",
    subtotalMinor: "2100",
    discountMinor: "0",
    taxAmountMinor: "0",
    expiresAt: Date.now() + 60000,
    items: [
      {
        cartItemNo: "ITEM",
        quoteItemNo: "QI",
        totalAmountMinor: "2100",
        configurationRequired: true,
        configurationFrozen: false,
      },
    ],
  };
  const cart: any = {
    cartNo: "CART",
    version: 1,
    currencyCode: "CNY",
    items: [
      {
        cartItemNo: "ITEM",
        configurationReference: "CFG",
        publicSlug: "test",
        productNameZhCN: "待补配商品",
        quantity: 1,
        purchaseCycles: 1,
        version: 1,
      },
    ],
  };
  await page.addInitScript(
    (draft) =>
      sessionStorage.setItem(
        "website-qycn-purchase-drafts",
        JSON.stringify({ CFG: draft }),
      ),
    {
      publicSlug: "test",
      configurationRevision: revision,
      values: [{ fieldToken: "cpu", type: "integer", integerValue: "4" }],
    },
  );
  await page.route("**/__website/session", (r) =>
    r.fulfill({ json: { checked: true, accessToken: "customer-session" } }),
  );
  await page.route("**/x/idc-core/api/v1/**", (r) => {
    const path = new URL(r.request().url()).pathname;
    let data: any;
    if (path.endsWith("/guest-cart-merge")) data = { merged: true, cart };
    else if (path.endsWith("/commerce/quotes")) {
      calls.quote++;
      data = { quote };
    } else if (
      path.endsWith("/commerce/quote-items/QI/purchase-configuration")
    ) {
      calls.freeze++;
      expect(r.request().postDataJSON().values[0].integerValue).toBe("4");
      quote.items[0].configurationFrozen = true;
      cart.version = 2;
      cart.latestQuote = quote;
      data = {
        snapshotNo: "S",
        configurationFrozen: true,
        configurationRevision: revision,
      };
    } else if (path.endsWith("/commerce/cart")) {
      calls.get++;
      data = { cart };
    } else if (path.endsWith("/marketing/coupons")) data = { items: [] };
    else return r.fallback();
    return r.fulfill({ json: { code: 0, data } });
  });
  await site.open("/cart.html");
  await expect(page.locator(".cart-total")).toContainText("CNY 21.00");
  await expect(
    page.getByRole("button", { name: "去结算", exact: true }),
  ).toBeEnabled();
  expect(calls).toEqual({ get: 1, quote: 1, freeze: 1 });
});

for (const compatibility of [false, true]) {
  test(`TC-10j-${compatibility ? "legacy" : "current"} 明确空车无需重读，旧owner缺字段保留回退`, async ({site,page}) => {
    const calls = { merge:0, get:0, quote:0 };
    await page.route("**/__website/session", route => route.fulfill({json:{checked:true,accessToken:"customer-session"}}));
    await page.route("**/x/idc-core/api/v1/account/commerce/**", route => {
      const path = new URL(route.request().url()).pathname;
      let data:any;
      if (path.endsWith("/guest-cart-merge")) {
        calls.merge++;
        data = compatibility ? {merged:false} : {merged:false,cart:null};
      } else if (path.endsWith("/cart")) { calls.get++; data={cart:null}; }
      else if (path.endsWith("/quotes")) { calls.quote++; data={}; }
      else return route.fallback();
      return route.fulfill({json:{code:0,data}});
    });
    await site.open("/cart.html?lang=en-US");
    await expect(page.getByRole("heading",{name:"Your cart is empty",exact:true})).toBeVisible();
    await expect(page.locator(".cart-loading")).toHaveCount(0);
    expect(calls).toEqual({merge:1,get:compatibility?1:0,quote:0});
    await site.screenshot("cart-empty-explicit-null-"+(compatibility?"legacy":"current"));
  });
}

for (const language of ["zh-CN", "en-US"]) {
  for (const scenario of ["repair", "frozen", "missing", "retry"]) {
    test(`TC-10k-${scenario}-${language} 复用有效报价时按本页草稿补冻结且不重复核价`, async ({ site, page }) => {
      const zh = language === "zh-CN";
      const calls = { merge: 0, get: 0, quote: 0, freeze: 0, order: 0 };
      const values = [{ fieldToken: "cpu", type: "integer", integerValue: "4" }];
      const quote: any = {
        quoteNo: "EXISTING",
        cartVersion: 2,
        currencyCode: "CNY",
        totalAmountMinor: "2700",
        subtotalMinor: "2700",
        discountMinor: "0",
        taxAmountMinor: "0",
        expiresAt: Date.now() + 60000,
        items: [{
          cartItemNo: "ITEM", quoteItemNo: "EXISTING-QI",
          totalAmountMinor: "2700", configurationRequired: true,
          configurationFrozen: scenario === "frozen",
        }],
      };
      const cart: any = {
        cartNo: "CART", version: 2, currencyCode: "CNY", latestQuote: quote,
        items: [{
          cartItemNo: "ITEM", configurationReference: "CFG", publicSlug: "test",
          productNameZhCN: "已选配商品", productNameEnUS: "Configured product",
          quantity: 1, purchaseCycles: 1, version: 1,
          periodUnitCode: "month", periodCount: 1,
        }],
      };
      if (scenario !== "missing") {
        await page.addInitScript((draft) => sessionStorage.setItem(
          "website-qycn-purchase-drafts", JSON.stringify({ CFG: draft }),
        ), { publicSlug: "test", configurationRevision: revision, values });
      }
      await page.route("**/__website/session", route => route.fulfill({
        json: { checked: true, accessToken: "customer-session" },
      }));
      await page.route("**/x/idc-core/api/v1/**", async route => {
        const request = route.request(), path = new URL(request.url()).pathname;
        let data: any;
        if (path.endsWith("/guest-cart-merge")) {
          calls.merge++; data = { merged: false, cart };
        } else if (path.endsWith("/commerce/quotes")) {
          calls.quote++; data = { quote };
        } else if (path.endsWith("/quote-items/EXISTING-QI/purchase-configuration")) {
          calls.freeze++;
          expect(request.postDataJSON()).toEqual({
            publicSlug: "test", configurationRevision: revision, values,
          });
          if (scenario === "retry" && calls.freeze === 1) {
            return route.fulfill({ status: 503, json: { code: 503 } });
          }
          quote.items[0].configurationFrozen = true;
          data = { snapshotNo: "SNAPSHOT", configurationFrozen: true, configurationRevision: revision };
        } else if (path.endsWith("/commerce/cart")) {
          calls.get++; data = { cart };
        } else if (path.endsWith("/commerce/orders")) {
          calls.order++; data = {};
        } else if (path.endsWith("/marketing/coupons")) data = { items: [] };
        else return route.fallback();
        await route.fulfill({ json: { code: 0, data } });
      });
      await site.open("/cart.html?lang=" + language);
      await expect(page.locator(".cart-total")).toContainText("CNY 27.00");
      const proceed = page.getByRole("button", {
        name: zh ? "去结算" : "Proceed to checkout", exact: true,
      });
      if (scenario === "retry") {
        await expect(proceed).toBeDisabled();
        await expect(page.locator(".cart-product-issue")).toHaveText(
          zh ? "补全配置" : "Complete configuration",
        );
        await expect(page.locator(".cart-status").getByRole("button", {
          name: zh ? "重试" : "Retry", exact: true,
        })).toBeVisible();
        expect(calls).toEqual({ merge: 1, get: 0, quote: 0, freeze: 1, order: 0 });
        await site.screenshot("existing-quote-freeze-failed-" + language);
        await site.cartAction(zh ? "重试" : "Retry");
      }
      if (scenario === "missing") {
        await expect(proceed).toBeDisabled();
        await expect(page.locator(".cart-product-issue")).toHaveText(
          zh ? "补全配置" : "Complete configuration",
        );
      } else {
        await expect(proceed).toBeEnabled();
        await expect(page.locator(".cart-product-issue")).toHaveCount(0);
      }
      expect(calls).toEqual({
        merge: scenario === "retry" ? 2 : 1,
        get: ["repair", "retry"].includes(scenario) ? 1 : 0,
        quote: 0,
        freeze: scenario === "retry" ? 2 : scenario === "repair" ? 1 : 0,
        order: 0,
      });
      await site.mobile();
      await site.screenshot("existing-quote-" + scenario + "-" + language);
      if (scenario !== "missing") {
        await site.cartAction(zh ? "去结算" : "Proceed to checkout");
        await expect(page.getByRole("heading", {
          name: zh ? "结算详情" : "Checkout details", exact: true,
        })).toBeVisible();
        await expect(page.locator(".cart-total")).toContainText("CNY 27.00");
      }
      expect(calls.order).toBe(0);
      expect(calls.quote).toBe(0);
    });
  }
}

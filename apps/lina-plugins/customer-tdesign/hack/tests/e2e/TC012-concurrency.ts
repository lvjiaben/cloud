import { test, expect, menus, now } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";

test.describe("TC-12 并发与失败恢复", () => {
  test("TC-12a 续费报价与确认期间锁定周期，失败重试不重复提交", async ({
    consolePage: page,
  }) => {
    const view = new CustomerPage(page);
    await page.route("**/api/v1/account/billing/renewal-entitlements*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            entitlements: [
              {
                entitlementNo: "ENT-race",
                entitlementVersion: 1,
                productNameZhCN: "续费服务",
                entitlementExpiresAt: now + 86400000,
                entitlementStatusCode: "active",
                autoRenewEnabled: false,
                canConfigureAutomaticRenewal: true,
                currencyCode: "CNY",
              },
            ],
            nextCursor: "",
          },
        },
      }),
    );
    let finish: () => void = () => {},
      quoteCalls = 0;
    const gate = new Promise<void>((r) => {
      finish = r;
    });
    await page.route("**/entitlements/ENT-race/renewal-quotes", async (r) => {
      quoteCalls++;
      const cycles = r.request().postDataJSON().renewalCycles;
      if (quoteCalls === 1) await gate;
      await r.fulfill({
        json: {
          code: 0,
          data: {
            quote: {
              quoteNo: `Q-${cycles}`,
              currencyCode: "CNY",
              totalAmountMinor: String(cycles * 1000),
              expiresAt: Date.now() + 60000,
              projectedExpiresAt: now + cycles * 86400000,
            },
          },
        },
      });
    });
    const confirmations: unknown[] = [];
    let fail = true;
    await page.route("**/renewal-quotes/Q-2/confirmations", (r) => {
      confirmations.push(r.request().postDataJSON());
      return r.fulfill(
        fail
          ? {
              status: 503,
              json: { code: 503, message: "暂时无法续费，请重试" },
            }
          : {
              json: {
                code: 0,
                data: { operation: { invoiceNo: "INV-TEST-001" } },
              },
            },
      );
    });
    await view.open("/account/billing/renewals");
    await view.button("续费").click();
    const dialog = page.getByRole("dialog", { name: "续费服务" });
    const cycles = dialog.locator("input").first();
    await view.button("查看续费金额").dblclick();
    await expect.poll(() => quoteCalls).toBe(1);
    await expect(cycles).toBeDisabled();
    await dialog.getByRole("button", { name: "关闭弹窗" }).click();
    await expect(dialog).toBeVisible();
    finish();
    await expect(dialog).toContainText("CNY 10.00");
    await expect(cycles).toBeEnabled();
    await cycles.fill("2");
    await cycles.press("Tab");
    await expect(dialog).not.toContainText("CNY 10.00");
    await view.button("查看续费金额").click();
    await expect(dialog).toContainText("CNY 20.00");
    await view.button("确认续费并付款").dblclick();
    await expect(dialog).toContainText("暂时无法续费，请重试");
    expect(confirmations).toHaveLength(1);
    fail = false;
    await view.button("确认续费并付款").dblclick();
    await expect(page).toHaveURL(/invoices\/INV-TEST-001/);
    expect(confirmations).toHaveLength(2);
    expect(confirmations[1]).toEqual(confirmations[0]);
    await view.screenshot("concurrency-renewal-recovered");
  });
  test("TC-12b 分类切换后旧失败不覆盖新列表，403重试保留登录", async ({
    consolePage: page,
  }) => {
    const view = new CustomerPage(page);
    const dirs = ["slow", "current"].map((directoryNo, i) => ({
      directoryNo,
      nameZhCn: i ? "当前分类" : "慢速分类",
      navigationType: "product",
      sortOrder: 20 + i,
    }));
    await page.route("**/account/product-directories?*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: { items: [...menus, ...dirs], total: menus.length + 2 },
        },
      }),
    );
    let finish: () => void = () => {},
      started = false,
      denied = true;
    const gate = new Promise<void>((r) => {
      finish = r;
    });
    await page.route("**/product-directories/slow/services?*", async (r) => {
      started = true;
      await gate;
      await r.fulfill({
        status: 503,
        json: { code: 503, message: "旧分类加载失败" },
      });
    });
    await page.route("**/product-directories/current/services?*", (r) =>
      r.fulfill(
        denied
          ? { status: 403, json: { code: 403 } }
          : {
              json: {
                code: 0,
                data: {
                  items: [
                    {
                      entitlementNo: "CURRENT",
                      productNameZhCn: "当前分类服务",
                      statusCode: "active",
                    },
                  ],
                  total: 1,
                  providers: [{ pluginId: "idc-resource", total: 1 }],
                },
              },
            },
      ),
    );
    await view.open("/account/product-directories/slow");
    await expect.poll(() => started).toBe(true);
    await page
      .getByRole("navigation")
      .getByRole("link", { name: "当前分类", exact: true })
      .click();
    await expect(view.main).toContainText("当前账号无法办理这项业务");
    expect(
      await page.evaluate(
        () => !!sessionStorage.getItem("customer-tdesign.session"),
      ),
    ).toBe(true);
    denied = false;
    await view.button("重新加载").click();
    await expect(view.serviceBody).toContainText("当前分类服务");
    const oldResponse = page.waitForResponse((r) =>
      r.url().includes("/slow/services"),
    );
    finish();
    await oldResponse;
    await expect(view.serviceBody).toContainText("当前分类服务");
    await expect(view.main).not.toContainText("旧分类加载失败");
    await view.screenshot("concurrency-category-latest");
  });
  test("TC-12c 充值弹窗取消重开后旧渠道响应不污染新选择", async ({
    consolePage: page,
  }) => {
    const view = new CustomerPage(page);
    let calls = 0,
      release: () => void = () => {};
    const gate = new Promise<void>((r) => {
      release = r;
    });
    await page.route("**/payment-channels*", async (r) => {
      const first = ++calls === 1;
      if (first) await gate;
      const currency = first ? "JPY" : "USD";
      await r.fulfill({
        json: {
          code: 0,
          data: {
            channels: [
              {
                channelNo: currency,
                displayName: currency + "支付",
                currencies: [currency],
                modes: ["page"],
              },
            ],
          },
        },
      });
    });
    await view.open("/account/billing?recharge=1");
    const dialog = page.getByRole("dialog", { name: "账户充值" });
    await expect.poll(() => calls).toBe(1);
    await dialog.getByRole("button", { name: "关闭弹窗" }).click();
    await expect(dialog).toHaveCount(0);
    await view.button("充值").click();
    await expect(dialog.locator(".t-select input").first()).toHaveValue("USD");
    const oldResponse = page.waitForResponse((r) =>
      r.url().includes("/payment-channels"),
    );
    release();
    await oldResponse;
    await expect(dialog.locator(".t-select input").first()).toHaveValue("USD");
    await dialog.locator(".t-select").nth(1).click();
    await expect(page.getByText("USD支付", { exact: true })).toBeVisible();
    await expect(page.getByText("JPY支付", { exact: true })).toHaveCount(0);
    await view.screenshot("concurrency-reopened-channel");
  });
  test("TC-12d 搜索加载锁定操作，离开后旧响应不得覆盖重新进入的状态", async ({
    consolePage: page,
  }) => {
    const view = new CustomerPage(page);
    let calls = 0,
      release: () => void = () => {},
      oldFinished = false;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route(
      "**/product-directories/cloud/services?*",
      async (route) => {
        const isProvider =
          new URL(route.request().url()).searchParams.get("providerId") ===
          "idc-resource";
        const call = isProvider ? ++calls : 0;
        if (call === 2) await gate;
        try {
          await route.fulfill({
            json: {
              code: 0,
              data: {
                items: [
                  {
                    entitlementNo: "transition",
                    productNameZhCn: "刷新服务",
                    entitlementOwnerId: "idc-resource",
                    statusCode: call >= 3 ? "active" : "pending_activation",
                    requiresManualProcessing: call < 3,
                    orderNo: "ORD-TEST-001",
                    ...(call >= 3 ? { managementPreparation: "ready" } : {}),
                  },
                ],
                total: 1,
                providers: [{ pluginId: "idc-resource", total: 1 }],
              },
            },
          });
        } catch (error) {
          if (call !== 2) throw error;
        } finally {
          if (call === 2) oldFinished = true;
        }
      },
    );
    await view.open("/account/product-directories");
    await expect(view.serviceBody).toContainText("待人工处理");
    await view.reloadServices();
    await expect.poll(() => calls).toBe(2);
    await expect(view.serviceSearch).toBeDisabled();
    await view.navigateMenu("总览");
    await view.navigateMenu("云服务器");
    await expect(view.serviceBody).toContainText("已激活");
    release();
    await expect.poll(() => oldFinished).toBe(true);
    await expect(view.serviceBody).not.toContainText("待开通");
    await expect(
      view.serviceBody.getByRole("link", { name: "管理", exact: true }),
    ).toHaveAttribute(
      "href",
      "/account/native-servers/by-entitlement/transition",
    );
    await view.screenshot("concurrency-service-latest");
  });
  test("TC-12e 刷新身份失败只尝试一次，重新登录恢复原分类", async ({
    apiPage: page,
  }) => {
    await page.addInitScript(
      () =>
        window === window.top &&
        sessionStorage.setItem(
          "customer-tdesign.session",
          JSON.stringify({
            accessToken: "expired",
            refreshToken: "expired-refresh",
          }),
        ),
    );
    const view = new CustomerPage(page);
    let services = 0,
      refreshes = 0;
    await page.route("**/auth/refresh", (r) => {
      refreshes++;
      return r.fulfill({ status: 401, json: { code: 401 } });
    });
    await page.route(
      "**/x/idc-notify/api/v1/public/account/password-logins",
      (r) =>
        r.fulfill({ json: { code: 0, data: { accessToken: "new-session" } } }),
    );
    await page.route("**/product-directories/private/services?*", (r) => {
      services++;
      return r.fulfill(
        r.request().headers().authorization === "Bearer new-session"
          ? {
              json: {
                code: 0,
                data: {
                  items: [
                    {
                      entitlementNo: "CURRENT",
                      productNameZhCn: "恢复后的服务",
                      statusCode: "active",
                    },
                  ],
                  total: 1,
                  providers: [{ pluginId: "idc-resource", total: 1 }],
                },
              },
            }
          : { status: 401, json: { code: 401 } },
      );
    });
    await page.route("**/account/product-directories?*", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            items: [
              ...menus,
              {
                directoryNo: "private",
                navigationType: "product",
                nameZhCn: "专用服务",
                sortOrder: 3,
              },
            ],
            total: menus.length + 1,
          },
        },
      }),
    );
    await view.open("/account/product-directories/private");
    await expect(page).toHaveURL(/auth\/login\?redirect=/);
    expect(new URL(page.url()).searchParams.get("redirect")).toBe(
      "/account/product-directories/private",
    );
    expect(refreshes).toBe(1);
    expect(services).toBe(1);
    expect(
      await page.evaluate(() =>
        sessionStorage.getItem("customer-tdesign.session"),
      ),
    ).toBeNull();
    await page
      .getByRole("textbox", { name: "账号", exact: true })
      .fill("test@example.invalid");
    await page.locator('input[aria-label="密码"]').fill("example-password");
    await view.button("登录用户中心").click();
    await expect(view.serviceBody).toContainText("恢复后的服务");
    await expect(page).toHaveURL(/account\/product-directories\/private$/);
    expect(refreshes).toBe(1);
    expect(services).toBe(3);
    await view.screenshot("concurrency-login-recovered");
  });
  test("TC-12f 旧刷新挂起时退出再登录，新账户独立刷新且不被旧响应清空", async ({
    apiPage: page,
  }) => {
    await page.addInitScript(
      () =>
        window === window.top &&
        sessionStorage.setItem(
          "customer-tdesign.session",
          JSON.stringify({ accessToken: "old", refreshToken: "refresh-old" }),
        ),
    );
    const view = new CustomerPage(page);
    let refreshes = 0,
      release: () => void = () => {};
    const gate = new Promise<void>((r) => {
      release = r;
    });
    await page.route("**/auth/refresh", async (r) => {
      refreshes++;
      if (r.request().postDataJSON().refreshToken === "refresh-old") {
        await gate;
        await r.fulfill({
          json: { code: 0, data: { accessToken: "old-renewed" } },
        });
      } else
        await r.fulfill({
          json: { code: 0, data: { accessToken: "new-renewed" } },
        });
    });
    await page.route(
      "**/x/idc-notify/api/v1/public/account/password-logins",
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: { accessToken: "new", refreshToken: "refresh-new" },
          },
        }),
    );
    await page.route("**/product-directories/cloud/services?*", (r) =>
      r.fulfill(
        r.request().headers().authorization === "Bearer new-renewed"
          ? {
              json: {
                code: 0,
                data: {
                  items: [
                    {
                      entitlementNo: "NEW",
                      productNameZhCn: "新会话服务",
                      statusCode: "active",
                    },
                  ],
                  total: 1,
                  providers: [{ pluginId: "idc-resource", total: 1 }],
                },
              },
            }
          : { status: 401, json: { code: 401 } },
      ),
    );
    await view.open("/account/product-directories");
    await expect.poll(() => refreshes).toBe(1);
    await view.button("账户菜单").click();
    await page.getByText("退出登录", { exact: true }).click();
    await expect(page).toHaveURL(/auth\/login/);
    await page
      .getByRole("textbox", { name: "账号", exact: true })
      .fill("new@example.invalid");
    await page.locator('input[aria-label="密码"]').fill("example-password");
    await view.button("登录用户中心").click();
    await expect(view.button("账户菜单")).toBeVisible();
    await page
      .getByRole("navigation")
      .getByRole("link", { name: "云服务器", exact: true })
      .click();
    await expect(view.serviceBody).toContainText("新会话服务");
    expect(refreshes).toBe(2);
    const oldResponse = page.waitForResponse((r) =>
      r.url().includes("/auth/refresh"),
    );
    release();
    await oldResponse;
    await expect(view.serviceBody).toContainText("新会话服务");
    expect(
      await page.evaluate(
        () =>
          JSON.parse(
            sessionStorage.getItem("customer-tdesign.session") || "null",
          )?.accessToken,
      ),
    ).toBe("new-renewed");
    await view.screenshot("concurrency-account-isolation");
  });
});

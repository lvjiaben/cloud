import { test, expect } from "@host-tests/fixtures/auth";
import { LoginPage } from "@host-tests/pages/LoginPage";
import { NativeServerPage } from "../pages/NativeServerPage";

const origin = process.env.CUSTOMER_E2E_BASE_URL || "http://127.0.0.1:6032";
const serverNo = "SVR-11111111111111111111111111111111";
const entitlementNo = "ent_11111111111111111111111111111111";
const native = `**/x/idc-resource/api/v1/account/native-servers/${serverNo}`;
function detail(overrides: Record<string, unknown> = {}) {
  return {
    serverNo,
    entitlementNo,
    productNameZhCn: "标准云服务器",
    productNameEnUs: "Standard cloud server",
    entitlementStatus: "active",
    runtimeState: "active",
    orderNo: "ORD_native_test",
    periodUnit: "month",
    periodCount: 1,
    startedAt: Date.now() - 10000,
    expiresAt: Date.now() + 86400000,
    observedAt: Date.now(),
    availableActions: ["stop", "reboot"],
    operations: [],
    addresses: ["203.0.113.9"],
    addressesConfirmed: true,
    credentialAvailable: false,
    ...overrides,
  };
}

test.describe("TC-5 原生服务器用户管理", () => {
  test.setTimeout(45000);
  test.beforeEach(async ({ page }) => {
    await page.route("**/api/v1/auth/login", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            accessToken: "fixture-native-session",
            refreshToken: "fixture-native-refresh",
            tenants: [],
          },
        },
      }),
    );
    await page.route("**/api/v1/user/info", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            id: 703,
            username: "native-ui",
            realName: "服务器测试客户",
            roles: ["user"],
            permissions: [],
            menus: [],
            homePath: "/account/billing/invoices",
          },
        },
      }),
    );
    await page.route("**/api/v1/plugins/dynamic", (r) =>
      r.fulfill({ json: { code: 0, data: { list: [] } } }),
    );
    await page.route("**/customer/external-menus", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    await page.route("**/x/idc-core/api/v1/account/product-directories*", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [], total: 0 } } }),
    );
    await page.route("**/account/initial-password/status", (r) =>
      r.fulfill({ json: { code: 0, data: { required: false } } }),
    );
    await page.route("**/account/billing/invoices*", (r) =>
      r.fulfill({ json: { code: 0, data: { invoices: [], nextCursor: "" } } }),
    );
    await page.goto(origin + "/auth/login");
    await new LoginPage(page).login("native-ui", "fixture-only-password");
  });

  test("TC-5o: 开机进入原请求队列，通道暂停有原因和时间，恢复后自动收敛", async ({ page }) => {
    test.setTimeout(90000);
    const model = new NativeServerPage(page, origin, serverNo);
    const waitingUntil = Date.now() + 300000;
    let sent = 0, completed = false;
    const operation = () => ({ operationNo: "nso_start_waiting", action: "start", status: completed ? "succeeded" : "pending", createdAt: Date.now(), updatedAt: Date.now(), ...(completed ? {} : {waitingUntil}) });
    await page.route(native, r => r.fulfill({json: {code: 0, data: detail({ runtimeState: completed ? "running" : "stopped", availableActions: sent && !completed ? [] : completed ? ["stop", "reboot"] : ["start"], operations: sent ? [operation()] : [] })}}));
    await page.route(native + "/operations", async r => {
      const body = r.request().postDataJSON();
      expect(body.action).toBe("start");
      sent++;
      await r.fulfill({json:{code:0,data:{operation:operation()}}});
    });
    await model.open();
    await model.start();
    await expect(page.getByTestId("operation-waiting-reason")).toContainText("此请求尚未发送");
    await expect(page.getByTestId("operation-waiting-reason")).toContainText("自动继续");
    await expect(page.getByRole("button",{name:"开机",exact:true})).toHaveCount(0);
    await new LoginPage(page).switchLanguage("English");
    await expect(page.getByTestId("operation-waiting-reason")).toContainText("has not been sent");
    await model.screenshot("start-waiting-reason");
    completed = true;
    await expect(page.getByTestId("native-server-operations")).toContainText("Completed", { timeout: 35000 });
    await expect(page.getByTestId("operation-waiting-reason")).toHaveCount(0);
    await expect(page.getByRole("button",{name:"Stop",exact:true})).toBeVisible();
    expect(sent).toBe(1);
  });

  test("TC-5n: 全部服务、商品目录与总览直接管理，按拥有方与准备状态显示入口", async ({ page }) => {
    const model = new NativeServerPage(page, origin, serverNo);
    await page.route("**/api/v1/user/info", r => r.fulfill({ json: { code: 0, data: { id: 703, username: "native-ui", realName: "服务器测试客户", roles: ["user"], permissions: [], menus: [], homePath: "/account" } } }));
    let resolveReads = 0;
    const writes: string[] = [];
    page.on("request", request => {
      if (request.url().includes("/account/") && !["GET", "HEAD"].includes(request.method())) writes.push(request.method());
    });
    const base = { productNo: "product-test", productNameZhCn: "标准云服务器", productNameEnUs: "Standard cloud server", pluginId: "connector-fixture", statusCode: "active", requiresManualProcessing: false, orderNo: "ORD_native_test", startedAt: 1792500000000, expiresAt: null };
    const items = [
      { ...base, entitlementNo, entitlementOwnerId: "idc-resource", managementPreparation: "ready" },
      { ...base, entitlementNo: "ent-other-owner", entitlementOwnerId: "certificate-owner", pluginId: "idc-resource" },
      { ...base, entitlementNo: "ent-preparing", entitlementOwnerId: "idc-resource", managementPreparation: "preparing" },
      { ...base, entitlementNo: "ent-pending", entitlementOwnerId: "idc-resource", statusCode: "pending_activation" },
    ];
    await page.route("**/x/idc-core/api/v1/account/product-directories/*/services*", r => r.fulfill({ json: { code: 0, data: { items, total: items.length } } }));
    await page.route(`**/account/native-servers/resolve/${entitlementNo}`, r => {
      resolveReads++;
      return r.fulfill({ json: { code: 0, data: { serverNo } } });
    });
    await page.route(native, r => r.fulfill({ json: { code: 0, data: detail() } }));
    await page.route("**/account/commerce/orders?*", r => r.fulfill({ json: { code: 0, data: { orders: [], nextCursor: "" } } }));
    await page.route("**/account/billing/overview", r => r.fulfill({ json: { code: 0, data: { overview: { balances: [], openReceivables: [], openInvoiceCount: 0, automaticRenewalEnabled: 0, rechargeAvailable: false } } } }));
    await model.openServiceDirectory();
    await expect(model.serviceManagement(entitlementNo)).toBeVisible();
    await expect(model.serviceManagement("ent-other-owner")).toHaveCount(0);
    await expect(model.serviceManagement("ent-preparing")).toHaveCount(0);
    await expect(model.serviceManagement("ent-pending")).toHaveCount(0);
    await expect(page.getByTestId("directory-management-preparation")).toContainText("管理入口准备中");
    expect(resolveReads).toBe(0);
    await model.screenshot("directory-management-entry");
    await model.serviceManagement(entitlementNo).click();
    await expect(page).toHaveURL(new RegExp(`/account/native-servers/${serverNo}$`));
    await expect(page.getByTestId("native-server-detail")).toContainText("203.0.113.9");
    expect(resolveReads).toBe(1);
    await model.openServiceDirectory("pdr_fixture");
    await expect(model.serviceManagement(entitlementNo)).toBeVisible();
    await new LoginPage(page).switchLanguage("English");
    await expect(model.serviceManagement(entitlementNo)).toHaveText("Manage");
    await page.route("**/account/customer/profile", r => r.fulfill({ json: { code: 0, data: { item: { displayName: "服务器测试客户", status: "active" } } } }));
    await model.openOverview();
    await expect(model.serviceManagement(entitlementNo)).toBeVisible();
    await expect(model.serviceManagement("ent-other-owner")).toHaveCount(0);
    await expect(model.serviceManagement("ent-preparing")).toHaveCount(0);
    await page.setViewportSize({ width: 390, height: 844 });
    await model.screenshot("overview-management-entry-mobile");
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth)).toBe(true);
    await model.serviceManagement(entitlementNo).click();
    await expect(page).toHaveURL(new RegExp(`/account/native-servers/${serverNo}$`));
    expect(resolveReads).toBe(2);
    expect(writes).toHaveLength(0);
  });

  test("TC-5m: 本地管理准备不改已付款与已交付，准备完成才进入管理页", async ({
    page,
  }) => {
    test.setTimeout(90000);
    let managementPreparation = "preparing",
      resolveReads = 0;
    const writes: string[] = [];
    page.on("request", (r) => {
      if (
        r.url().includes("/account/commerce/") &&
        !["GET", "HEAD"].includes(r.method())
      )
        writes.push(r.method());
    });
    await page.route("**/account/commerce/orders/ORD_native_test", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            order: {
              summary: {
                orderNo: "ORD_native_test",
                currencyCode: "CNY",
                statusCode: "paid",
                totalAmountMinor: "1000",
                createdAt: 1792500000000,
                canCancel: false,
                canPreparePayment: false,
              },
              items: [],
              deliveries: [
                {
                  productNameZhCN: "标准云服务器",
                  productNameEnUS: "Standard cloud server",
                  quantity: 1,
                  statusCode: "completed",
                  ongoingService: true,
                  updatedAt: 1792500000000,
                },
              ],
              reviews: [],
              serviceEntitlements: [
                {
                  managementPreparation,
                  entitlementNo,
                  entitlementOwnerId: "idc-resource",
                  orderItemNo: "item-native-test",
                  productNameZhCN: "标准云服务器",
                  productNameEnUS: "Standard cloud server",
                  statusCode: "active",
                  startedAt: 1792500000000,
                },
              ],
            },
          },
        },
      }),
    );
    await page.route(
      `**/account/native-servers/resolve/${entitlementNo}`,
      (r) => {
        resolveReads++;
        return r.fulfill({ json: { code: 0, data: { serverNo } } });
      },
    );
    await page.route(native, (r) =>
      r.fulfill({ json: { code: 0, data: detail() } }),
    );
    await page.goto(origin + "/account/orders/ORD_native_test");
    const services = page.getByTestId("commerce-service-entitlements");
    await expect(page.getByTestId("commerce-deliveries")).toContainText(
      "已完成",
    );
    await expect(services).toContainText("服务中");
    await expect(
      services.getByTestId("service-management-preparation"),
    ).toContainText("管理入口准备中，请稍后刷新。");
    await expect(
      services.getByRole("button", { name: "管理", exact: true }),
    ).toHaveCount(0);
    expect(resolveReads).toBe(0);
    managementPreparation = "needs_attention";
    await page.reload();
    await expect(
      services.getByTestId("service-management-preparation"),
    ).toContainText("管理入口需要管理员处理，请联系支持。");
    await expect(services).toContainText("服务中");
    await expect(page.getByTestId("commerce-deliveries")).toContainText(
      "已完成",
    );
    await page.setViewportSize({ width: 390, height: 844 });
    await new NativeServerPage(page, origin, serverNo).screenshot(
      "management-attention-mobile",
    );
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await new LoginPage(page).switchLanguage("English");
    await expect(
      services.getByTestId("service-management-preparation"),
    ).toContainText(
      "Management access needs administrator attention. Contact support.",
    );
    await expect(
      services.getByRole("button", { name: "Manage", exact: true }),
    ).toHaveCount(0);
    managementPreparation = "ready";
    await page.reload();
    await expect(
      services.getByTestId("service-management-preparation"),
    ).toHaveCount(0);
    await services.getByRole("button", { name: "Manage", exact: true }).click();
    await expect(page).toHaveURL(
      new RegExp(`/account/native-servers/${serverNo}$`),
    );
    await expect(page.getByTestId("native-server-detail")).toContainText(
      "Standard cloud server",
    );
    expect(resolveReads).toBe(1);
    expect(writes).toHaveLength(0);
  });

  test("TC-5a: 普通重启确认、丢失响应复用原请求、异步任务收敛", async ({
    page,
  }) => {
    let requests: Array<Record<string, unknown>> = [];
    let reads = 0;
    await page.route(native, (r) => {
      reads += 1;
      const pending = requests.length > 1 && reads < 4;
      const operations =
        requests.length > 1
          ? [
              {
                operationNo: "nso_test_1",
                action: "reboot",
                status: pending ? "running" : "succeeded",
                createdAt: Date.now(),
                updatedAt: Date.now(),
              },
            ]
          : [];
      return r.fulfill({
        json: {
          code: 0,
          data: detail({
            operations,
            availableActions: pending ? [] : ["stop", "reboot"],
          }),
        },
      });
    });
    await page.route(native + "/operations", (r) => {
      requests.push(r.request().postDataJSON());
      if (requests.length === 1)
        return r.fulfill({
          status: 503,
          json: { code: 50, message: "Unavailable" },
        });
      return r.fulfill({
        json: {
          code: 0,
          data: {
            operation: {
              operationNo: "nso_test_1",
              action: "reboot",
              status: "pending",
              createdAt: Date.now(),
              updatedAt: Date.now(),
            },
          },
        },
      });
    });
    const view = new NativeServerPage(page, origin, serverNo);
    await view.open();
    await view.reboot();
    await expect(
      page.getByText(
        "关机或重启会中断服务器上的业务。请确认已经保存工作并允许本次操作。",
        { exact: true },
      ),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "确认执行", exact: true }),
    ).toBeDisabled();
    expect(requests).toHaveLength(0);
    await view.acceptImpact();
    await view.confirm();
    await expect(
      page.getByText(
        "操作请求未能确认，请使用同一请求重试；不要重复创建操作。",
        { exact: true },
      ),
    ).toBeVisible();
    await page.getByRole("button", { name: "重试原请求", exact: true }).click();
    await expect(page.getByTestId("native-server-operations")).toContainText(
      "执行中",
    );
    expect(requests).toHaveLength(2);
    expect(requests[0]).toEqual(requests[1]);
    expect(requests[1].confirmed).toBe(true);
    await expect(
      page.getByRole("button", { name: "重启", exact: true }),
    ).toHaveCount(0);
    await expect(page.getByTestId("native-server-operations")).toContainText(
      "已完成",
      { timeout: 16000 },
    );
    await expect(
      page.getByRole("button", { name: "重启", exact: true }),
    ).toBeEnabled();
    await expect(page.getByTestId("native-server-detail")).toContainText(
      "203.0.113.9",
    );
    await expect(page.getByTestId("native-server-detail")).not.toContainText(
      "nativeServers.",
    );
    await view.screenshot("completed-operation");
  });

  test("TC-5b: 未知运行状态阻止操作并显示中文原因，窄屏不溢出", async ({
    page,
  }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.route(native, (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: detail({
            runtimeState: "unknown",
            observedAt: undefined,
            availableActions: [],
          }),
        },
      }),
    );
    const view = new NativeServerPage(page, origin, serverNo);
    await view.open();
    await expect(
      page.getByText("暂时无法确认上游运行状态，操作已禁用。请刷新重试。", {
        exact: true,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: "重启", exact: true }),
    ).toHaveCount(0);
    await expect(page.getByTestId("native-server-detail")).toContainText(
      "服务状态",
    );
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= window.innerWidth,
      ),
    ).toBe(true);
    await view.screenshot("unknown-mobile");
  });

  test("TC-5c: 初始密码显式确认、只展示一次、关闭即清除", async ({ page }) => {
    let reveals = 0;
    await page.route(native, (r) =>
      r.fulfill({
        json: { code: 0, data: detail({ credentialAvailable: reveals === 0 }) },
      }),
    );
    await page.route(native + "/initial-credential/reveal", (r) => {
      reveals += 1;
      return r.fulfill({
        headers: { "Cache-Control": "no-store" },
        json: { code: 0, data: { password: "Fixture-Password-Not-Real" } },
      });
    });
    const view = new NativeServerPage(page, origin, serverNo);
    await view.open();
    await page
      .getByRole("button", { name: "查看初始密码", exact: true })
      .click();
    await expect(
      page.getByText(
        "密码只展示一次，关闭窗口后将无法再次查看。请准备好安全保存。",
        { exact: true },
      ),
    ).toBeVisible();
    expect(reveals).toBe(0);
    await view.acceptImpact();
    await view.confirm();
    await expect(view.credentialInput).toHaveValue("Fixture-Password-Not-Real");
    await expect(
      page.getByRole("button", { name: "我已保存，关闭", exact: true }),
    ).toBeVisible();
    await page.keyboard.press("Escape");
    await expect(view.credentialInput).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "查看初始密码", exact: true }),
    ).toHaveCount(0);
    expect(reveals).toBe(1);
    await view.screenshot("credential-consumed");
  });

  test("TC-5d: 已交付订单解析到对应公开服务器", async ({ page }) => {
    await page.route("**/account/commerce/orders/ORD_native_test", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            order: {
              summary: {
                orderNo: "ORD_native_test",
                currencyCode: "CNY",
                statusCode: "paid",
                totalAmountMinor: "1000",
                createdAt: Date.now(),
                canCancel: false,
                canPreparePayment: false,
              },
              items: [],
              deliveries: [],
              reviews: [],
              serviceEntitlements: [
                {
                  entitlementNo,
                  entitlementOwnerId: "idc-resource",
                  orderItemNo: "item-native-test",
                  productNameZhCN: "标准云服务器",
                  productNameEnUS: "Standard cloud server",
                  statusCode: "active",
                  startedAt: Date.now(),
                },
              ],
            },
          },
        },
      }),
    );
    await page.route(
      `**/account/native-servers/resolve/${entitlementNo}`,
      (r) => r.fulfill({ json: { code: 0, data: { serverNo } } }),
    );
    await page.route(native, (r) =>
      r.fulfill({ json: { code: 0, data: detail() } }),
    );
    await page.goto(origin + "/account/orders/ORD_native_test");
    await page
      .getByTestId("commerce-service-entitlements")
      .getByRole("button", { name: "管理", exact: true })
      .click();
    await expect(page).toHaveURL(
      new RegExp(`/account/native-servers/${serverNo}$`),
    );
    await expect(page.getByTestId("native-server-detail")).toContainText(
      "标准云服务器",
    );
    await expect(page.getByTestId("native-server-detail")).toContainText(
      "运行中",
    );
  });
  test("TC-5e: 我的服务器列表从资源拥有方分页读取并进入对应详情", async ({
    page,
  }) => {
    await page.route("**/api/v1/plugins/dynamic", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            list: [
              {
                id: "idc-resource",
                installed: 1,
                enabled: 1,
                runtimeState: "normal",
              },
            ],
          },
        },
      }),
    );
    let detailReads = 0;
    await page.route("**/account/servers?*", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [], nextCursor: "" } } }),
    );
    await page.route("**/account/server-transfers/incoming*", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    await page.route("**/account/native-servers?*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            items: [
              {
                serverNo,
                productNameZhCn: "标准云服务器",
                productNameEnUs: "Standard cloud server",
                state: "active",
              },
            ],
            total: 1,
          },
        },
      }),
    );
    await page.route(native, (r) => {
      detailReads++;
      return r.fulfill({ json: { code: 0, data: detail() } });
    });
    await page.goto(origin + "/account/servers");
    const list = page.getByTestId("native-server-list");
    await expect(list).toContainText("标准云服务器");
    await expect(list).toContainText(serverNo);
    await expect(list).not.toContainText("nativeServers.");
    expect(detailReads).toBe(0);
    await expect(page.getByTestId("customer-server-list")).toHaveCount(0);
    await list.getByRole("button", { name: "管理", exact: true }).click();
    await expect(page).toHaveURL(
      new RegExp(`/account/native-servers/${serverNo}$`),
    );
    await expect(page.getByTestId("native-server-detail")).toContainText(
      "203.0.113.9",
    );
    await new NativeServerPage(page, origin, serverNo).screenshot("from-list");
  });

  test("TC-5f: 续费按当前报价生成账单，失败重试保留同一确认请求", async ({
    page,
  }) => {
    const confirmations: Array<Record<string, unknown>> = [];
    await page.route(native, (r) =>
      r.fulfill({ json: { code: 0, data: detail() } }),
    );
    await page.route(
      `**/account/entitlements/${entitlementNo}/renewal-quotes`,
      (r) => {
        expect(r.request().postDataJSON().renewalCycles).toBe(1);
        return r.fulfill({
          json: {
            code: 0,
            data: {
              quote: {
                quoteNo: "RQ_fixture_renewal",
                currencyCode: "CNY",
                totalAmountMinor: "12345",
                taxAmountMinor: "0",
                projectedExpiresAt: Date.now() + 86400000 * 31,
                expiresAt: Date.now() + 600000,
              },
            },
          },
        });
      },
    );
    await page.route(
      "**/account/renewal-quotes/RQ_fixture_renewal/confirmations",
      (r) => {
        confirmations.push(r.request().postDataJSON());
        return confirmations.length === 1
          ? r.fulfill({
              status: 503,
              json: { code: 50, message: "Unavailable" },
            })
          : r.fulfill({
              json: {
                code: 0,
                data: {
                  operation: {
                    operationNo: "RNO_fixture_renewal",
                    invoiceNo: "INV_fixture_renewal",
                    statusCode: "pending_payment",
                  },
                },
              },
            });
      },
    );
    const view = new NativeServerPage(page, origin, serverNo);
    await view.open();
    await page.getByTestId("native-server-renew").click();
    const modal = page.getByTestId("native-renewal-modal");
    await expect(modal).toContainText(
      "按当前商品价格核价，保留现有服务器配置。",
    );
    await expect(
      page.getByRole("button", { name: "确认并前往付款", exact: true }),
    ).toBeDisabled();
    await view.screenshot("renewal-open");
    await page.getByTestId("native-renewal-quote").click();
    await expect(page.getByTestId("native-renewal-preview")).toContainText(
      "123.45",
    );
    await expect(modal).toContainText("付款成功后才会延长服务期限");
    await expect(modal).not.toContainText("serverRenewal.");
    await page
      .getByRole("button", { name: "确认并前往付款", exact: true })
      .click();
    await expect(modal).toContainText("未能确认续费，请重试");
    await view.screenshot("renewal-retry");
    await page
      .getByRole("button", { name: "确认并前往付款", exact: true })
      .click();
    await expect(page).toHaveURL(
      /\/account\/billing\/invoices\/INV_fixture_renewal$/,
    );
    expect(confirmations).toHaveLength(2);
    expect(confirmations[0]).toEqual(confirmations[1]);
  });
  test("TC-5g: 修改续费周期使旧报价失效，禁止用旧金额确认", async ({
    page,
  }) => {
    let confirms = 0;
    await page.route(native, (r) =>
      r.fulfill({ json: { code: 0, data: detail() } }),
    );
    await page.route(
      `**/account/entitlements/${entitlementNo}/renewal-quotes`,
      (r) =>
        r.fulfill({
          json: {
            code: 0,
            data: {
              quote: {
                quoteNo: "RQ_old_cycle",
                currencyCode: "CNY",
                totalAmountMinor: "100",
                taxAmountMinor: "0",
                projectedExpiresAt: Date.now() + 86400000,
                expiresAt: Date.now() + 600000,
              },
            },
          },
        }),
    );
    await page.route("**/account/renewal-quotes/*/confirmations", (r) => {
      confirms++;
      return r.abort();
    });
    const view = new NativeServerPage(page, origin, serverNo);
    await view.open();
    await page.getByTestId("native-server-renew").click();
    await page.getByTestId("native-renewal-quote").click();
    await expect(page.getByTestId("native-renewal-preview")).toBeVisible();
    await page.getByTestId("native-renewal-cycles").fill("2");
    await page.getByTestId("native-renewal-cycles").blur();
    await expect(page.getByTestId("native-renewal-preview")).toHaveCount(0);
    await expect(
      page.getByRole("button", { name: "确认并前往付款", exact: true }),
    ).toBeDisabled();
    expect(confirms).toBe(0);
    await view.screenshot("renewal-changed");
  });

  test("TC-5h: 电源任务等待期间不误报初始密码已展示，结束后恢复查看入口", async ({
    page,
  }) => {
    let pending = true;
    await page.route(native, (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: detail({
            credentialAvailable: !pending,
            availableActions: pending ? [] : ["stop", "reboot"],
            operations: [
              {
                operationNo: "nso_pending_credential",
                action: "reboot",
                status: pending ? "running" : "succeeded",
                createdAt: Date.now(),
                updatedAt: Date.now(),
              },
            ],
          }),
        },
      }),
    );
    const view = new NativeServerPage(page, origin, serverNo);
    await view.open();
    const credentials = page.getByTestId("native-server-credential");
    await expect(credentials).toContainText(
      "操作正在执行，状态会自动更新。请等待当前任务完成。",
    );
    await expect(credentials).not.toContainText(
      "初始密码尚未可交付或已经展示过",
    );
    await expect(
      credentials.getByRole("button", { name: "查看初始密码", exact: true }),
    ).toHaveCount(0);
    await view.screenshot("credential-during-power");
    await new LoginPage(page).switchLanguage("English");
    await page.reload();
    await expect(credentials).toContainText(
      "The operation is in progress. Status updates automatically. Wait for the current task to finish.",
    );
    await expect(credentials).not.toContainText(
      "The initial password is not ready or was already delivered",
    );
    await expect(credentials).not.toContainText("nativeServers.");
    await view.screenshot("credential-during-power-english");
    pending = false;
    await page
      .getByRole("button", { name: "Refresh status", exact: true })
      .click();
    await expect(
      credentials.getByRole("button", {
        name: "View initial password",
        exact: true,
      }),
    ).toBeVisible();
    await expect(credentials).not.toContainText(
      "Wait for the current task to finish",
    );
    await view.screenshot("credential-after-power");
  });

  for (const [id, english] of [
    ["i", false],
    ["j", true],
  ] as const) {
    test(`TC-5${id}: ${english ? "English" : "中文"}未知状态只读重查后恢复实际服务器`, async ({
      page,
    }) => {
      let reads = 0,
        writes = 0,
        recovered = false;
      page.on("request", (request) => {
        if (
          request.url().includes(`/native-servers/${serverNo}`) &&
          request.method() !== "GET"
        )
          writes += 1;
      });
      await page.route(native, (r) => {
        reads += 1;
        return r.fulfill({
          json: {
            code: 0,
            data: detail(
              !recovered
                ? {
                    runtimeState: "unknown",
                    observedAt: undefined,
                    availableActions: [],
                    addresses: [],
                    addressesConfirmed: false,
                    credentialAvailable: false,
                  }
                : { credentialAvailable: true },
            ),
          },
        });
      });
      if (english) {
        await page.waitForURL(origin + "/account/billing/invoices");
        await page.waitForLoadState("networkidle");
        await new LoginPage(page).switchLanguage("English");
      }
      await page.clock.install();
      const view = new NativeServerPage(page, origin, serverNo);
      await view.open();
      const credentials = page.getByTestId("native-server-credential");
      await expect(credentials).toContainText(
        english
          ? "The server state is temporarily unavailable. Refresh the status before viewing the initial password."
          : "暂时无法读取服务器状态，请稍后刷新再查看初始密码。",
      );
      await expect(
        page.getByRole("button", {
          name: english ? "Reboot" : "重启",
          exact: true,
        }),
      ).toHaveCount(0);
      await view.screenshot(
        `observation-unavailable-${english ? "english" : "chinese"}`,
      );
      const initialReads = reads;
      recovered = true;
      await page.clock.fastForward(5000);
      await expect(page.getByTestId("native-server-detail")).toContainText(
        "203.0.113.9",
      );
      await expect(
        page.getByRole("button", {
          name: english ? "Reboot" : "重启",
          exact: true,
        }),
      ).toBeEnabled();
      await expect(
        credentials.getByRole("button", {
          name: english ? "View initial password" : "查看初始密码",
          exact: true,
        }),
      ).toBeVisible();
      await expect(credentials).not.toContainText("nativeServers.");
      await page.clock.fastForward(60000);
      expect(reads).toBe(initialReads + 1);
      expect(writes).toBe(0);
      await view.screenshot(
        `observation-recovered-${english ? "english" : "chinese"}`,
      );
    });
  }

  test("TC-5k: 持续未知最多三次重查，人工刷新可恢复且隐藏或离开页面不继续查询", async ({
    page,
  }) => {
    await page.route("**/api/v1/plugins/dynamic", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            list: [
              {
                id: "idc-resource",
                installed: 1,
                enabled: 1,
                runtimeState: "normal",
              },
            ],
          },
        },
      }),
    );
    let reads = 0,
      recovered = false,
      emptyServerReads = 0;
    await page.route("**/account/native-servers?*", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [], total: 0 } } }),
    );
    await page.route("**/account/native-servers/", (r) => {
      emptyServerReads += 1;
      return r.fulfill({
        json: {
          code: 0,
          data: detail({ runtimeState: "unknown", availableActions: [] }),
        },
      });
    });
    await page.route(native, (r) => {
      reads += 1;
      return r.fulfill({
        json: {
          code: 0,
          data: detail(
            recovered
              ? {}
              : {
                  runtimeState: "unknown",
                  observedAt: undefined,
                  availableActions: [],
                  addresses: [],
                  addressesConfirmed: false,
                },
          ),
        },
      });
    });
    await page.clock.install();
    const view = new NativeServerPage(page, origin, serverNo);
    await view.open();
    await expect.poll(() => reads).toBe(1);
    for (let count = 2; count <= 4; count += 1) {
      await page.clock.fastForward(5000);
      await expect.poll(() => reads).toBe(count);
    }
    await page.clock.fastForward(60000);
    expect(reads).toBe(4);
    await expect(
      page.getByRole("button", { name: "重启", exact: true }),
    ).toHaveCount(0);
    recovered = true;
    await view.refresh();
    await expect(
      page.getByRole("button", { name: "重启", exact: true }),
    ).toBeEnabled();
    expect(reads).toBe(5);
    recovered = false;
    await view.refresh();
    await expect.poll(() => reads).toBe(6);
    await page.evaluate(() =>
      Object.defineProperty(document, "hidden", {
        value: true,
        configurable: true,
      }),
    );
    await page.clock.fastForward(60000);
    expect(reads).toBe(6);
    await page.evaluate(() =>
      Object.defineProperty(document, "hidden", {
        value: false,
        configurable: true,
      }),
    );
    await view.refresh();
    await expect.poll(() => reads).toBe(7);
    await view.backToServers();
    await expect(page).toHaveURL(origin + "/account/servers");
    await expect(page.getByTestId("native-server-list")).toBeVisible();
    await page.clock.fastForward(60000);
    expect(reads).toBe(7);
    expect(emptyServerReads).toBe(0);
  });

  test("TC-5l: 迟到的未知重查响应不覆盖手动刷新的新状态", async ({ page }) => {
    let reads = 0;
    let release: (() => void) | undefined;
    const delayed = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route(native, async (r) => {
      reads += 1;
      const index = reads;
      if (index === 2) await delayed;
      await r.fulfill({
        json: {
          code: 0,
          data: detail(
            index < 3
              ? {
                  runtimeState: "unknown",
                  observedAt: undefined,
                  availableActions: [],
                  addresses: [],
                  addressesConfirmed: false,
                }
              : { credentialAvailable: true },
          ),
        },
      });
    });
    await page.clock.install();
    const view = new NativeServerPage(page, origin, serverNo);
    await view.open();
    await expect.poll(() => reads).toBe(1);
    await page.clock.fastForward(5000);
    await expect.poll(() => reads).toBe(2);
    await view.refresh();
    await expect(
      page.getByRole("button", { name: "重启", exact: true }),
    ).toBeEnabled();
    const response = page.waitForResponse(
      (r) =>
        r.url() ===
        origin + `/x/idc-resource/api/v1/account/native-servers/${serverNo}`,
    );
    release!();
    await response;
    await expect(page.getByTestId("native-server-detail")).toContainText(
      "203.0.113.9",
    );
    await expect(
      page.getByRole("button", { name: "重启", exact: true }),
    ).toBeEnabled();
    await page.clock.fastForward(60000);
    expect(reads).toBe(3);
    await view.screenshot("observation-late-read-discarded");
  });
});

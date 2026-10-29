// UI-only HTTP fixtures prove independent panels; actual paid provisioning is
// exercised against the isolated database by idc-core TC066 and TC070.
import type { Page } from "@host-tests/support/playwright";
import { test, expect } from "@host-tests/fixtures/auth";
import { LoginPage } from "@host-tests/pages/LoginPage";
import { AccountConsolePage } from "../pages/AccountConsolePage";
const origin = process.env.CUSTOMER_E2E_BASE_URL || "http://127.0.0.1:6032";
const serverNo = "SVR-77777777777777777777777777777777";
async function fixture(page: Page, enabled = true) {
  const state = {
    managementPreparation: undefined as
      | "preparing"
      | "ready"
      | "needs_attention"
      | undefined,
    lists: 0,
    details: 0,
    financeFail: false,
    servicesFail: false,
    serviceReads: 0,
  };
  await page.route("**/api/v1/auth/login", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          accessToken: "fixture-console",
          refreshToken: "fixture-console-refresh",
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
          id: 707,
          username: "console-fixture",
          realName: "控制台测试客户",
          roles: ["user"],
          permissions: [],
          menus: [],
          homePath: "/account",
        },
      },
    }),
  );
  await page.route("**/api/v1/plugins/dynamic", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          list: [
            {
              id: "idc-resource",
              installed: 1,
              enabled: enabled ? 1 : 0,
              runtimeState: "normal",
            },
          ],
        },
      },
    }),
  );
  await page.route("**/customer/external-menus", (r) =>
    r.fulfill({ json: { code: 0, data: { items: [] } } }),
  );
  await page.route("**/account/product-directories*", (r) =>
    r.request().isNavigationRequest()
      ? r.continue()
      : r.fulfill({ json: { code: 0, data: { items: [], total: 0 } } }),
  );
  await page.route("**/account/product-directories/all/services?*", (r) => {
    state.serviceReads++;
    return state.servicesFail
      ? r.fulfill({
          status: 503,
          json: { code: 50, message: "隔离验收：暂时无法读取服务" },
        })
      : r.fulfill({
          json: {
            code: 0,
            data: {
              total: 37,
              items: [
                {
                  entitlementNo: "ENT_console_storage",
                  managementPreparation: state.managementPreparation,
                  productNameZhCn: "通用对象存储",
                  productNameEnUs: "Object storage",
                  statusCode: "active",
                  requiresManualProcessing: false,
                  expiresAt: null,
                  orderNo: "ORD_console_fixture",
                },
                {
                  entitlementNo: "ENT_console_manual",
                  productNameZhCn: "人工交付证书",
                  productNameEnUs: "Manually delivered certificate",
                  statusCode: "pending_activation",
                  requiresManualProcessing: true,
                  expiresAt: null,
                  orderNo: "ORD_console_fixture",
                },
              ],
            },
          },
        });
  });
  await page.route("**/account/initial-password/status", (r) =>
    r.fulfill({ json: { code: 0, data: { required: false } } }),
  );
  await page.route("**/account/customer/profile", (r) =>
    r.fulfill({
      json: { code: 0, data: { item: { displayName: "控制台测试客户" } } },
    }),
  );
  await page.route("**/account/commerce/orders?*", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          orders: [
            {
              orderNo: "ORD_console_fixture",
              totalAmountMinor: "1100",
              currencyCode: "CNY",
              statusCode: "paid",
              createdAt: 1792500000000,
              canPreparePayment: false,
            },
          ],
          nextCursor: "",
        },
      },
    }),
  );
  await page.route("**/account/billing/overview", (r) =>
    state.financeFail
      ? r.fulfill({
          status: 503,
          json: { code: 50, message: "fixture balance unavailable" },
        })
      : r.fulfill({
          json: {
            code: 0,
            data: {
              overview: {
                balances: [
                  {
                    accountTypeCode: "customer_balance",
                    currencyCode: "CNY",
                    availableBalanceMinor: "9223372036854775807",
                  },
                ],
                openInvoiceCount: 3,
              },
            },
          },
        }),
  );
  await page.route(/\/account\/native-servers\?(?:.*)$/, (r) => {
    state.lists++;
    return r.fulfill({
      json: {
        code: 0,
        data: {
          items: [
            {
              serverNo,
              productNameZhCn: "控制台陕西服务器",
              productNameEnUs: "Console Shaanxi server",
              state: "active",
            },
          ],
          total: 73,
        },
      },
    });
  });
  await page.route("**/account/native-servers/" + serverNo, (r) => {
    state.details++;
    return r.fulfill({
      json: {
        code: 0,
        data: {
          serverNo,
          entitlementNo: "ent_console",
          productNameZhCn: "控制台陕西服务器",
          productNameEnUs: "Console Shaanxi server",
          entitlementStatus: "active",
          runtimeState: "active",
          orderNo: "ORD_console_fixture",
          startedAt: 1792500000000,
          expiresAt: 1795092000000,
          availableActions: [],
          operations: [],
          addressesConfirmed: true,
          addresses: ["203.0.113.7"],
          credentialAvailable: false,
        },
      },
    });
  });
  return state;
}
async function login(page: Page) {
  await page.goto(origin + "/auth/login");
  await new LoginPage(page).login("console-fixture", "fixture-password");
  await page.getByTestId("customer-console").waitFor();
}
test.describe("TC-7 客户控制台与服务器入口", () => {
  test.setTimeout(45000);
  test("TC-7f: 非服务器服务的管理准备显示在首页及全部服务，原状态和总数保留", async ({
    page,
  }) => {
    const state = await fixture(page, false);
    state.managementPreparation = "needs_attention";
    await login(page);
    const view = new AccountConsolePage(page, origin);
    await expect(
      view.services.getByTestId("console-management-preparation"),
    ).toContainText("管理入口需要管理员处理，请联系支持。");
    await expect(view.services).toContainText("通用对象存储");
    await expect(view.serviceTotal).toContainText("37");
    await view.screenshot("generic-management-attention");
    await view.openAllServices();
    const list = page.getByTestId("customer-product-directories"),
      row = list
        .locator(".vxe-table--main-wrapper .vxe-body--row")
        .filter({ hasText: "通用对象存储" });
    await expect(row).toContainText("运行中");
    await expect(
      row.getByTestId("directory-management-preparation"),
    ).toContainText("管理入口需要管理员处理，请联系支持。");
    await expect(
      row.getByRole("button", { name: "查看订单", exact: true }),
    ).toBeVisible();
    await expect(list).toContainText("37");
    state.managementPreparation = "ready";
    await page.reload();
    await expect(
      row.getByTestId("directory-management-preparation"),
    ).toHaveCount(0);
    await expect(row).toContainText("运行中");
    expect(state.details).toBe(0);
  });
  test("TC-7a: 显示准确余额、分页总数和最近订单，直接管理已开通服务器", async ({
    page,
  }) => {
    const state = await fixture(page);
    await login(page);
    const view = new AccountConsolePage(page, origin);
    await expect(view.root).toContainText("控制台测试客户，欢迎回来");
    await expect(view.root).toContainText("92,233,720,368,547,758.07");
    await expect(view.servers).toContainText("控制台陕西服务器");
    await expect(view.servers).toContainText("73");
    await expect(view.orders).toContainText("ORD_console_fixture");
    await expect(view.services).toContainText("通用对象存储");
    await expect(view.services).toContainText("人工交付证书");
    await expect(view.services).toContainText("待人工处理");
    await expect(view.serviceTotal).toContainText("37");
    await expect(
      view.root.getByRole("button", { name: "购买产品", exact: true }),
    ).toBeVisible();
    await expect(view.orders).toContainText("11.00");
    await expect(view.root).not.toContainText("accountConsole.");
    expect(state.details).toBe(0);
    await view.screenshot("loaded");
    await view.manageFirst();
    await expect(page.getByTestId("native-server-detail")).toContainText(
      "203.0.113.7",
    );
    expect(state.details).toBe(1);
    await view.screenshot("managed");
  });
  test("TC-7b: 余额失败不影响服务器和订单，刷新后可恢复", async ({ page }) => {
    const state = await fixture(page);
    state.financeFail = true;
    await login(page);
    const view = new AccountConsolePage(page, origin);
    await expect(view.root).toContainText("余额暂时无法读取，请重试。");
    await expect(view.servers).toContainText("控制台陕西服务器");
    await expect(view.orders).toContainText("ORD_console_fixture");
    state.financeFail = false;
    await view.retryBalance();
    await expect(view.root).toContainText("92,233,720,368,547,758.07");
    await expect(view.root).not.toContainText("余额暂时无法读取");
    await view.screenshot("balance-recovered");
  });
  test("TC-7c: 资源插件停用不读取服务器，独立入口显示原因且窄屏不溢出", async ({
    page,
  }) => {
    const state = await fixture(page, false);
    await page.setViewportSize({ width: 390, height: 844 });
    await login(page);
    const view = new AccountConsolePage(page, origin);
    await expect(view.orders).toContainText("ORD_console_fixture");
    await expect(view.servers).toHaveCount(0);
    await expect(view.services).toContainText("通用对象存储");
    await expect(view.serviceTotal).toContainText("37");
    expect(state.lists).toBe(0);
    expect(
      await page.evaluate(
        () => document.documentElement.scrollWidth <= innerWidth,
      ),
    ).toBe(true);
    await view.screenshot("mobile");
    await view.openServers();
    await expect(page.getByTestId("customer-server-console")).toContainText(
      "云服务器服务暂不可用，请联系支持人员。",
    );
    await expect(
      page.getByRole("button", { name: "其他服务器服务", exact: true }),
    ).toHaveCount(0);
    expect(state.lists).toBe(0);
    await view.screenshot("disabled");
  });
  test("TC-7d: 通用服务读取失败独立重试，恢复准确总数和非服务器商品", async ({
    page,
  }) => {
    const state = await fixture(page, false);
    state.servicesFail = true;
    await login(page);
    const view = new AccountConsolePage(page, origin);
    await expect(view.services).toContainText("服务暂时无法读取，请重试。");
    await expect(view.serviceTotal).toContainText("—");
    await expect(view.orders).toContainText("ORD_console_fixture");
    await expect(view.root).toContainText("92,233,720,368,547,758.07");
    await view.screenshot("services-failed");
    state.servicesFail = false;
    await view.retryServices();
    await expect(view.services).toContainText("人工交付证书");
    await expect(view.services).not.toContainText("服务暂时无法读取");
    await expect(view.serviceTotal).toContainText("37");
    expect(state.serviceReads).toBe(2);
    expect(state.lists).toBe(0);
    await view.screenshot("services-recovered");
    await view.openAllServices();
    await expect(
      page.getByTestId("customer-product-directories"),
    ).toBeVisible();
    await expect(page).toHaveURL(/\/account\/product-directories$/);
  });
  test("TC-7e: 英文首页使用通用产品与服务文案，停用服务器不影响服务预览", async ({
    page,
  }) => {
    const state = await fixture(page, false);
    await login(page);
    await new LoginPage(page).switchLanguage("English");
    await page.reload();
    const view = new AccountConsolePage(page, origin);
    await expect(
      view.root.getByRole("button", { name: "Buy products", exact: true }),
    ).toBeVisible();
    await expect(view.services).toContainText("My services");
    await expect(view.services).toContainText("Object storage");
    await expect(view.services).toContainText("Manually delivered certificate");
    await expect(view.services).toContainText("Needs staff attention");
    await expect(view.serviceTotal).toContainText("37");
    await expect(view.root).not.toContainText("accountConsole.");
    expect(state.lists).toBe(0);
    await view.screenshot("generic-services-en");
  });
});

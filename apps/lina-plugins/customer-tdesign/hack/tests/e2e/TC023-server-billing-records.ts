import { test, expect, now } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
const server = {
  serverNo: "SVR-billing",
  entitlementNo: "ENT_billing",
  productNameZhCn: "应用服务器",
  runtimeState: "stopped",
  entitlementStatus: "suspended",
  addressesConfirmed: true,
  addresses: [],
  orderNo: "ORD-TEST-001",
  periodUnit: "month",
  periodCount: 1,
  startedAt: now,
  availableActions: [],
  operations: [],
  credentialAvailable: false,
};
test.describe("TC-23 服务器增购记录与附属账期", () => {
  test("TC-23a 暂停服务仍可查看真实增购状态，游标翻页且未知结果不显示成功", async ({
    consolePage: page,
  }) => {
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-billing",
      (r) => r.fulfill({ json: { code: 0, data: server } }),
    );
    await page.route("**/native-servers/SVR-billing/management-options", (r) =>
      r.fulfill({ json: { code: 0, data: { images: [], accessories: [] } } }),
    );
    const queries: string[] = [];
    await page.route(
      "**/native-servers/SVR-billing/commercial-operations?*",
      (r) => {
        const url = new URL(r.request().url());
        queries.push(url.search);
        const next = url.searchParams.has("cursor");
        return r.fulfill({
          json: {
            code: 0,
            data: {
              items: [
                {
                  operationNo: next ? "BOP_second" : "BOP_first",
                  quoteNo: "nqt_owned",
                  orderNo: "ORD-TEST-001",
                  invoiceNo: "INV-TEST-001",
                  currencyCode: "CNY",
                  totalAmountMinor: "9007199254740993",
                  action: next ? "traffic_add" : "backup_create",
                  status: next
                    ? "awaiting_financial_settlement"
                    : "manual_review",
                  lastErrorCode: "",
                  createdAt: now,
                  updatedAt: now,
                  completedAt: null,
                },
              ],
              nextCursor: next ? "" : "cursor-owned",
            },
          },
        });
      },
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-billing");
    await page.getByRole("tab", { name: "操作记录", exact: true }).click();
    const card = view.main
      .locator(".t-card")
      .filter({ hasText: "增购与配置变更记录" });
    await expect(card).toContainText("需要核对");
    await expect(card).toContainText("勿重复办理");
    await expect(card).not.toContainText("已完成");
    await expect(card).toContainText("CNY 90,071,992,547,409.93");
    await card.getByRole("button", { name: "下一页", exact: true }).click();
    await expect(card).toContainText("等待付款");
    await expect
      .poll(() => queries.some((q) => q.includes("cursor=cursor-owned")))
      .toBe(true);
    await expect(
      card.getByRole("link", { name: "查看账单", exact: true }),
    ).toHaveAttribute("href", "/account/billing/invoices/INV-TEST-001");
    await view.screenshot("server-commercial-cursor-history");
  });
  test("TC-23b 仅真实对象匹配名称，免费无假到期，待付账单金额为空不显示零", async ({
    consolePage: page,
  }) => {
    await page.route(
      "**/x/idc-resource/api/v1/account/native-servers/SVR-billing",
      (r) => r.fulfill({ json: { code: 0, data: server } }),
    );
    await page.route("**/native-servers/SVR-billing/management-options", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            images: [],
            accessories: [
              {
                objectNo: "ACO_paid",
                kind: "backup",
                name: "月度备份",
                state: "available",
                projection: {},
              },
            ],
          },
        },
      }),
    );
    await page.route("**/native-servers/SVR-billing/accessory-billing?*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            items: [
              {
                accessoryEntitlementNo: "AEN_paid",
                objectNo: "ACO_paid",
                kind: "backup",
                billingMode: "paid",
                status: "payment_pending",
                currencyCode: "CNY",
                periodUnit: "month",
                periodCount: 1,
                currentPeriodStartAt: now,
                currentPeriodEndAt: now + 86400000,
                autoRenewEnabled: true,
                pendingInvoiceNo: "INV-TEST-001",
                pendingInvoiceStatus: "open",
                pendingAmountMinor: null,
                createdAt: now,
                updatedAt: now,
              },
              {
                accessoryEntitlementNo: "AEN_free",
                objectNo: "ACO_unmapped",
                kind: "snapshot",
                billingMode: "free",
                status: "active",
                currencyCode: "CNY",
                periodUnit: "",
                periodCount: 0,
                currentPeriodStartAt: null,
                currentPeriodEndAt: null,
                autoRenewEnabled: false,
                pendingInvoiceNo: "",
                pendingInvoiceStatus: "",
                pendingAmountMinor: null,
                createdAt: now,
                updatedAt: now,
              },
            ],
            nextCursor: "",
          },
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/native-servers/SVR-billing");
    await page.getByRole("tab", { name: "快照与备份", exact: true }).click();
    const card = view.main
      .locator(".t-card")
      .filter({ hasText: "快照与备份账期" });
    await expect(card).toContainText("月度备份");
    await expect(card).toContainText("待续费付款");
    await expect(card).toContainText("账单总额：尚未确认");
    await expect(card).not.toContainText("CNY 0.00");
    await expect(card).toContainText("ACO_unmapped");
    await expect(card).toContainText("免费对象");
    await expect(card).not.toContainText("1970");
    await expect(card).toContainText("付款后仍需确认资源状态，恢复操作以实际可用为准");
    await expect(
      card.getByRole("link", { name: "查看并支付", exact: true }),
    ).toHaveAttribute("href", "/account/billing/invoices/INV-TEST-001");
    await view.screenshot("server-accessory-billing");
    await page.setViewportSize({ width: 390, height: 844 });
    await expect
      .poll(() =>
        page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      )
      .toBe(true);
    await view.screenshot("server-accessory-billing-mobile");
  });
});

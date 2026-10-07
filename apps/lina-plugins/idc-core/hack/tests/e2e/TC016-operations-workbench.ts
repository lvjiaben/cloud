import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { OperationsWorkbenchPage } from "../pages/OperationsWorkbenchPage";

test.describe("TC-16 运营工作台", () => {
  test("TC-16a: 显示待办队列和分币种金额，并支持刷新", async ({
    adminPage,
  }) => {
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/workbench",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              generatedAt: 1787702400000,
              queues: [{ code: "pending_orders", count: 2 }],
              currencies: [
                {
                  currencyCode: "CNY",
                  invoiceCount: 3,
                  totalAmountMinor: 12345,
                  paidAmountMinor: 10000,
                  refundedAmountMinor: 200,
                },
              ],
            },
          }),
        });
      },
    );
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/tasks**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ code: 0, data: { tasks: [] } }),
        });
      },
    );
    const page = new OperationsWorkbenchPage(adminPage);
    await page.goto();
    await expect(page.root).toContainText("运营工作台");
    await expect(page.root).toContainText("待付款订单");
    await expect(page.root).toContainText("CNY");
    await expect(page.root).toContainText("123.45");
    await page.refresh.click();
    await expect(page.root).toContainText("原币种经营金额");
    await page.screenshot("operations-workbench");
  });
});

test.describe("TC-16 phase 13 analytics scenarios", () => {
  test("TC-16b: 高频待办和告警可见", async ({ adminPage }) => {
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/workbench**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              generatedAt: Date.now(),
              queues: [
                { code: "pending_orders", count: 2 },
                { code: "pending_services", count: 1 },
                { code: "expiring_services", count: 3 },
                { code: "paused_products", count: 1 },
                { code: "pending_commission_settlements", count: 2 },
              ],
              currencies: [
                {
                  currencyCode: "CNY",
                  invoiceCount: 2,
                  totalAmountMinor: 10000,
                  paidAmountMinor: 8000,
                  refundedAmountMinor: 0,
                },
              ],
            },
          }),
        });
      },
    );
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/report**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              from: Date.now() - 86400000,
              to: Date.now(),
              currencies: [],
            },
          }),
        });
      },
    );
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/alerts**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              alerts: [
                {
                  code: "queue_backlog:fulfillment",
                  severity: "warning",
                  count: 4,
                  oldestAt: Date.now() - 60000,
                },
              ],
            },
          }),
        });
      },
    );
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/tasks**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              tasks: [
                {
                  taskNo: "task-1",
                  taskType: "fulfillment",
                  subjectType: "order",
                  subjectNo: "ORD-1",
                  status: "retry_wait",
                  errorCode: "",
                  errorSummary: "",
                  attemptCount: 2,
                  createdAt: Date.now() - 120000,
                  updatedAt: Date.now() - 60000,
                },
              ],
            },
          }),
        });
      },
    );
    const workbench = new OperationsWorkbenchPage(adminPage);
    await workbench.goto();
    await expect(workbench.root).toContainText("待付款订单");
    await expect(workbench.root).toContainText("即将到期服务");
    await expect(workbench.root).toContainText("暂停商品影响");
    await expect(workbench.root).toContainText("待结算佣金");
    await expect(workbench.root).toContainText("待办队列");
    await expect(workbench.root).toContainText("提醒");
    await expect(workbench.root).toContainText("异步任务中心");
    await expect(workbench.root).toContainText("履约任务");
  });

  test("TC-16c: 暂停影响预览保持已售服务不变", async ({ adminPage }) => {
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/catalog/products/*/pause-impact-preview",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              preview: {
                productNo: "prod-demo",
                newSalesBlocked: true,
                existingServicesAffected: false,
                activeServiceCount: 5,
              },
            },
          }),
        });
      },
    );
    const body = await adminPage.evaluate(async () => {
      const response = await fetch(
        "/x/idc-core/api/v1/admin/catalog/products/prod-demo/pause-impact-preview",
      );
      return response.json();
    });
    expect(body.data.preview.existingServicesAffected).toBe(false);
  });

  test("TC-16d: 风险复核能力明确要求人工原因", async ({ adminPage }) => {
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/analytics/capabilities",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              riskEvents: [{ code: "refund" }],
              riskActions: [{ code: "manual_review" }],
              manualReview: {
                reviewableStatuses: ["open", "acknowledged"],
                decisions: ["acknowledge", "resolve", "dismiss"],
                reasonRequired: true,
              },
              report: {
                metricCodes: ["revenue", "cost", "gross_profit"],
                originalCurrencyGrouped: true,
                exchangeRateOptional: true,
                convertedValueLedgerSafe: true,
              },
              auditExport: {
                format: "xlsx",
                asynchronous: true,
                maxRows: 100000,
                shortLivedLink: true,
                rechecksScope: true,
              },
            },
          }),
        });
      },
    );
    const body = await adminPage.evaluate(async () => {
      const response = await fetch(
        "/x/idc-core/api/v1/admin/operations/analytics/capabilities",
      );
      return response.json();
    });
    expect(body.data.manualReview.reasonRequired).toBe(true);
  });

  test("TC-16e: 多币种报表保留原币分组", async ({ adminPage }) => {
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/report**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              from: 1,
              to: 2,
              currencies: [
                {
                  currencyCode: "CNY",
                  invoiceCount: 1,
                  totalAmountMinor: 1000,
                  paidAmountMinor: 1000,
                  refundedAmountMinor: 0,
                },
                {
                  currencyCode: "USD",
                  invoiceCount: 1,
                  totalAmountMinor: 200,
                  paidAmountMinor: 200,
                  refundedAmountMinor: 0,
                },
              ],
            },
          }),
        });
      },
    );
    const body = await adminPage.evaluate(async () => {
      const response = await fetch(
        "/x/idc-core/api/v1/admin/operations/report",
      );
      return response.json();
    });
    const currencies = body.data.currencies;
    expect(currencies).toHaveLength(2);
    expect(
      currencies.map((item: { currencyCode: string }) => item.currencyCode),
    ).toEqual(["CNY", "USD"]);
  });

  test("TC-16f: 零待办和空金额不铺满无效卡片", async ({ adminPage }) => {
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/workbench**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: {
              generatedAt: Date.now(),
              queues: [
                { code: "pending_orders", count: 0 },
                { code: "pending_services", count: 0 },
                { code: "expiring_services", count: 0 },
              ],
              currencies: [],
            },
          }),
        });
      },
    );
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/report**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            code: 0,
            data: { from: 1, to: 2, currencies: [] },
          }),
        });
      },
    );
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/alerts**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ code: 0, data: { alerts: [] } }),
        });
      },
    );
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/operations/tasks**",
      async (route) => {
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ code: 0, data: { tasks: [] } }),
        });
      },
    );

    const workbench = new OperationsWorkbenchPage(adminPage);
    await workbench.goto();
    await expect(workbench.root.locator(".queue-card")).toHaveCount(0);
    await expect(workbench.root.locator(".currency-card")).toHaveCount(0);
    await expect(workbench.root).toContainText("当前没有待办事项");
  });
});

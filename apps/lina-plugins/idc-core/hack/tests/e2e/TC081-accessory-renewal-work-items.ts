import { test, expect } from "@host-tests/fixtures/auth";
import { config } from "@host-tests/fixtures/config";
import { AccessoryRenewalPage } from "../pages/AccessoryRenewalPage";
import { installAccessoryRenewalMock, renewalPermission, workItems } from "../support/accessory-renewal-mock";

test.describe("TC-81 员工附属资源续费待办", () => {
  test.skip(!process.env.E2E_RENEWAL_SOURCE_RUNTIME && !process.env.E2E_RENEWAL_DEPLOYED, "Requires an isolated source runtime or an explicitly selected deployment check");

  test("TC-81a: 资源菜单展示双语待办、服务端分页和受权限保护的账单入口", async ({ browser }) => {
    const context = await browser.newContext({ baseURL: config.baseURL });
    try {
      const page = await context.newPage();
      const mock = await installAccessoryRenewalMock(page);
      const view = new AccessoryRenewalPage(page);
      await view.login(); await view.locale("zh-CN");
      await expect(view.headerTrigger).toHaveCount(0);
      await view.open();
      await expect(view.trigger).toHaveText(/附属资源续费待办\s*\(21\)/);
      await view.screenshot("resource-page");
      await expect(view.content.getByText("待付款", { exact: true }).first()).toBeVisible();
      await expect(view.content.getByText("账单关闭待处理", { exact: true }).first()).toBeVisible();
      await expect(view.content.getByText("snapshot-1", { exact: true })).toBeVisible();
      await expect(view.content.getByText("snapshot-21", { exact: true })).toHaveCount(0);
      expect(mock.listQueries.at(-1)).toEqual({ page: 1, pageSize: 20 });
      await view.screenshot("drawer");
      await view.nextPage();
      await expect(view.content.getByText("snapshot-21", { exact: true })).toBeVisible();
      expect(mock.listQueries.at(-1)).toEqual({ page: 2, pageSize: 20 });
      await view.screenshot("page-two");
      await view.pageSize50();
      await expect.poll(() => mock.listQueries.at(-1)).toEqual({ page: 1, pageSize: 50 });
      await expect(view.content.getByText("snapshot-21", { exact: true })).toBeVisible();
      await view.locale("en-US");
      await expect(view.trigger).toHaveText(/Accessory renewal work items\s*\(21\)/);
      for (const title of ["Service number", "Customer number", "Invoice number", "Reason", "Created at", "Action"]) {
        await expect(view.content.locator(".vxe-header--column").filter({ hasText: title }).first()).toBeVisible();
      }
      await expect(view.content.getByText("Awaiting payment", { exact: true }).first()).toBeVisible();
      await expect(view.content.getByText("Invoice closure pending", { exact: true }).first()).toBeVisible();
      await expect(page.getByTestId("accessory-renewal-invoice-renewal-work-1")).toBeVisible();
      expect(mock.syncBodies).toContainEqual({ locale: "en-US" });
      expect(mock.syncBodies).toContainEqual({ locale: "zh-CN" });
      await view.screenshot("english");
      await view.viewInvoice("renewal-work-1");
      await expect(page).toHaveURL(/\/idc\/finance\?invoiceNo=renewal-invoice-1/);
    } finally { await context.close(); }
  });

  test("TC-81b: 同步与列表失败可重试，无财务权限不显示账单动作", async ({ browser }) => {
    const context = await browser.newContext({ baseURL: config.baseURL });
    try {
      const page = await context.newPage();
      await page.clock.install();
      const mock = await installAccessoryRenewalMock(page, [renewalPermission]);
      mock.failSync = true; mock.failList = true;
      const view = new AccessoryRenewalPage(page);
      await view.login(); await view.locale("zh-CN"); await view.open();
      await expect(view.syncError).toContainText("请刷新重试");
      await expect(view.listError).toContainText("请刷新重试");
      await expect(page.getByTestId("accessory-renewal-invoice-renewal-work-1")).toHaveCount(0);
      await view.screenshot("recoverable-errors");
      mock.failSync = false; mock.failList = false;
      await view.refresh();
      await expect(view.trigger).toHaveText(/附属资源续费待办\s*\(21\)/);
      await expect(view.content.getByText("snapshot-1", { exact: true })).toBeVisible();
      await expect(view.syncError).toHaveCount(0); await expect(view.listError).toHaveCount(0);
      await expect(page.getByTestId("accessory-renewal-invoice-renewal-work-1")).toHaveCount(0);
      await view.screenshot("recovered");
      const before = mock.syncBodies.length;
      await page.clock.fastForward(60_000);
      await expect.poll(() => mock.syncBodies.length).toBeGreaterThan(before);
      await context.setOffline(true);
      const offlineCount = mock.syncBodies.length;
      await page.clock.fastForward(60_000);
      expect(mock.syncBodies.length).toBe(offlineCount);
      await context.setOffline(false);
      await expect.poll(() => mock.syncBodies.length).toBeGreaterThan(offlineCount);
    } finally { await context.close(); }
  });

  test("TC-81c: 权限撤销、会话结束和离开重进隔离迟到结果", async ({ browser }) => {
    const context = await browser.newContext({ baseURL: config.baseURL });
    try {
      const page = await context.newPage();
      await page.clock.install();
      const mock = await installAccessoryRenewalMock(page, [renewalPermission]);
      const view = new AccessoryRenewalPage(page);
      await view.login(); await view.locale("zh-CN");
      mock.holdNextList = true; await view.open();
      await expect.poll(() => Boolean(mock.heldList)).toBe(true);
      await view.close(); await view.open();
      await expect(view.content.getByText("snapshot-1", { exact: true })).toBeVisible();
      await mock.heldList!.fulfill({ json: { code: 0, data: { items: [{ ...workItems[0], subjectNo: "STALE-CLOSED-DRAWER" }], total: 1 } } });
      await expect(view.content.getByText("STALE-CLOSED-DRAWER")).toHaveCount(0);
      mock.heldList = undefined; mock.holdNextList = true; await view.refresh();
      await expect.poll(() => Boolean(mock.heldList)).toBe(true);
      await view.changeScope("permissions", []);
      await expect(view.trigger).toHaveCount(0); await expect(view.content).toBeHidden();
      await mock.heldList!.fulfill({ json: { code: 0, data: { items: workItems, total: 21 } } });
      await expect(page.getByText("snapshot-1", { exact: true })).toHaveCount(0);
      await view.screenshot("permission-revoked");
      await view.changeScope("permissions", [renewalPermission]);
      await expect(view.content).toBeVisible();
      // Finish the requests started by restoring permission before measuring
      // that logout prevents any subsequent scheduled synchronization.
      await expect(view.content.getByText("snapshot-1", { exact: true })).toBeVisible();
      await view.changeScope("logout");
      await expect(view.trigger).toHaveCount(0);
      const calls = mock.syncBodies.length;
      await page.clock.fastForward(60_000);
      expect(mock.syncBodies.length).toBe(calls);
    } finally { await context.close(); }
  });

  test("TC-81d: 初始无权限不查询，窄屏仍可查看并分页", async ({ browser }) => {
    const context = await browser.newContext({ baseURL: config.baseURL, viewport: { width: 1440, height: 960 } });
    try {
      const page = await context.newPage();
      const mock = await installAccessoryRenewalMock(page, []);
      const view = new AccessoryRenewalPage(page);
      await view.login(); await view.locale("zh-CN");
      await expect(view.trigger).toHaveCount(0);
      expect(mock.syncBodies).toHaveLength(0); expect(mock.listQueries).toHaveLength(0);
      await view.changeScope("permissions", [renewalPermission]);
      await expect(view.headerTrigger).toHaveCount(0);
      await view.open();
      await expect(view.trigger).toHaveText(/附属资源续费待办\s*\(21\)/);
      await view.narrow();
      await expect(view.content.getByText("snapshot-1", { exact: true })).toBeVisible();
      await expect(view.content.locator(".vxe-pager--total")).toContainText("21");
      await expect(view.content.locator(".vxe-pager--sizes")).toBeVisible();
      await view.screenshot("narrow-drawer");
      await view.nextPage();
      await expect(view.content.getByText("snapshot-21", { exact: true })).toBeVisible();
      expect(mock.listQueries.at(-1)).toEqual({ page: 2, pageSize: 20 });
    } finally { await context.close(); }
  });

  test("TC-81e: 隔离后端真实待办与重复通知同步保持同一事件", async ({ browser }) => {
    test.skip(!process.env.E2E_RENEWAL_LIVE, "Requires the real isolated backend and its seeded pending work item; no API or locale mocks");
    const expectedWorkItem = process.env.E2E_RENEWAL_EXPECTED_WORK_ITEM;
    expect(expectedWorkItem, "Provide the isolated fixture's pending work item number").toBeTruthy();
    const context = await browser.newContext({ baseURL: config.baseURL });
    try {
      const page = await context.newPage();
      const view = new AccessoryRenewalPage(page);
      const syncResponse = () => page.waitForResponse(response => response.request().method() === "POST" && new URL(response.url()).pathname.endsWith("/admin/server-accessory-renewal-notification-syncs"));
      const initialResponse = syncResponse();
      await view.login();
      const initial = await (await initialResponse).json();
      expect(initial.code).toBe(0);
      expect(initial.data.total).toBeGreaterThan(0);
      await view.locale("zh-CN");
      const listResponse = page.waitForResponse(response => new URL(response.url()).pathname.endsWith("/admin/server-accessory-renewal-work-items"));
      await view.open();
      const list = await (await listResponse).json();
      expect(list.code).toBe(0);
      const item = list.data.items.find((row: { workItemNo: string }) => row.workItemNo === expectedWorkItem);
      expect(item, "The real list must include the isolated pending work item").toBeTruthy();
      expect(["balance_payment_failed", "deletion_invoice_pending"]).toContain(item.reasonCode);
      await expect(view.content.getByText(item.subjectNo, { exact: true }).first()).toBeVisible();
      await expect(view.content.getByText(item.reasonCode === "deletion_invoice_pending" ? "账单关闭待处理" : "待付款", { exact: true }).first()).toBeVisible();
      const synced = syncResponse();
      await view.refresh();
      const first = await (await synced).json();
      expect(first.code).toBe(0); expect(first.data.eventNo).toBeTruthy();
      const repeated = syncResponse();
      await view.refresh();
      const second = await (await repeated).json();
      expect(second.code).toBe(0);
      expect(second.data).toEqual(first.data);
      await test.info().attach("live-renewal-evidence", {
        body: JSON.stringify({ workItem: item, firstSync: first.data, repeatedSync: second.data }, null, 2),
        contentType: "application/json",
      });
      await expect(view.syncError).toHaveCount(0); await expect(view.listError).toHaveCount(0);
      await view.screenshot("live-no-mock");
    } finally { await context.close(); }
  });

  test("TC-81f: 真实当前员工的续费通知显示在原站内消息列表且同语言同步不重复", async ({ browser }) => {
    test.skip(!process.env.E2E_RENEWAL_LIVE || !process.env.E2E_RENEWAL_INBOX, "Requires the final isolated backend with real current-user inbox delivery; no mocks");
    const invoiceNo = process.env.E2E_RENEWAL_EXPECTED_INVOICE;
    expect(invoiceNo, "Provide the isolated fixture's invoice number").toBeTruthy();
    const context = await browser.newContext({ baseURL: config.baseURL });
    try {
      const page = await context.newPage();
      const view = new AccessoryRenewalPage(page);
      await view.login(); await view.locale("zh-CN"); await view.open();
      await expect.poll(async () => {
        await view.refresh();
        return (await view.renewalMessages(invoiceNo!)).filter(message => message.title === "附属资源续费待处理").length;
      }, { timeout: 90_000, intervals: [1000, 2000, 5000] }).toBe(1);
      const before = await view.renewalMessages(invoiceNo!);
      await view.refresh();
      const messages = await view.renewalMessages(invoiceNo!);
      expect(messages.map(message => message.id).sort()).toEqual(before.map(message => message.id).sort());
      const localized = messages.filter(message => message.title === "附属资源续费待处理");
      expect(localized).toHaveLength(1);
      expect(localized[0].sourceId).toBeTruthy();
      expect(localized[0].content).toContain(invoiceNo);
      await test.info().attach("live-inbox-evidence", {
        body: JSON.stringify(messages, null, 2), contentType: "application/json",
      });
      await view.close(); await view.openNotifications();
      await expect(page.getByText(localized[0].title, { exact: true }).first()).toBeVisible();
      await view.screenshot("live-original-inbox");
    } finally { await context.close(); }
  });
  test("TC-81g: 实际部署的资源菜单可进入待办页且顶栏无业务按钮", async ({ browser }) => {
    test.skip(!process.env.E2E_RENEWAL_DEPLOYED, "Requires the current deployment; no menu, locale, or API mocks");
    const context = await browser.newContext({ baseURL: config.baseURL });
    try {
      const page = await context.newPage();
      const view = new AccessoryRenewalPage(page);
      await view.login();
      await expect(view.headerTrigger).toHaveCount(0);
      const response = page.waitForResponse(item => new URL(item.url()).pathname.endsWith("/admin/server-accessory-renewal-work-items"));
      await view.open();
      const result = await (await response).json();
      expect(result.code).toBe(0);
      expect(result.data.total).toBeGreaterThanOrEqual(0);
      await expect(view.trigger).toContainText("附属资源续费待办");
      await expect(view.content.locator(".vxe-pager--total")).toContainText(String(result.data.total));
      for (const item of result.data.items) await expect(view.content.getByText(item.subjectNo, {exact:true}).first()).toBeVisible();
      await expect(view.syncError).toHaveCount(0);
      await expect(view.listError).toHaveCount(0);
      await view.screenshot("deployed-navigation");
    } finally { await context.close(); }
  });

});

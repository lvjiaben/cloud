import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { ProviderTaskPage } from "../pages/ProviderTaskPage";
import { ServerLifecyclePage } from "../pages/ServerLifecyclePage";
import {
  installProviderTaskMock,
  providerTaskFixture,
} from "../support/provider-task-mock";
import {
  installServerLifecycleMock,
  lifecycleInvoiceNo,
  lifecycleOrderNo,
} from "../support/server-lifecycle-mock";

test.use({
  baseURL: process.env.E2E_CUSTOMER_BASE_URL?.trim() || "http://127.0.0.1:6677",
});

test.describe("TC-1 服务器付款、日常操作与异常回收生命周期", () => {
  test("TC-1a: 付款后先显示异步开通，完成后才显示已交付", async ({ page }) => {
    const state = await installServerLifecycleMock(page);
    const lifecycle = new ServerLifecyclePage(page);
    await lifecycle.login();

    await page.goto(`/account/billing/invoices/${lifecycleInvoiceNo}`);
    const finance = page.getByTestId("customer-finance-page");
    await expect(finance).toContainText("生命周期云服务器");
    await page.getByTestId("finance-balance-pay").click();
    await page.getByRole("button", { name: "确认支付" }).click();
    await expect.poll(() => state.balancePaymentBodies.length).toBe(1);
    expect(Object.keys(state.balancePaymentBodies[0] || {})).toEqual([
      "idempotencyKey",
    ]);
    await expect(finance).toContainText("已付清");

    await page.goto(`/account/orders/${lifecycleOrderNo}`);
    const commerce = page.getByTestId("customer-commerce-page");
    await expect(commerce).toContainText("开通中");
    await expect(commerce.getByText("已完成", { exact: true })).toHaveCount(0);
    await lifecycle.screenshot("server-lifecycle-provisioning-after-payment");

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(commerce).toContainText("已完成");
    expect(state.commerceReads).toBeGreaterThanOrEqual(2);
  });

  test("TC-1b: 客户关机后可以恢复开机，页面始终读取最新运行状态", async ({
    page,
  }) => {
    const state = await installServerLifecycleMock(page);
    const lifecycle = new ServerLifecyclePage(page);
    await lifecycle.login();
    await lifecycle.openServer();
    await expect(lifecycle.root).toContainText("运行中");

    await page.getByTestId("server-action-stop").click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "确认执行" })
      .click();
    await page
      .locator(".ant-modal-confirm")
      .filter({ hasText: "关机会中断服务器上的全部业务" })
      .getByRole("button", { name: "确认执行" })
      .click();
    await expect.poll(() => state.operationBodies.length).toBe(1);
    await expect(lifecycle.root).toContainText("已关机");

    await page.getByTestId("server-action-start").click();
    await expect.poll(() => state.operationBodies.length).toBe(2);
    expect(state.operationBodies.map((item) => item.action)).toEqual([
      "stop",
      "start",
    ]);
    await expect(lifecycle.root).toContainText("运行中");
  });

  test("TC-1c: 重装提交后锁住同一服务器的其他写操作", async ({ page }) => {
    const state = await installServerLifecycleMock(page);
    const lifecycle = new ServerLifecyclePage(page);
    await lifecycle.login();
    await lifecycle.openServer();

    await page.getByTestId("server-action-reinstall").click();
    await page.getByTestId("server-reinstall-image").click();
    await page.getByText("Linux / Ubuntu 24.04 LTS", { exact: false }).click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "确认执行" })
      .click();
    const reinstallConfirm = page
      .locator(".ant-modal-confirm")
      .filter({ hasText: "重装会覆盖当前系统" });
    await expect(reinstallConfirm).toBeVisible();
    await page.waitForTimeout(500);
    await reinstallConfirm.getByRole("button", { name: "确认执行" }).click();

    await expect.poll(() => state.operationBodies.length).toBe(1);
    await expect(lifecycle.root).toContainText("服务器操作处理中");
    await expect(page.getByTestId("server-action-reboot")).toBeDisabled();
    await expect(page.getByTestId("server-action-ip_change")).toBeDisabled();
    expect(state.operationBodies[0]).toMatchObject({
      action: "reinstall",
      parameters: { os: 101 },
      proofToken: "lifecycle-proof",
    });
    await lifecycle.screenshot("server-lifecycle-reinstall-mutex");
  });

  test("TC-1d: 客户确认免费换 IP 后看到新的安全地址投影", async ({ page }) => {
    const state = await installServerLifecycleMock(page);
    const lifecycle = new ServerLifecyclePage(page);
    await lifecycle.login();
    await lifecycle.openServer();
    await expect(lifecycle.root).toContainText("203.0.113.10");

    await page.getByTestId("server-action-ip_change").click();
    await page
      .getByRole("dialog")
      .getByRole("button", { name: "确认执行" })
      .click();
    await page
      .locator(".ant-modal-confirm")
      .filter({ hasText: "更换 IP 可能导致原地址立即失效" })
      .getByRole("button", { name: "确认执行" })
      .click();
    const quote = page
      .locator(".ant-modal-confirm")
      .filter({ hasText: "本次将更换 1 个 IP" });
    await expect(quote).toContainText("¥0.00");
    await quote.getByRole("button", { name: "确认更换 IP" }).click();

    await expect.poll(() => state.checkoutBodies.length).toBe(1);
    expect(state.quoteBodies).toHaveLength(1);
    expect(state.authorizationBodies.at(-1)).toMatchObject({
      action: "ip_change",
      parameters: { ip_num: 1 },
    });
    await expect(lifecycle.root).toContainText("198.51.100.28");
    await expect(lifecycle.root).not.toContainText("203.0.113.10");
  });

  test("TC-1e: 部分回收结果不明时停止自动化并由员工接管", async ({
    adminPage,
  }) => {
    const state = await installProviderTaskMock(adminPage);
    const tasks = new ProviderTaskPage(adminPage);
    await tasks.goto();
    await expect(tasks.row(providerTaskFixture.taskNo)).toContainText(
      "结果未知",
    );
    await tasks.openDetail(providerTaskFixture.taskNo);
    await tasks
      .evidence(providerTaskFixture.evidenceOne)
      .getByRole("checkbox")
      .check();
    await tasks.selectReason("供应商结果仍无法确定");
    await adminPage.getByTestId("provider-task-takeover-submit").click();
    const confirm = adminPage
      .locator(".ant-modal-confirm")
      .filter({ hasText: "确认关闭自动化并承担处理责任？" });
    await expect(confirm).toContainText("不会发送任何供应商请求");
    await confirm.getByRole("button", { name: "确认接管" }).click();

    await expect(tasks.detailDrawer).toContainText("人工接管结果");
    expect(state.takeoverRequests).toHaveLength(1);
    expect(state.takeoverRequests[0]?.body).toEqual({
      evidenceNos: [providerTaskFixture.evidenceOne],
      expectedVersion: 3,
      reasonCode: "provider_result_unresolved",
    });
    expect(state.takeoverRequests[0]?.body).not.toHaveProperty("retry");
    await tasks.screenshot("server-lifecycle-partial-reclaim-takeover");
  });
});

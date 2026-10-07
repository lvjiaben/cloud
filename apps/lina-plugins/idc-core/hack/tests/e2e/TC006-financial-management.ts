import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { FinancialManagementPage } from "../pages/FinancialManagementPage";
import {
  financialInvoiceNo,
  installFinancialManagementMock,
} from "../support/financial-management-mock";

test.describe("TC-6 后台财务工作台", () => {
  test("TC-6a: 大金额保持精确且账单目录使用完整数字分页", async ({
    adminPage,
  }) => {
    const state = await installFinancialManagementMock(adminPage);
    const finance = new FinancialManagementPage(adminPage);
    await finance.goto();

    await expect(finance.pageRoot).toContainText("CNY 9007199254740993.01");
    await expect(finance.invoiceRow(financialInvoiceNo)).toContainText(
      "CNY 9007199254740993.01",
    );
    await expect(finance.invoiceRow(financialInvoiceNo)).toContainText(
      "财务测试客户 · 13800138006 · #10006",
    );
    await expect(finance.invoiceRow(financialInvoiceNo)).not.toContainText(
      "CUS_FINANCE_0001",
    );
    await finance.pageRoot.getByRole("combobox", { name: "客户" }).click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("财务测试客户 · 13800138006 · #10006", { exact: true })
      .click();
    await finance.pageRoot.getByRole("button", { name: /搜\s*索/u }).click();
    await expect
      .poll(() => state.invoiceRequests.at(-1)?.searchParams.get("customerNo"))
      .toBe("CUS_FINANCE_0001");
    const requestCountBeforeNextPage = state.invoiceRequests.length;
    const pager = finance.pageRoot.locator(".vxe-pager:visible").first();
    await expect(pager.locator(".vxe-pager--total")).toContainText("11");
    await expect(pager.locator(".vxe-pager--sizes")).toBeVisible();
    await pager.locator(".vxe-pager--next-btn").click();
    await expect
      .poll(() => state.invoiceRequests.length)
      .toBeGreaterThan(requestCountBeforeNextPage);
    expect(state.invoiceRequests.at(-1)?.searchParams.get("cursor")).toBeNull();
    expect(state.invoiceRequests.at(-1)?.searchParams.get("page")).toBe("2");
    expect(state.invoiceRequests.at(-1)?.searchParams.get("limit")).toBe("10");
    await expect(finance.invoiceRow("INV_FINANCE_PAGE_10")).toBeVisible();
  });

  test("TC-6b: 人工入账先签发精确预览票据且提交体不能直接写资金状态", async ({
    adminPage,
  }) => {
    const state = await installFinancialManagementMock(adminPage);
    const finance = new FinancialManagementPage(adminPage);
    await finance.goto();
    await expect(finance.invoiceRow(financialInvoiceNo)).toBeVisible();

    await adminPage.getByTestId("finance-manual-receipt").click();
    const modal = adminPage.getByRole("dialog").filter({
      has: adminPage.getByTestId("finance-action-modal"),
    });
    await modal.getByRole("radio", { name: "客户余额" }).check();
    await adminPage.getByTestId("finance-customer-no").click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("财务测试客户 · 13800138006 · #10006", { exact: true })
      .click();
    await adminPage.getByTestId("finance-amount-minor").fill("8800");
    await adminPage
      .getByTestId("finance-proof-reference")
      .fill("private-proof-finance-001");
    await adminPage
      .getByTestId("finance-receiving-account")
      .fill("merchant-collection-cny");
    await adminPage
      .getByTestId("finance-evidence")
      .fill("verified offline transfer receipt");
    await modal.locator(".ant-btn-primary").click();

    await expect.poll(() => state.executeBodies.length).toBe(1);
    expect(state.proofBodies).toHaveLength(1);
    const preview = state.proofBodies[0] ?? {};
    const execution = state.executeBodies[0] ?? {};
    expect(execution.requestNo).toBe(preview.requestNo);
    expect(execution.evidenceDigest).toBe(preview.evidenceDigest);
    expect(execution.proofToken).toBe("opaque-finance-proof");
    expect(`${execution.evidenceDigest}`).toMatch(/^[0-9a-f]{64}$/);
    expect(JSON.stringify(execution)).not.toContain(
      "verified offline transfer receipt",
    );
    for (const forbidden of [
      "paid",
      "statusCode",
      "ledgerTransactionNo",
      "balanceMinor",
    ]) {
      expect(execution).not.toHaveProperty(forbidden);
      expect(preview).not.toHaveProperty(forbidden);
    }
  });

  test("TC-6c: 商家流水和调账详情只显示安全业务信息", async ({ adminPage }) => {
    const state = await installFinancialManagementMock(adminPage);
    const finance = new FinancialManagementPage(adminPage);
    await finance.goto();

    await finance.pageRoot.getByRole("tab", { name: "账本流水" }).click();
    await expect(finance.pageRoot).toContainText("平台账务");
    await expect(finance.pageRoot).toContainText("人工收款");
    await expect(finance.pageRoot).not.toContainText(/CUS_|manual_receipt/);
    await finance.pageRoot
      .getByRole("combobox", { name: "精确交易类型" })
      .click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("人工收款", { exact: true })
      .click();
    await finance.pageRoot.getByRole("button", { name: /搜\s*索/u }).click();
    await expect
      .poll(() =>
        state.transactionRequests
          .at(-1)
          ?.searchParams.get("transactionTypeCode"),
      )
      .toBe("manual_receipt");
    await adminPage
      .getByTestId("finance-transaction-detail-BLT_FINANCE_MERCHANT_0001")
      .click();
    const transactionDrawer = adminPage.getByRole("dialog", {
      name: "不可变交易流水",
    });
    await expect(transactionDrawer).not.toContainText(
      /1001|INTERNAL_SOURCE_0001|manual_receipt/,
    );
    await adminPage.keyboard.press("Escape");
    await expect(transactionDrawer).toBeHidden();

    await finance.pageRoot.getByRole("tab", { name: "调账复核" }).click();
    await adminPage
      .getByTestId("finance-adjustment-detail-BAR_FINANCE_0001")
      .click();
    const adjustmentDrawer = adminPage.getByRole("dialog", {
      name: "调账复核证据",
    });
    await expect(adjustmentDrawer).toContainText(
      "财务测试客户 · 13800138006 · #10006",
    );
    await expect(adjustmentDrawer).not.toContainText(
      /CUS_FINANCE_0001|1001|2002|aaaaaaaaaaaaaaaa|证据摘要|复核人/,
    );
  });

  test("TC-6d: 财务便捷操作保留脱敏确认、个人筛选和 XLSX 边界", async ({
    adminPage,
  }) => {
    const state = await installFinancialManagementMock(adminPage);
    const finance = new FinancialManagementPage(adminPage);
    await finance.goto();
    await expect(finance.invoiceRow(financialInvoiceNo)).toBeVisible();

    await adminPage
      .getByTestId("finance-resend-invoice")
      .fill(financialInvoiceNo);
    await adminPage.getByTestId("finance-resend-preview").click();
    await expect(finance.pageRoot).toContainText("a***@e***.com");
    await expect(finance.pageRoot).not.toContainText(
      "alice.billing@example.com",
    );
    await adminPage.getByTestId("finance-resend-confirm").click();
    await adminPage.locator("button.finance-resend-submit").click();
    await expect.poll(() => state.resendBodies.length).toBe(1);
    expect(state.resendBodies[0]).toHaveProperty("requestNo");
    expect(state.resendBodies[0]).not.toHaveProperty("email");

    await adminPage.getByTestId("finance-note-target").fill(financialInvoiceNo);
    await adminPage
      .getByTestId("finance-note-content")
      .fill("private finance note");
    await adminPage.getByTestId("finance-note-append").click();
    await expect.poll(() => state.noteBodies.length).toBe(1);
    expect(state.noteBodies[0]).not.toHaveProperty("customerVisible");
    await expect(finance.pageRoot).toContainText("private finance note");

    await adminPage.getByTestId("finance-filter-name").fill("我的待付款账单");
    await adminPage.getByTestId("finance-filter-save").click();
    await expect.poll(() => state.filterBodies.length).toBe(1);
    expect(state.filterBodies[0]).not.toHaveProperty("sharedWith");
    await finance.pageRoot.getByRole("button", { name: "应用" }).click();
    await expect(adminPage.getByText("筛选条件已应用")).toBeVisible();

    await expect(adminPage.getByText("批量续费", { exact: true })).toHaveCount(
      0,
    );

    await adminPage.getByTestId("finance-export-create").click();
    await expect.poll(() => state.exportBodies.length).toBe(1);
    expect(state.exportBodies[0]?.exportType).toBe("invoice");
    expect(JSON.stringify(state.exportBodies[0])).not.toContain("csv");
    await expect(finance.pageRoot).toContainText("XLSX 已生成");
    await expect(finance.pageRoot).toContainText("数据行数: 1");
  });

  test("TC-6e: 账单表格高度稳定且完整数字分页始终可见", async ({
    adminPage,
  }) => {
    await installFinancialManagementMock(adminPage);
    const finance = new FinancialManagementPage(adminPage);
    await finance.goto();

    const activeGrid = finance.pageRoot.locator(".vxe-grid:visible").first();
    const pager = finance.pageRoot.locator(".vxe-pager:visible").first();
    await expect(activeGrid).toBeVisible();
    await expect(pager.locator(".vxe-pager--prev-btn")).toBeVisible();
    await expect(pager.locator(".vxe-pager--next-btn")).toBeVisible();
    await expect(pager.locator(".vxe-pager--total")).toContainText("11");
    await expect(pager.locator(".vxe-pager--sizes")).toBeVisible();

    const initialBox = await activeGrid.boundingBox();
    expect(initialBox?.height).toBeGreaterThan(300);
    expect(initialBox?.height).toBeLessThanOrEqual(440);

    await finance.pageRoot.getByRole("tab", { name: "账本流水" }).click();
    await finance.pageRoot
      .getByRole("tab", { exact: true, name: "账单" })
      .click();
    const finalBox = await finance.pageRoot
      .locator(".vxe-grid:visible")
      .first()
      .boundingBox();
    expect(
      Math.abs((finalBox?.height ?? 0) - (initialBox?.height ?? 0)),
    ).toBeLessThanOrEqual(2);
    await expect(
      finance.pageRoot.locator(".vxe-pager:visible").first(),
    ).toBeVisible();
  });
});

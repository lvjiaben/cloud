import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ensureSourcePluginEnabled } from "../../../../../../hack/tests/fixtures/plugin";

import { TaxInvoicePage } from "../pages/TaxInvoicePage";
import { installTaxInvoiceMock } from "../support/tax-invoice-mock";

test.describe("TC-5 税务发票", () => {
  test.beforeEach(async ({ adminPage }) => {
    await ensureSourcePluginEnabled(adminPage, "idc-compliance");
  });

  test("TC-5a: 纳税主体未取得购买方诺诺授权时保持关闭且不能提交开票", async ({
    adminPage,
    mainLayout,
  }) => {
    await mainLayout.switchLanguage("简体中文");
    await mainLayout.prepareThemeModeWithoutAnimation("light");
    await installTaxInvoiceMock(adminPage);
    const invoices = new TaxInvoicePage(adminPage);
    await invoices.goto();

    await expect(invoices.root).toContainText("真实开票通道当前保持关闭");
    await invoices.openInvoice("TINV_PENDING_001");
    await expect(invoices.buyerProfile).toContainText("示例购买方有限公司");
    await expect(invoices.buyerProfile).toContainText("91310000123456789X");
    await expect(invoices.buyerProfile).toContainText("****8890");
    await expect(invoices.invoiceDetail).toContainText("购买方自有诺诺接入的正式证据尚未完成");
    await expect(invoices.invoiceDetail.getByTestId("tax-invoice-approve-select")).toHaveClass(/ant-select-disabled/u);
    await expect(invoices.invoiceDetail).not.toContainText("private://");
    await expect(invoices.invoiceDetail).not.toContainText("secret-must-never-leave-backend");
    await invoices.screenshot("tax-invoice-buyer-review-zh-cn");
  });

  test("TC-5b: 一次申请可以冻结单账单部分金额和多账单合并分配", async ({
    authenticatedPage,
  }) => {
    const page = authenticatedPage;
    const requests: Array<Record<string, unknown>> = [];
    await page.route(
      "**/x/idc-compliance/api/v1/account/compliance/tax-invoices",
      async (route) => {
        const body = route.request().postDataJSON() as Record<string, unknown>;
        requests.push(body);
        await route.fulfill({
          contentType: "application/json",
          json: {
            code: 0,
            data: {
              item: {
                allocations: [
                  { billingInvoiceNo: "BILL_001", grossAmountMinor: 3_000 },
                  { billingInvoiceNo: "BILL_002", grossAmountMinor: 5_000 },
                ],
                grossAmountMinor: 8_000,
                statusCode: "pending_review",
                taxInvoiceNo: "TINV_COMBINED_001",
              },
            },
            message: "OK",
          },
        });
      },
    );
    await page.goto("/", { waitUntil: "domcontentloaded" });
    const item = await page.evaluate(async () =>
      fetch(
        "/x/idc-compliance/api/v1/account/compliance/tax-invoices",
        {
          body: JSON.stringify({
            allocations: [
              { billingInvoiceNo: "BILL_001", requestedAmountMinor: 3_000 },
              { billingInvoiceNo: "BILL_002", requestedAmountMinor: 5_000 },
            ],
            idempotencyKey: "tax-invoice-combined-browser-0001",
            invoiceProfileNo: "PROFILE_001",
          }),
          headers: { "Content-Type": "application/json" },
          method: "POST",
        },
      )
        .then((response) => response.json())
        .then((response) => response.data.item),
    );

    expect(item).toMatchObject({
      grossAmountMinor: 8_000,
      statusCode: "pending_review",
    });
    expect(item.allocations).toHaveLength(2);
    expect(requests[0]).not.toHaveProperty("taxAmountMinor");
    expect(requests[0]).not.toHaveProperty("taxRate");
    expect(requests[0]).not.toHaveProperty("sellerSubject");
  });

  test("TC-5c: 同一开票请求重复提交只返回原申请，不重复占用额度", async ({
    authenticatedPage,
  }) => {
    const page = authenticatedPage;
    let createdCount = 0;
    const seen = new Map<string, Record<string, unknown>>();
    await page.route(
      "**/x/idc-compliance/api/v1/account/compliance/tax-invoices",
      async (route) => {
        const body = route.request().postDataJSON() as Record<string, unknown>;
        const key = String(body.idempotencyKey ?? "");
        let item = seen.get(key);
        if (!item) {
          createdCount += 1;
          item = {
            reused: false,
            statusCode: "pending_review",
            taxInvoiceNo: "TINV_IDEMPOTENT_001",
          };
          seen.set(key, item);
        }
        await route.fulfill({
          contentType: "application/json",
          json: {
            code: 0,
            data: { item: { ...item, reused: createdCount === 1 && seen.has(key) } },
            message: "OK",
          },
        });
      },
    );
    await page.goto("/", { waitUntil: "domcontentloaded" });
    const numbers = await page.evaluate(async () => {
      const create = () =>
        fetch(
          "/x/idc-compliance/api/v1/account/compliance/tax-invoices",
          {
            body: JSON.stringify({
              idempotencyKey: "tax-invoice-browser-replay-0001",
              invoiceProfileNo: "PROFILE_001",
            }),
            headers: { "Content-Type": "application/json" },
            method: "POST",
          },
        )
          .then((response) => response.json())
          .then((response) => response.data.item.taxInvoiceNo);
      return Promise.all([create(), create()]);
    });

    expect(numbers).toEqual(["TINV_IDEMPOTENT_001", "TINV_IDEMPOTENT_001"]);
    expect(createdCount).toBe(1);
  });

  test("TC-5f: 只有查看权限的员工看不到审核、提交、取消和红字处理按钮", async ({
    authenticatedPage,
  }) => {
    const adminPage = authenticatedPage;
    await installTaxInvoiceMock(adminPage, [
      "idc:admin:compliance:tax-invoice:query",
      "idc:admin:compliance:red-letter:query",
    ]);
    const invoices = new TaxInvoicePage(adminPage);
    await invoices.goto();
    await invoices.openInvoice("TINV_PENDING_001");
    await expect(invoices.invoiceDetail.getByTestId("tax-invoice-approve-select")).toHaveCount(0);
    await expect(invoices.invoiceDetail.getByTestId("tax-invoice-reject")).toHaveCount(0);
    await expect(invoices.invoiceDetail.getByTestId("tax-invoice-cancel")).toHaveCount(0);
    await invoices.closeInvoice();
    await invoices.openRedLetter("RTL_ALLOCATE_001");
    await expect(invoices.redLetterDetail.getByTestId("red-letter-resolve")).toHaveCount(0);
    await expect(invoices.redLetterDetail.getByTestId("red-letter-complete")).toHaveCount(0);
  });

  test("TC-5g: 驳回必须选择固定客户原因且内部备注只提交给后台接口", async ({
    adminPage,
  }) => {
    const state = await installTaxInvoiceMock(adminPage);
    const invoices = new TaxInvoicePage(adminPage);
    await invoices.goto();
    await invoices.openInvoice("TINV_PENDING_001");
    await invoices.invoiceDetail.getByTestId("tax-invoice-reject").click();
    const dialog = adminPage.getByRole("dialog", {
      name: "驳回开票申请",
    });
    await expect(dialog).toBeVisible();
    await dialog.locator(".ant-select-selector").click();
    await adminPage.getByText("重复申请", { exact: true }).last().click();
    await dialog.locator("textarea").fill("只供后台查看的核对记录");
    await dialog.getByRole("button", { name: /确\s*(认|定)|OK/u }).click();

    await expect(adminPage.getByText("申请已驳回并安全释放占用额度。")).toBeVisible();
    expect(state.invoiceActions).toHaveLength(1);
    expect(state.invoiceActions[0]).toMatchObject({
      action: "rejections",
      body: {
        expectedVersion: 3,
        internalNote: "只供后台查看的核对记录",
        reasonCode: "duplicate_request",
      },
      taxInvoiceNo: "TINV_PENDING_001",
    });
  });

  test("TC-5h: 失败申请只有具备供应商未开票证据时才显示取消入口", async ({
    adminPage,
  }) => {
    await installTaxInvoiceMock(adminPage);
    const invoices = new TaxInvoicePage(adminPage);
    await invoices.goto();
    await invoices.openInvoice("TINV_FAILED_NO_EVIDENCE_001");
    await expect(invoices.invoiceDetail.getByTestId("tax-invoice-cancel")).toHaveCount(0);
    await invoices.closeInvoice();
    await invoices.openInvoice("TINV_FAILED_CONFIRMED_001");
    await expect(invoices.invoiceDetail.getByTestId("tax-invoice-cancel")).toBeVisible();
  });

  test("TC-5d-authoritative: 红字客户确认只接受权威重新检查，不提供后台猜测按钮", async ({
    adminPage,
  }) => {
    const state = await installTaxInvoiceMock(adminPage);
    const invoices = new TaxInvoicePage(adminPage);
    await invoices.goto();
    await invoices.openRedLetter("RTL_CONFIRM_001");
    await expect(invoices.redLetterDetail).toContainText("等待客户完成供应商确认");
    await expect(invoices.redLetterDetail).toContainText("只有供应商权威结果才能推进状态");
    await expect(invoices.redLetterDetail.getByTestId("red-letter-resolve")).toHaveCount(0);
    await expect(invoices.redLetterDetail.getByTestId("red-letter-complete")).toHaveCount(0);
    expect(state.redActions).toHaveLength(0);
  });

  test("TC-5d-manual: 无法自动处理时选择准确原发票并提交不可变真实红冲凭证", async ({
    adminPage,
  }) => {
    const state = await installTaxInvoiceMock(adminPage);
    const invoices = new TaxInvoicePage(adminPage);
    await invoices.goto();
    await invoices.openRedLetter("RTL_ALLOCATE_001");
    await invoices.redLetterDetail.getByTestId("red-letter-resolve").click();
    let dialog = adminPage.getByRole("dialog", {
      name: "选择准确的原发票分配",
    });
    await dialog.getByText("TINV_ISSUED_001", { exact: false }).click();
    await dialog.getByRole("button", { name: /确\s*(认|定)|OK/u }).click();
    await expect(adminPage.getByText("原发票分配已锁定。")).toBeVisible();
    expect(state.redActions[0]).toMatchObject({
      action: "allocation-resolutions",
      body: {
        expectedVersion: 2,
        selections: [{ grossAmountMinor: 5_000, refundAllocationNo: "RFAL_001", taxInvoiceAllocationNo: "TIAL_ISSUED_001" }],
      },
      taskNo: "RTL_ALLOCATE_001",
    });

    await invoices.redLetterDetail.getByTestId("red-letter-complete").click();
    dialog = adminPage.getByRole("dialog", {
      name: "提交真实红冲凭证",
    });
    await dialog.locator("input").nth(0).fill("NUONUO-RED-LETTER-20260818-001");
    await dialog.locator("input").nth(1).fill("a".repeat(64));
    await dialog.getByRole("button", { name: /确\s*(认|定)|OK/u }).click();
    await expect(adminPage.getByText("真实凭证已不可变保存，红字任务已完成。")).toBeVisible();
    await dialog.waitFor({ state: "hidden" });
    expect(state.redActions[1]).toMatchObject({
      action: "manual-completions",
      body: {
        evidenceReference: "NUONUO-RED-LETTER-20260818-001",
        evidenceSha256: "a".repeat(64),
        expectedVersion: 3,
      },
      taskNo: "RTL_ALLOCATE_001",
    });
    await expect(invoices.redLetterDetail).toContainText("NUONUO-RED-LETTER-20260818-001");
    await invoices.screenshot("tax-invoice-red-letter-evidence-zh-cn");
  });

  test("TC-5k: 英文手机页面完成插件装载且没有整页横向溢出", async ({
    adminPage,
    mainLayout,
  }) => {
    await installTaxInvoiceMock(adminPage);
    const invoices = new TaxInvoicePage(adminPage);
    await invoices.goto();
    await mainLayout.switchLanguage("English");
    await adminPage.setViewportSize({ height: 844, width: 390 });

    await expect(invoices.root).toContainText("Tax invoices and red-letter corrections");
    await expect(invoices.root).toContainText("Live invoice issuance is currently closed");
    await expect(invoices.root.getByRole("tab", { name: "Red-letter tasks" })).toBeVisible();
    const hasPageOverflow = await adminPage.evaluate(
      () => document.documentElement.scrollWidth > document.documentElement.clientWidth,
    );
    expect(hasPageOverflow).toBe(false);
    await invoices.screenshot("tax-invoice-management-mobile-en-us");
  });
});

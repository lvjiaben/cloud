import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { CreditManagementPage } from "../pages/CreditManagementPage";
import { openStandaloneWorkspace } from "../support/standalone-workspace";
import {
  creditApplicationNo,
  installCreditManagementMock,
} from "../support/credit-management-mock";

test.describe("TC011 后台授信管理", () => {
  for (const [id, standalone] of [["h", false], ["i", true]] as const) {
    test(`TC011${id}: ${standalone ? "独立" : "正式"}信用拒绝清除授权数据和材料而非保留旧页`, async ({ authenticatedPage: adminPage }) => {
      const state = await installCreditManagementMock(adminPage, { standalone: true });
      state.rowCount = 23;
      const credit = new CreditManagementPage(adminPage);
      if (standalone) await openStandaloneWorkspace(adminPage, "/idc/finance");
      else await credit.goto();
      const grid = credit.panel.locator(".vxe-grid:visible");
      const rows = grid.locator(".vxe-table--main-wrapper .vxe-body--row");
      await expect(rows).toHaveCount(10);
      await credit.openDetail(creditApplicationNo);
      await expect(adminPage.getByTestId("credit-evidence-open")).toContainText("bank-statement.pdf");
      state.deniedDetail = 403;
      await adminPage.getByTestId("credit-evidence-open").click();
      await expect(adminPage.getByTestId("credit-application-detail-drawer")).toHaveCount(0);
      await expect(adminPage.getByTestId("credit-evidence-open")).toHaveCount(0);
      await expect(rows).toHaveCount(10);
      state.deniedDetail = 0;

      for (const [directory, label] of [["applications", "授信申请"], ["accounts", "信用账户"], ["statements", "信用账单"]] as const) {
        await credit.panel.getByRole(standalone ? "button" : "tab", { name: label, exact: true }).click();
        await expect(grid.locator(".vxe-pager--total")).toContainText("23");
        state.deniedDirectories[directory] = 403;
        await grid.locator(".vxe-pager--next-btn").click();
        await expect(rows).toHaveCount(0);
        await expect(grid.locator(".vxe-pager--total")).toContainText("0");
        delete state.deniedDirectories[directory];
        const alert = adminPage.locator(standalone ? ".finance-workspace .danger-alert" : '[role="alert"]').filter({ hasText: "授信数据加载失败。" });
        await alert.getByRole("button", { name: /重\s*试/u }).click();
        await expect(rows).toHaveCount(10);
        await expect(grid.locator(".vxe-pager--total")).toContainText("23");
      }
      state.deniedDirectories.statements = 401;
      await grid.locator(".vxe-pager--next-btn").click();
      for (const label of ["信用账单", "信用账户", "授信申请"]) {
        await credit.panel.getByRole(standalone ? "button" : "tab", { name: label, exact: true }).click();
        await expect(rows).toHaveCount(0);
        await expect(grid.locator(".vxe-pager--total")).toContainText("0");
      }
      await credit.screenshot(`credit-${standalone ? "standalone" : "formal"}-access-cleared`);
    });
  }
  for (const [id, standalone] of [["f", false], ["g", true]] as const) {
    test(`TC011${id}: ${standalone ? "独立" : "正式"}授信三表按准确总数查询全部数字页及页规模`, async ({ authenticatedPage: adminPage }) => {
      const pageErrors: string[] = [];
      adminPage.on("pageerror", (error) => pageErrors.push(error.message));
      const state = await installCreditManagementMock(adminPage, { standalone: true });
      state.rowCount = 123;
      const credit = new CreditManagementPage(adminPage);
      if (standalone) await openStandaloneWorkspace(adminPage, "/idc/finance");
      else await credit.goto();
      for (const [directory, label, number] of [
        ["applications", "授信申请", creditApplicationNo],
        ["accounts", "信用账户", "CRA_CREDIT_OVERDUE"],
        ["statements", "信用账单", "CRS_CREDIT_OVERDUE"],
      ] as const) {
        await credit.panel.getByRole(standalone ? "button" : "tab", { name: label, exact: true }).click();
        const grid = credit.panel.locator(".vxe-grid:visible");
        const rows = grid.locator(".vxe-table--main-wrapper .vxe-body--row");
        await expect(grid.locator(".vxe-pager--total")).toContainText("123");
        await expect(rows).toHaveCount(10);
        await grid.locator(".vxe-pager--end-btn").click();
        await expect(rows.first()).toContainText(`${number}_121`);
        await expect(rows).toHaveCount(3);
        await expect.poll(() => state.listRequests.filter((request) => request.directory === directory).at(-1)).toMatchObject({ page: 13, limit: 10 });
        await grid.locator(".vxe-pager--prev-btn").click();
        await expect(rows.first()).toContainText(`${number}_111`);
        state.failLists = true;
        await grid.locator(".vxe-pager--next-btn").click();
        await expect(adminPage.locator(standalone ? ".finance-workspace .danger-alert" : '[role="alert"]').filter({ hasText: "授信数据加载失败。" })).toBeVisible();
        await expect(credit.panel.locator('.vxe-pager--num-btn.is--active')).toHaveText("12");
        await expect(rows.first()).toContainText(`${number}_111`);
        state.failLists = false;
        await adminPage.locator(standalone ? ".finance-workspace .danger-alert" : '[role="alert"]').getByRole("button", { name: /重\s*试/u }).click();
        await expect(rows.first()).toContainText(`${number}_121`);
        // Change size directly from a later page: the business list resets to page one.
        for (const size of [20, 50, 100, 10]) {
          await grid.locator(".vxe-pager--sizes").click();
          await adminPage.locator(".vxe-select-option:visible").filter({ hasText: new RegExp(`^${size}(?:\\s|条|/)`) }).click({ timeout: 10_000 });
          await expect.poll(() => state.listRequests.filter((request) => request.directory === directory).at(-1)).toMatchObject({ page: 1, limit: size });
          await expect(rows.first()).toContainText(number);
          await expect(grid.locator(".vxe-pager--total")).toContainText("123");
        }
      }
      await credit.panel.locator(".vxe-grid:visible .vxe-pager").scrollIntoViewIfNeeded();
      await expect(credit.panel.locator(".vxe-grid:visible .vxe-pager")).toBeInViewport();
      await credit.screenshot(`credit-${standalone ? "standalone" : "formal"}-complete-pagination`);
      expect(pageErrors).toEqual([]);
    });
  }
  test("TC011e: 独立财务产物的三类授信表也使用真实 Vben", async ({
    authenticatedPage: adminPage,
  }) => {
    const state = await installCreditManagementMock(adminPage, {
      standalone: true,
    });
    state.rowCount = 12;
    await openStandaloneWorkspace(adminPage, "/idc/finance");
    const credit = new CreditManagementPage(adminPage);
    const grid = credit.panel.locator(".vxe-grid:visible");
    await expect(grid).toBeVisible();
    await expect(
      grid.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(10);
    await grid.locator(".vxe-pager--next-btn").click();
    await expect(
      grid.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(2);
    await credit.panel
      .getByRole("button", { name: "信用账户", exact: true })
      .click();
    await expect(grid).toContainText("已冻结");
    await credit.panel
      .getByRole("button", { name: "信用账单", exact: true })
      .click();
    await expect(grid).toContainText("已逾期");
    await credit.screenshot("credit-standalone-vben");

    await adminPage.route(
      "**/x-assets/vben-adapter/v0.1.0/isolated/runtime.js",
      (route) => route.abort("failed"),
      { times: 1 },
    );
    await adminPage.reload({ waitUntil: "domcontentloaded" });
    await expect(
      adminPage.locator(".finance-workspace .danger-alert"),
    ).toContainText("授信数据加载失败。");
    await expect(grid).toHaveCount(0);
    await credit.screenshot("credit-standalone-resource-failure");
    await adminPage
      .locator(".finance-workspace .danger-alert")
      .getByRole("button", { name: "重试", exact: true })
      .click();
    await expect(grid).toBeVisible();
    await expect(
      grid.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(10);
  });
  test("TC011a: 有权限员工无需额外认证和必填材料即可处理历史授信申请", async ({
    adminPage,
  }) => {
    const state = await installCreditManagementMock(adminPage);
    const credit = new CreditManagementPage(adminPage);
    await credit.goto();
    await expect(credit.applicationRow(creditApplicationNo)).toContainText(
      "1/2",
    );
    await credit.openReview(creditApplicationNo);
    const modal = adminPage.getByTestId("credit-review-modal");
    await expect(modal.getByText("通过", { exact: true })).toBeVisible();
    await expect(modal.getByText("拒绝", { exact: true })).toBeVisible();
    await credit.screenshot("credit-review-vben-form");
    await adminPage
      .getByRole("dialog")
      .getByRole("button", { name: /确\s*定/u })
      .click();
    await expect.poll(() => state.reviewBodies.length).toBe(1);
    expect(state.proofBodies).toHaveLength(0);
    expect(state.reviewBodies[0]).toMatchObject({
      decisionCode: "approved",
    });
    expect(state.reviewBodies[0]).not.toHaveProperty("proofToken");
    expect(String(state.reviewBodies[0]?.commentDigest)).toMatch(
      /^[0-9a-f]{64}$/u,
    );
    expect(JSON.stringify(state.reviewBodies[0])).not.toContain("银行流水");
    await expect(credit.applicationRow(creditApplicationNo)).toContainText(
      "已通过",
    );
    await modal.waitFor({ state: "hidden" });
    await credit.screenshot("admin-credit-approved-zh");
  });

  test("TC011b: 英文后台明确显示逾期账单和冻结账户", async ({
    adminPage,
    mainLayout,
  }) => {
    await installCreditManagementMock(adminPage);
    const credit = new CreditManagementPage(adminPage);
    await mainLayout.switchLanguage("English");
    await credit.goto();
    await adminPage.getByText("Credit accounts", { exact: true }).click();
    await expect(credit.panel).toContainText("Frozen");
    await expect(credit.panel).toContainText("CRA_CREDIT_OVERDUE");
    await adminPage.getByText("Credit statements", { exact: true }).click();
    await expect(credit.panel).toContainText("Overdue");
    await credit.screenshot("admin-credit-overdue-en");
  });

  test("TC011c: 授信三张业务表统一使用有界 Vben 表格和分页", async ({
    adminPage,
  }) => {
    await installCreditManagementMock(adminPage);
    const credit = new CreditManagementPage(adminPage);
    await credit.goto();

    for (const [tabName, header, expectedStatus] of [
      ["授信申请", "申请编号", "待审批"],
      ["信用账户", "账户编号", "已冻结"],
      ["信用账单", "账单编号", "已逾期"],
    ] as const) {
      await credit.panel.getByRole("tab", { name: tabName }).click();
      const grid = credit.panel.locator(".vxe-grid:visible").first();
      await expect(grid).toBeVisible();
      await expect(grid.locator(".vxe-table--header-wrapper")).toContainText(
        header,
      );
      await expect(
        grid.locator(".vxe-table--main-wrapper .vxe-table--body-wrapper"),
      ).toContainText(expectedStatus);
      await expect(grid.locator(".vxe-pager")).toBeVisible();
      const box = await grid.boundingBox();
      expect(box?.height).toBeLessThanOrEqual(380);
      const colors = await grid.evaluate((element) => {
        const style = getComputedStyle(element);
        return {
          body: style.getPropertyValue("--vxe-ui-layout-background-color"),
          header: style.getPropertyValue(
            "--vxe-ui-table-header-background-color",
          ),
          muted: `hsl(${style.getPropertyValue("--muted").trim()})`,
        };
      });
      expect(colors.body.trim()).toBe("hsl(0 0% 100%)");
      expect(colors.header.trim()).toBe(colors.muted);
      expect(colors.header.trim()).not.toBe(colors.body.trim());
    }
    await expect(credit.panel.locator("form:visible")).toHaveCount(0);
    await credit.screenshot("credit-vben-gray-header-grids");
  });

  test("TC011d: 授信表分页保持十条且刷新失败保留数据并可重试", async ({
    adminPage,
  }) => {
    const state = await installCreditManagementMock(adminPage);
    state.rowCount = 23;
    const credit = new CreditManagementPage(adminPage);
    await credit.goto();
    const grid = credit.panel.locator(".vxe-grid");
    const rows = grid.locator(".vxe-table--main-wrapper .vxe-body--row");
    await expect(rows).toHaveCount(10);
    await grid.locator(".vxe-pager--next-btn").click();
    await expect(rows.first()).toContainText(`${creditApplicationNo}_11`);
    await expect(rows).toHaveCount(10);
    state.failLists = true;
    await credit.panel.getByRole("button", { name: /重\s*试/u }).click();
    await expect(credit.panel.getByRole("alert")).toContainText(
      "授信数据加载失败。",
    );
    await expect(rows.first()).toContainText(`${creditApplicationNo}_11`);
    await credit.screenshot("credit-grid-refresh-failure");
    state.failLists = false;
    state.rowCount = 0;
    await credit.panel
      .getByRole("alert")
      .getByRole("button", { name: /重\s*试/u })
      .click();
    await expect(rows).toHaveCount(0);
    await expect(grid).toContainText("暂无记录");
  });
});

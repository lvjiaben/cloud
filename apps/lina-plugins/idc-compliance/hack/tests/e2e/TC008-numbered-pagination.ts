import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { ensureSourcePluginEnabled } from "../../../../../../hack/tests/fixtures/plugin";
import { EnterpriseVerificationPage } from "../pages/EnterpriseVerificationPage";
import { TaxInvoicePage } from "../pages/TaxInvoicePage";
import { NumberedTable } from "../pages/NumberedTable";
import { installEnterpriseVerificationMock } from "../support/enterprise-verification-mock";
import { installTaxInvoiceMock } from "../support/tax-invoice-mock";

async function assertThreePages(table: NumberedTable) {
  await expect(table.rows).toHaveCount(10);
  await expect(table.pager).toContainText("25");
  await table.assertBoundedGeometry();
  await table.selectPage(3);
  await expect(table.activePage()).toHaveText("3");
  await expect(table.rows).toHaveCount(5);
  await expect(table.pager).toContainText("25");
  await table.selectSize(20);
  await expect(table.activePage()).toHaveText("1");
  await expect(table.rows).toHaveCount(20);
  await expect(table.pager).toBeInViewport();
  await table.assertBoundedGeometry();
}

test.describe("TC-8 合规六类列表数字分页", () => {
  test.beforeEach(async ({ adminPage, mainLayout }) => {
    await ensureSourcePluginEnabled(adminPage, "idc-compliance");
    await mainLayout.switchLanguage("简体中文");
  });

  test("TC-8a: 企业认证准确总数、末页与切换每页条数", async ({ adminPage }) => {
    await installEnterpriseVerificationMock(adminPage, 25);
    const view = new EnterpriseVerificationPage(adminPage);
    await view.goto();
    await expect(view.root).toContainText("企业认证");
    await assertThreePages(new NumberedTable(adminPage, view.root));
    await view.screenshot("enterprise-numbered-pagination");
  });

  test("TC-8c: 税票和红冲各自保留页状态及准确总数", async ({ adminPage }) => {
    await installTaxInvoiceMock(adminPage, undefined, 25);
    const view = new TaxInvoicePage(adminPage);
    await view.goto();
    await expect(view.root).toContainText("发票");
    await assertThreePages(new NumberedTable(adminPage, view.root));
    await view.screenshot("tax-invoices-numbered-pagination");
    await view.openRedLetter("RTL_EVIDENCE_001");
    await view.closeRedLetter();
    await assertThreePages(new NumberedTable(adminPage, view.root));
    await view.screenshot("red-letter-numbered-pagination");
  });

  test("TC-8d: 暂时故障回到已确认页，权限失效清空数据和总数", async ({ adminPage }) => {
    await installEnterpriseVerificationMock(adminPage, 25);
    let failureStatus = 0;
    await adminPage.route(/\/x\/idc-compliance\/api\/v1\/admin\/compliance\/enterprise-verifications(?:\?.*)?$/u, async (route) => {
      if (!failureStatus) return route.fallback();
      await route.fulfill({ status: failureStatus, json: { code: failureStatus, data: null, message: failureStatus === 403 ? "无权访问企业认证记录" : "企业认证列表暂时不可用" } });
    });
    const view = new EnterpriseVerificationPage(adminPage);
    await view.goto();
    const table = new NumberedTable(adminPage, view.root);
    await expect(table.rows).toHaveCount(10);
    failureStatus = 503;
    const unavailable = adminPage.waitForResponse((response) => response.url().includes("/enterprise-verifications?") && response.status() === 503);
    await table.selectPage(2);
    await unavailable;
    await expect(table.activePage()).toHaveText("1");
    await expect(table.rows).toHaveCount(10);
    await expect(table.pager).toContainText("25");
    await view.screenshot("enterprise-pagination-temporary-failure");
    failureStatus = 403;
    const forbidden = adminPage.waitForResponse((response) => response.url().includes("/enterprise-verifications?") && response.status() === 403);
    await table.selectPage(2);
    await forbidden;
    await expect(table.rows).toHaveCount(0);
    await expect(table.pager.locator(".vxe-pager--total")).toContainText("0");
    await expect(view.root).not.toContainText("测******公司");
    await view.screenshot("enterprise-pagination-permission-cleared");
  });
});

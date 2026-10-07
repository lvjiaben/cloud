import { expect, test } from "../../../../../../hack/tests/fixtures/auth";

import { RefundManagementPage } from "../pages/RefundManagementPage";
import {
  installRefundManagementMock,
  managementRefundNo,
} from "../support/refund-management-mock";

test.describe("TC-10 后台在线退款审核", () => {
  test("TC-10a: 英文后台详情显示英文冻结明细和长期权益终止影响", async ({
    adminPage,
    mainLayout,
  }) => {
    const state = await installRefundManagementMock(adminPage);
    const refunds = new RefundManagementPage(adminPage);
    await mainLayout.switchLanguage("English");
    await refunds.goto();
    await expect(refunds.row(managementRefundNo)).toContainText("退款测试客户 · 138****0001 · #10086");
    await expect(refunds.row(managementRefundNo)).not.toContainText("CUS_REFUND_0001");
    await refunds.panel.getByRole("combobox", { name: "Customer" }).click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("退款测试客户 · 138****0001 · #10086", { exact: true })
      .click();
    await refunds.panel.getByRole("button", { name: /Query|Search/iu }).click();
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get("customerNo")).toBe("CUS_REFUND_0001");
    await expect(refunds.row(managementRefundNo)).toContainText(
      "Pending review",
    );
    await refunds.openDetail(managementRefundNo);
    const drawer = adminPage.getByTestId("refund-detail-drawer-content");
    await expect(drawer).toContainText("退款测试客户 · 138****0001 · #10086");
    await expect(drawer).not.toContainText("CUS_REFUND_0001");
    await expect(drawer).toContainText("Production cloud server");
    await expect(drawer).not.toContainText("生产云服务器");
    await expect(drawer).toContainText(
      "This long-lived entitlement must end after the refund completes",
    );
    await expect(drawer).not.toContainText("ENT_REFUND_0001");
    expect(state.detailRequests).toEqual([managementRefundNo]);
    await refunds.screenshot("admin-refund-detail-impact-en");
  });

  test("TC-10b: 人工拒绝只提交摘要证据并刷新权威状态", async ({
    adminPage,
  }) => {
    const state = await installRefundManagementMock(adminPage);
    const refunds = new RefundManagementPage(adminPage);
    await refunds.goto();
    await refunds.openReview(managementRefundNo);
    await refunds.chooseDecision("驳回");
    await refunds.fillEvidence("账单、服务状态和客户申请已经复核");
    await refunds.submitReview();
    await expect.poll(() => state.reviewBodies.length).toBe(1);
    expect(state.proofBodies).toHaveLength(0);
    expect(state.reviewBodies[0]).toMatchObject({
      decisionCode: "reject",
      expectedVersion: 1,
      proofToken: "",
    });
    expect(String(state.reviewBodies[0]?.evidenceDigest)).toMatch(
      /^[0-9a-f]{64}$/u,
    );
    expect(JSON.stringify(state.reviewBodies[0])).not.toContain(
      "账单、服务状态",
    );
    await expect(refunds.row(managementRefundNo)).toContainText("已驳回");
    await refunds.screenshot("admin-refund-rejected-zh");
  });

  test("TC-10c: 高额批准先取得精确重新登录凭证再提交同一审核", async ({
    adminPage,
  }) => {
    const state = await installRefundManagementMock(adminPage, {
      highAmount: true,
    });
    const refunds = new RefundManagementPage(adminPage);
    await refunds.goto();
    await refunds.openReview(managementRefundNo);
    const modal = adminPage.getByTestId("refund-review-modal-content");
    await expect(modal).toContainText("必须最近重新完成一次完整登录");
    await refunds.fillEvidence("高额退款的账单、服务影响和客户身份均已复核");
    await refunds.screenshot("admin-refund-high-amount-confirm-zh");
    await refunds.submitReview();
    await expect.poll(() => state.reviewBodies.length).toBe(1);
    expect(state.proofBodies).toHaveLength(1);
    expect(state.reviewBodies[0]?.requestNo).toBe(
      state.proofBodies[0]?.requestNo,
    );
    expect(state.reviewBodies[0]?.evidenceDigest).toBe(
      state.proofBodies[0]?.evidenceDigest,
    );
    expect(state.reviewBodies[0]?.proofToken).toBe("opaque-refund-proof");
    await expect(refunds.row(managementRefundNo)).toContainText("已批准");
  });
});

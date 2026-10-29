import { test, expect } from "../../../../../../hack/tests/fixtures/auth";

import { readFile } from "node:fs/promises";

import * as XLSX from "xlsx";

import { MarketingOperationsPage } from "../pages/MarketingOperationsPage";
import { installMarketingOperationsMock } from "../support/marketing-mock";

test.describe("TC-3 营销运营工作台", () => {
  test("TC-3j: 九张业务表使用准确总数、数字页码与每页条数选择", async ({
    adminPage,
  }) => {
    await installMarketingOperationsMock(adminPage, {
      enableInsights: true,
      enableFinancialActions: true,
      directoryRows: 25,
    });
    const requests: URL[] = [];
    adminPage.on("request", (request) => {
      if (
        request.method() === "GET" &&
        request.url().includes("/admin/marketing/")
      )
        requests.push(new URL(request.url()));
    });
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();
    const verify = async (
      scope: import("@host-tests/support/playwright").Locator,
      endpoint: string,
    ) => {
      const pager = scope.locator(".vxe-pager:visible");
      await expect(pager.locator(".vxe-pager--total")).toContainText("25");
      await expect(pager.locator(".vxe-pager--sizes")).toBeVisible();
      await expect(
        scope.locator(".vxe-table--main-wrapper .vxe-body--row"),
      ).toHaveCount(10);
      await pager
        .locator(".vxe-pager--num-btn")
        .filter({ hasText: /^3$/ })
        .click();
      await expect
        .poll(() =>
          requests
            .filter((url) => url.pathname.endsWith(endpoint))
            .at(-1)
            ?.searchParams.get("page"),
        )
        .toBe("3");
      await expect(
        scope.locator(".vxe-table--main-wrapper .vxe-body--row"),
      ).toHaveCount(5);
      await pager.locator(".vxe-pager--sizes").click();
      await adminPage
        .locator(".vxe-select-option:visible")
        .filter({ hasText: /^20(?:\s|条|\/)/ })
        .click();
      await expect
        .poll(() =>
          requests
            .filter((url) => url.pathname.endsWith(endpoint))
            .at(-1)
            ?.searchParams.get("limit"),
        )
        .toBe("20");
      await expect(pager.locator(".vxe-pager--num-btn.is--active")).toHaveText(
        "1",
      );
      await expect(
        scope.locator(".vxe-table--main-wrapper .vxe-body--row"),
      ).toHaveCount(20);
      expect(
        requests
          .filter((url) => url.pathname.endsWith(endpoint))
          .every((url) => !url.searchParams.has("cursor")),
      ).toBe(true);
    };
    await verify(
      adminPage.getByTestId("marketing-performance-report"),
      "/performance",
    );
    await verify(
      adminPage.getByTestId("marketing-abuse-cases"),
      "/abuse-cases",
    );
    await adminPage
      .getByTestId("marketing-performance-report")
      .getByRole("button", { name: "成本缺失" })
      .click();
    const missing = adminPage
      .getByRole("dialog")
      .filter({ hasText: "缺失成本的订单项目" });
    await verify(missing, "/missing-costs");
    await adminPage.keyboard.press("Escape");
    await expect(missing).toBeHidden();
    await marketing.openTab("活动与人群");
    await verify(adminPage.getByTestId("segment-directory"), "/segments");
    await verify(adminPage.getByTestId("campaign-directory"), "/campaigns");
    await marketing.openTab("权益与积分");
    for (const [testId, endpoint] of [
      ["asset-directory", "/assets"],
      ["points-directory", "/points/accounts"],
      ["value-grant-directory", "/grants"],
      ["value-grant-adjustment-queue", "/grant-adjustments"],
    ]) {
      await verify(adminPage.getByTestId(testId!), endpoint!);
    }
    await marketing.screenshot("marketing-nine-numbered-pagers");
  });
  test("TC-3a: 中文概览展示分群、活动、积分和明确币种权益", async ({
    adminPage,
  }) => {
    await installMarketingOperationsMock(adminPage);
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();

    await expect(
      marketing.pageRoot.getByText("营销运营", { exact: true }),
    ).toBeVisible();
    await expect(
      marketing.pageRoot
        .locator(".signal-card")
        .first()
        .getByText("12,480", { exact: true }),
    ).toBeVisible();
    await expect(
      marketing.pageRoot.getByText("928,400", { exact: true }),
    ).toBeVisible();
    await expect(marketing.currencyCard("CNY")).toContainText("8,800");
    await expect(marketing.currencyCard("USD")).toContainText("3,400");
    await expect(marketing.pageRoot).toContainText(
      "金额不会按汇率换算或跨币种相加",
    );
    await marketing.openTab("最近记录");
    await expect(
      marketing.campaignRow("campaign-first-recharge"),
    ).toContainText("首充赠送");
    await expect(marketing.grantRow("grant-first-recharge-0001")).toContainText(
      "CUS_marketing_customer_0001",
    );
    await expect(marketing.pageRoot).not.toContainText("plugin.idc-core");
    await marketing.screenshot("marketing-operations-zh-cn");
  });

  test("TC-3b: 手工刷新只重新读取同一个有界概览接口", async ({ adminPage }) => {
    const state = await installMarketingOperationsMock(adminPage);
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();
    expect(state.overviewRequests).toHaveLength(1);
    expect(state.directoryRequests).toHaveLength(0);

    await marketing.refresh();
    expect(state.overviewRequests).toHaveLength(2);
    expect(state.directoryRequests).toHaveLength(0);
    expect(
      state.overviewRequests.every((request) =>
        request.pathname.endsWith("/admin/marketing/overview"),
      ),
    ).toBe(true);
  });

  test("TC-3c: 英文界面完整翻译且保留逐币种边界", async ({
    adminPage,
    mainLayout,
  }) => {
    await installMarketingOperationsMock(adminPage);
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();
    await mainLayout.switchLanguage("English");

    await expect(
      marketing.pageRoot.getByText("Marketing Operations", { exact: true }),
    ).toBeVisible();
    await expect(marketing.pageRoot).toContainText(
      "Amounts are never converted or added across currencies",
    );
    await expect(marketing.currencyCard("CNY")).toContainText("8,800");
    await marketing.pageRoot
      .getByRole("tab", { name: "Recent activity" })
      .click();
    await expect(
      marketing.campaignRow("campaign-first-recharge"),
    ).toContainText("First recharge bonus");
    await expect(marketing.pageRoot).not.toContainText("plugin.idc-core");
    await marketing.screenshot("marketing-operations-en-us");
  });

  test("TC-3d: 分群与活动队列支持有界分页并打开类型化详情", async ({
    adminPage,
  }) => {
    const state = await installMarketingOperationsMock(adminPage);
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();
    await marketing.openTab("活动与人群");

    await expect(marketing.segmentRow("loyal_customers")).toContainText(
      "12,480",
    );
    await expect(
      marketing.managementCampaignRow("summer_cashback"),
    ).toContainText("145");
    await marketing.openSegmentDetail("loyal_customers");
    await expect(adminPage.getByText("registered_age_days")).toBeVisible();
    await expect(adminPage.getByText("snapshot-loyal-v3")).toBeVisible();
    await marketing.closeDrawer();

    await marketing.nextSegmentPage();
    await expect(marketing.segmentRow("dormant_customers")).toBeVisible();
    expect(
      state.directoryRequests.some(
        (request) =>
          request.searchParams.get("page") === "2" &&
          request.searchParams.get("limit") === "10" &&
          !request.searchParams.has("cursor"),
      ),
    ).toBe(true);

    await marketing.openCampaignDetail("summer_cashback");
    await expect(adminPage.getByText("benefit-summer-cashback")).toBeVisible();
    await expect(adminPage.getByText("1,000 CNY")).toBeVisible();
  });

  test("TC-3e: 优惠资产、积分和赠送权益保持币种与非货币边界", async ({
    adminPage,
  }) => {
    await installMarketingOperationsMock(adminPage);
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();
    await marketing.openTab("权益与积分");

    await expect(marketing.assetRow("asset-coupon-001")).toContainText(
      "500 CNY",
    );
    await marketing.openAssetDetail("asset-coupon-001");
    await expect(adminPage.getByText("asset-reservation-001")).toBeVisible();
    await marketing.closeDrawer();

    await expect(
      marketing.pointsRow("CUS_marketing_customer_0001"),
    ).toContainText("1,200");
    await marketing.openPointsDetail("CUS_marketing_customer_0001");
    await expect(adminPage.getByText("非货币积分账簿")).toBeVisible();
    await expect(adminPage.getByText("points-lot-001")).toBeVisible();
    await marketing.closeDrawer();

    await expect(
      marketing.valueGrantDirectoryRow("grant-ledger-001"),
    ).toContainText("8,800 CNY");
    await marketing.screenshotAssetDirectory("marketing-asset-directory");
    await marketing.openValueGrantDirectoryDetail("grant-ledger-001");
    await expect(adminPage.getByText("ledger-entry-001")).toBeVisible();
    await expect(adminPage.getByText("value-entry-001")).toBeVisible();
  });

  test("TC-3f: 营销资产与积分管理动作使用精确目标、原因码和幂等请求", async ({
    adminPage,
  }) => {
    const state = await installMarketingOperationsMock(adminPage);
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();
    await marketing.openTab("权益与积分");

    await adminPage.getByTestId("marketing-asset-issue").click();
    await adminPage
      .getByTestId("marketing-action-customer")
      .fill("CUS_marketing_customer_0003");
    await adminPage
      .getByTestId("marketing-action-campaign")
      .fill("campaign-coupon-2026");
    await adminPage
      .getByTestId("marketing-action-benefit")
      .fill("benefit-coupon-001");
    await adminPage
      .getByRole("dialog")
      .locator("button.ant-btn-primary")
      .last()
      .click();
    await expect.poll(() => state.actionRequests.length).toBe(1);
    expect(state.actionRequests[0]?.body).toMatchObject({
      benefitNo: "benefit-coupon-001",
      campaignNo: "campaign-coupon-2026",
      customerNo: "CUS_marketing_customer_0003",
      reasonCode: "customer_service",
    });
    expect(String(state.actionRequests[0]?.body.requestNo)).toMatch(
      /^marketing-asset_issue-/,
    );

    await marketing
      .assetRow("asset-revocable-002")
      .getByTestId("marketing-asset-revoke")
      .click();
    await adminPage
      .getByRole("dialog")
      .locator("button.ant-btn-primary")
      .last()
      .click();
    await expect.poll(() => state.actionRequests.length).toBe(2);
    expect(state.actionRequests[1]?.body).toMatchObject({
      expectedUpdatedAt: 1_786_339_261_000,
      reasonCode: "risk_control",
    });

    await adminPage.getByTestId("marketing-points-credit").click();
    await adminPage
      .getByTestId("marketing-action-customer")
      .fill("CUS_marketing_customer_0001");
    await adminPage.getByTestId("marketing-action-points").fill("100");
    await adminPage
      .getByRole("dialog")
      .locator("button.ant-btn-primary")
      .last()
      .click();
    await expect.poll(() => state.actionRequests.length).toBe(3);
    expect(state.actionRequests[2]?.body).toMatchObject({
      points: 100,
      reasonCode: "manual_correction",
    });

    await marketing.openPointsDetail("CUS_marketing_customer_0001");
    await adminPage.getByTestId("marketing-points-reverse").click();
    await adminPage.getByTestId("marketing-action-points").fill("500");
    await adminPage
      .getByRole("dialog")
      .locator("button.ant-btn-primary")
      .last()
      .click();
    await expect.poll(() => state.actionRequests.length).toBe(4);
    expect(state.actionRequests[3]?.body).toMatchObject({
      awardSourceReference: "order-001",
      awardSourceType: "order",
      points: 500,
      reasonCode: "refund_correction",
    });
  });

  test("TC-3g: 赠送权益有权限即可入账与冲正并允许处理历史待办", async ({
    adminPage,
  }) => {
    const state = await installMarketingOperationsMock(adminPage, {
      enableFinancialActions: true,
    });
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();
    await marketing.openTab("权益与积分");
    await expect(adminPage.getByText("有对应权限即可入账或冲正，系统保留完整流水。", {exact:true})).toBeVisible();

    await marketing
      .valueGrantDirectoryRow("grant-pending-002")
      .getByTestId("value-grant-post-request")
      .click();
    await adminPage
      .getByRole("dialog")
      .locator("button.ant-btn-primary")
      .last()
      .click();
    await expect.poll(() => state.actionRequests.length).toBe(1);
    expect(state.actionRequests[0]?.body).toMatchObject({
      reasonCode: "campaign_fulfillment",
    });
    expect(String(state.actionRequests[0]?.body.requestNo)).toMatch(
      /^marketing-grant_post-/,
    );

    await marketing
      .valueGrantDirectoryRow("grant-ledger-001")
      .getByTestId("value-grant-reverse-request")
      .click();
    await adminPage.getByTestId("marketing-action-amount-minor").fill("250");
    await adminPage
      .getByRole("dialog")
      .locator("button.ant-btn-primary")
      .last()
      .click();
    await expect.poll(() => state.actionRequests.length).toBe(2);
    expect(state.actionRequests[1]?.body).toMatchObject({
      amountMinor: "250",
      reasonCode: "refund_correction",
    });

    await marketing
      .adjustmentRow("bar-review-001")
      .getByTestId("value-grant-adjustment-review")
      .click();
    await expect(
      adminPage.getByTestId("marketing-action-decision"),
    ).toBeVisible();
    await adminPage
      .getByRole("dialog")
      .locator("button.ant-btn-primary")
      .last()
      .click();
    await expect.poll(() => state.actionRequests.length).toBe(3);
    expect(state.actionRequests[2]?.body).toEqual({
      decision: "approve",
      reasonCode: "manual_correction",
    });
  });

  test("TC-3h: 批量优惠码只在本次生成后下载 XLSX", async ({ adminPage }) => {
    const state = await installMarketingOperationsMock(adminPage, {
      enableCouponBatch: true,
    });
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();
    await marketing.openTab("权益与积分");

    await adminPage.getByTestId("marketing-coupon-batch-generate").click();
    const dialog = adminPage
      .getByRole("dialog")
      .filter({ hasText: "批量生成一次性优惠码" });
    await expect(dialog).toContainText(
      "完整优惠码只会在这次生成完成后下载一次",
    );
    await adminPage
      .getByTestId("marketing-coupon-batch-campaign")
      .fill("campaign-coupon-2026");
    await adminPage
      .getByTestId("marketing-coupon-batch-benefit")
      .fill("benefit-coupon-001");
    await adminPage.getByTestId("marketing-coupon-batch-count").fill("25");
    await adminPage.getByTestId("marketing-coupon-batch-length").fill("16");
    await marketing.screenshot("marketing-coupon-batch-dialog-zh");

    const downloadPromise = adminPage.waitForEvent("download");
    await dialog.locator("button.ant-btn-primary").last().click();
    const download = await downloadPromise;
    expect(download.suggestedFilename()).toMatch(/^coupon-codes-.+\.xlsx$/u);
    const downloadPath = await download.path();
    expect(downloadPath).toBeTruthy();
    const bytes = await readFile(downloadPath!);
    expect(bytes.subarray(0, 2).toString("utf8")).toBe("PK");
    const workbook = XLSX.read(bytes, { type: "buffer" });
    const rows = XLSX.utils.sheet_to_json<string[]>(
      workbook.Sheets[workbook.SheetNames[0]!]!,
      { header: 1 },
    );
    expect(rows).toEqual([
      ["Batch number", "Coupon code"],
      ["COUPON_BATCH_0001", "SUMMER2026CODE01"],
      ["COUPON_BATCH_0001", "SUMMER2026CODE02"],
    ]);
    await expect.poll(() => state.couponBatchRequests.length).toBe(1);
    expect(state.couponBatchRequests[0]).toMatchObject({
      benefitNo: "benefit-coupon-001",
      campaignNo: "campaign-coupon-2026",
      codeLength: 16,
      count: 25,
    });
    expect(String(state.couponBatchRequests[0]?.requestNo)).toMatch(
      /^coupon-batch-/u,
    );
    await expect(dialog).toHaveCount(0);
  });

  test("TC-3i: 活动效果、防刷复核和历史成本补录保持独立权限与二次确认", async ({
    adminPage,
  }) => {
    const state = await installMarketingOperationsMock(adminPage, {
      enableInsights: true,
    });
    const marketing = new MarketingOperationsPage(adminPage);
    await marketing.goto();

    const performance = adminPage.getByTestId("marketing-performance-report");
    await expect(performance).toContainText("夏季活动");
    await expect(performance).toContainText("100000 CNY");
    await expect(performance).toContainText("非货币");

    const abuse = adminPage.getByTestId("marketing-abuse-cases");
    await expect(abuse).toContainText("MKT_CASE_0001");
    await expect(abuse).toContainText("退款套利");
    await expect(abuse).toContainText("关联账号数");

    await performance.getByRole("button", { name: "成本缺失" }).click();
    const missingCosts = adminPage
      .getByRole("dialog")
      .filter({ hasText: "缺失成本的订单项目" });
    await expect(missingCosts).toContainText("ORDER_0001");
    await expect(missingCosts).toContainText("POOL_BINDING_0001");
    await missingCosts.getByRole("button", { name: "补录成本" }).click();
    const correction = adminPage
      .getByRole("dialog")
      .filter({ hasText: "补录历史订单成本" });
    await correction.locator("input").fill("2500");
    await correction.locator("textarea").fill("历史采购成本核对完成");
    await correction.locator("button.ant-btn-primary").last().click();
    const costConfirm = adminPage
      .getByRole("dialog")
      .filter({ hasText: "确认补录历史成本" });
    await costConfirm.locator("button.ant-btn-primary").click();
    await expect.poll(() => state.insightMutations.length).toBe(1);
    expect(state.insightMutations[0]?.body).toMatchObject({
      expectedVersion: 1,
      reason: "历史采购成本核对完成",
      unitCostMinor: "2500",
    });
    await expect(costConfirm).toBeHidden();
    await expect(correction).toBeHidden();
    await adminPage.keyboard.press("Escape");
    await expect(missingCosts).toBeHidden();

    await abuse.getByRole("button", { name: "确认违规" }).click();
    const review = adminPage
      .getByRole("dialog")
      .filter({ hasText: "处理营销防刷案件" });
    await review.locator("textarea").fill("证据核对完成并确认违规");
    await review.locator("button.ant-btn-primary").last().click();
    const reviewConfirm = adminPage
      .getByRole("dialog")
      .filter({ hasText: "再次确认处理结果" });
    await reviewConfirm.locator("button.ant-btn-primary").click();
    await expect.poll(() => state.insightMutations.length).toBe(2);
    expect(state.insightMutations[1]?.body).toEqual({
      decision: "confirmed_abuse",
      expectedVersion: 1,
      reason: "证据核对完成并确认违规",
    });
  });
});

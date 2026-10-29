import { expect, test } from "../support/catalog-auth";

import { CommerceOrdersPage } from "../pages/CommerceOrdersPage";
import {
  commerceAdminOrderNo,
  installCommerceOrderMock,
} from "../support/commerce-order-mock";

test.describe("TC-4 后台订单处置", () => {
  test("TC-4g: 完整分页超过一百条并在翻页失败时保留已确认页", async ({ adminPage }) => {
    const state = await installCommerceOrderMock(adminPage, { listSize: 125 });
    const orders = new CommerceOrdersPage(adminPage);
    await orders.goto();
    const pager = orders.pageRoot.locator(".vxe-pager");
    const activePage = pager.locator(".vxe-pager--num-btn.is--active");
    await expect(pager).toContainText("共 125 条记录");
    await orders.lastPage();
    await expect(activePage).toHaveText("13");
    await expect(orders.pageRoot.locator(".vxe-table--main-wrapper .vxe-body--row")).toHaveCount(5);
    await orders.showPagination();
    await orders.screenshot("commerce-orders-numbered-last-page");

    await orders.pageSize(100);
    await expect(activePage).toHaveText("1");
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get("limit")).toBe("100");
    await orders.pageSize(20);
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get("limit")).toBe("20");
    await orders.lastPage();
    await expect(activePage).toHaveText("7");

    state.failNextList = true;
    await orders.previousPage();
    await expect(orders.pageRoot.locator(".ant-alert")).toBeVisible();
    await expect(activePage).toHaveText("7");
    await expect(orders.pageRoot.locator(".vxe-table--main-wrapper .vxe-body--row")).toHaveCount(5);
    await orders.screenshot("commerce-orders-failed-page-preserved");
    await orders.previousPage();
    await expect(activePage).toHaveText("6");
    await expect(orders.pageRoot.locator(".vxe-table--main-wrapper .vxe-body--row")).toHaveCount(20);
    await expect(pager).toContainText("共 125 条记录");
    await orders.search();
    await expect(activePage).toHaveText("1");
    state.denyNextList = true;
    await orders.nextPage();
    await expect(orders.pageRoot.locator(".ant-alert")).toBeVisible();
    await expect(orders.pageRoot.locator(".vxe-table--main-wrapper .vxe-body--row")).toHaveCount(0);
    await expect(pager).toContainText("共 0 条记录");
    await orders.screenshot("commerce-orders-permission-revoked");
  });

  test("TC-4a: 有界列表保留精确大金额并使用完整数字分页", async ({ adminPage }) => {
    const state = await installCommerceOrderMock(adminPage);
    const orders = new CommerceOrdersPage(adminPage);
    await orders.goto();

    await expect(orders.pageRoot.locator(".page-heading")).toHaveCount(0);
    await expect(orders.row(commerceAdminOrderNo)).toBeVisible();
    await orders.scrollTableToEnd();
    await expect(orders.row(commerceAdminOrderNo)).toContainText(
      "CNY 9007199254740993.01",
    );
    await orders.scrollTableToStart();
    await expect(orders.pageRoot).not.toContainText("集中查看订单、账单、库存预留、商品交付与审核状态。");
    await expect(orders.pageRoot.getByRole("combobox", { name: "客户搜索", exact: true })).toBeVisible();
    await expect(orders.pageRoot.locator(".ant-select-loading")).toHaveCount(0);
    await expect(orders.row(commerceAdminOrderNo)).toContainText("订单测试客户 · 138****8008 · #10008");
    await expect(orders.row(commerceAdminOrderNo)).not.toContainText("CUS_admin_customer_0001");
    await orders.pageRoot.getByRole("combobox", { name: "客户" }).click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("订单测试客户 · 138****8008 · #10008", { exact: true })
      .click();
    await orders.pageRoot.getByRole("button", { name: /搜\s*索/u }).click();
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get("customerNo")).toBe("CUS_admin_customer_0001");
    const requestCountBeforeNextPage = state.listRequests.length;
    await orders.nextPage();
    await expect.poll(() => state.listRequests.length).toBeGreaterThan(requestCountBeforeNextPage);
    expect(state.listRequests.at(-1)?.searchParams.get("page")).toBe(
      "2",
    );
    expect(state.listRequests.at(-1)?.searchParams.get("limit")).toBe("10");
    await expect(orders.pageRoot.locator(".vxe-pager")).toContainText("共 12 条记录");
    await expect(orders.pageRoot.locator(".vxe-pager--num-btn.is--active")).toHaveText("2");
  });

  test("TC-4b: 聚合详情展示通用商品交付、持续服务和安全双语状态", async ({
    adminPage,
    mainLayout,
  }) => {
    const state = await installCommerceOrderMock(adminPage,{deliveryStatus:"manual_processing"});
    const orders = new CommerceOrdersPage(adminPage);
    await orders.goto();
    await orders.openDetail(commerceAdminOrderNo);

    const drawer = adminPage.getByRole("dialog");
    await expect(drawer).toContainText("订单测试客户 · 138****8008 · #10008");
    await expect(drawer).toContainText("延长预付款期限");
    await expect(drawer).not.toContainText("extended_prepay");
    await expect(drawer).not.toContainText("CUS_admin_customer_0001");
    await expect(drawer).toContainText("商品交付");
    await expect(drawer).toContainText("待人工处理");
    await expect(drawer).toContainText("通配符 SSL 证书");
    await expect(drawer).toContainText("商务手机");
    await expect(drawer).toContainText("包含持续服务");
    await expect(drawer).toContainText("一次性交付");
    await expect(drawer).toContainText("持续服务");
    await expect(drawer).toContainText("服务中");
    await expect(drawer).toContainText("审核记录");
    await expect(drawer).toContainText("风控审核");
    await expect(drawer).toContainText("待处理");
    await expect(drawer).toContainText("状态暂不可用");
    await expect(drawer).toContainText("审核类型暂不可用");
    await expect(drawer).toContainText("暂未接入完整审核材料，当前不能批准或驳回；请联系风控负责人核实。");
    await expect(drawer.getByRole("button", { name: "批准风控", exact: true })).toBeDisabled();
    await expect(drawer).not.toContainText(
      /held|future_inventory_status|future_review_status|future_review_type|unitSequence|serverQuantity|fulfillmentNo|FUL_ADMIN/iu,
    );
    expect(state.detailRequests).toHaveLength(1);
    await expect.poll(async () => drawer.evaluate(element => element.getBoundingClientRect().right <= window.innerWidth + 1)).toBe(true);
    await orders.screenshot("idc-admin-commerce-order-detail");

    await adminPage.keyboard.press("Escape");
    await expect(drawer).toBeHidden();

    await mainLayout.switchLanguage("English");
    await orders.openDetail(commerceAdminOrderNo);
    const englishDrawer = adminPage.getByRole("dialog");
    await expect(englishDrawer).toContainText("Product delivery");
    await expect(englishDrawer).toContainText("Needs staff attention");
    await expect(englishDrawer).toContainText("Extended prepayment deadline");
    await expect(englishDrawer).not.toContainText("extended_prepay");
    await expect(englishDrawer).toContainText("Wildcard SSL certificate");
    await expect(englishDrawer).toContainText("Business phone");
    await expect(englishDrawer).toContainText("Ongoing service included");
    await expect(englishDrawer).toContainText("One-time delivery");
    await expect(englishDrawer).toContainText("Ongoing services");
    await expect(englishDrawer).toContainText("Active");
    await expect(englishDrawer).toContainText("Risk review");
    await expect(englishDrawer).toContainText("Pending");
    await expect(englishDrawer).toContainText("Status unavailable");
    await expect(englishDrawer).toContainText("Review type unavailable");
    await expect(englishDrawer).toContainText("Complete review evidence is not connected yet. Approval and rejection are unavailable; contact the risk review owner.");
    await expect(englishDrawer).not.toContainText(
      /held|future_inventory_status|future_review_status|future_review_type|unitSequence|serverQuantity|fulfillmentNo|FUL_ADMIN/iu,
    );
  });

  test("TC-4c: 企业订单无合同入口或后台签约依赖，正常展示交付", async ({ adminPage }) => {
    const state = await installCommerceOrderMock(adminPage);
    const orders = new CommerceOrdersPage(adminPage);
    await orders.goto();
    await orders.openDetail(commerceAdminOrderNo);
    const drawer = adminPage.getByRole("dialog");
    await expect(drawer).toContainText("订单测试客户 · 138****8008 · #10008");
    await expect(drawer).toContainText("通配符 SSL 证书");
    await expect(drawer).toContainText("商品交付");
    await expect(orders.optionalContractSection).toHaveCount(0);
    await expect(drawer.getByTestId("contract-policy-editor")).toHaveCount(0);
    expect(state.retiredContractRequests).toHaveLength(0);
    expect(state.detailRequests).toHaveLength(1);
    await orders.screenshot("order-contract-retired");
  });

  test("TC-4e: 缺少明细的订单仍可查看且不依赖已删除合同接口", async ({ adminPage }) => {
    const state = await installCommerceOrderMock(adminPage, { emptyItems: true });
    const orders = new CommerceOrdersPage(adminPage);
    await orders.goto();
    await orders.openDetail(commerceAdminOrderNo);

    const drawer = adminPage.getByRole("dialog");
    await expect(drawer).toContainText("订单测试客户 · 138****8008 · #10008");
    await expect(orders.optionalContractSection).toHaveCount(0);
    expect(state.detailRequests).toHaveLength(1);
    expect(state.retiredContractRequests).toHaveLength(0);
  });

  test("TC-4f: 桌面和窄屏将横向滚动限制在订单表内且保持多行与分页", async ({ adminPage }) => {
    const state = await installCommerceOrderMock(adminPage, { listSize: 12 });
    const orders = new CommerceOrdersPage(adminPage);
    await orders.resize(1366);
    await orders.goto();
    await expect(orders.row(commerceAdminOrderNo)).toBeVisible();

    for (const width of [1366, 1920, 768]) {
      await orders.resize(width);
      await expect.poll(async () => {
        const metrics = await orders.layoutMetrics();
        return metrics.gridRight <= metrics.rootRight + 1 && metrics.rootRight <= metrics.viewport + 1;
      }).toBe(true);
      const metrics = await orders.layoutMetrics();
      expect(metrics.gridWidth).toBeGreaterThan(200);
      expect(metrics.tableWidth).toBeGreaterThan(metrics.tableViewportWidth);
      expect(metrics.actionRight).toBeLessThanOrEqual(metrics.rootRight + 1);
      expect(metrics.pagerRight).toBeLessThanOrEqual(metrics.rootRight + 1);
      expect(metrics.bodyHeight).toBeGreaterThan(120);
      expect(metrics.bodyHeight).toBeLessThan(metrics.viewportHeight);
      expect(Math.abs(metrics.rootBottom - metrics.gridBottom)).toBeLessThanOrEqual(16);
      expect(metrics.pagerBottom).toBeLessThanOrEqual(metrics.viewportHeight);
      expect(metrics.visibleRows).toBeGreaterThan(2);
      await orders.screenshot(`commerce-orders-contained-${width}`);
      await orders.scrollTableToEnd();
      await expect(orders.fixedDetailAction).toBeInViewport();
    }

    await orders.showPagination();
    await expect(orders.nextPageAction).toBeInViewport();
    await orders.nextPage();
    await expect.poll(() => state.listRequests.at(-1)?.searchParams.get("page")).toBe("2");
    await expect(orders.previousPageAction).toBeEnabled();
  });

  test("TC-4h: 订单表随窗口高度伸展，分页留在屏幕内", async ({ adminPage }) => {
    await installCommerceOrderMock(adminPage, { listSize: 12 });
    const orders = new CommerceOrdersPage(adminPage);
    await orders.resize(1440, 720);
    await orders.goto();
    await expect(orders.row(commerceAdminOrderNo)).toBeVisible();
    let previousHeight = 0;
    for (const height of [720, 900, 1200]) {
      await orders.resize(1440, height);
      await expect.poll(async () => {
        const metrics = await orders.layoutMetrics();
        return Math.abs(metrics.rootBottom - metrics.gridBottom) <= 16 && metrics.pagerBottom <= height;
      }).toBe(true);
      const metrics = await orders.layoutMetrics();
      expect(metrics.bodyHeight).toBeGreaterThan(previousHeight + 100);
      previousHeight = metrics.bodyHeight;
      await orders.screenshot(`commerce-orders-full-height-${height}`);
    }
  });
});

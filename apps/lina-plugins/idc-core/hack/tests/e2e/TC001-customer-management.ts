import { customerTest as test } from "../support/customer-admin-session";
import { expect } from "../../../../../../hack/tests/fixtures/auth";

import { CustomerManagementPage } from "../pages/CustomerManagementPage";
import { installCustomerManagementMock } from "../support/customer-mock";

test.describe("TC-1 客户管理列表与客户详情", () => {

  test("TC-1a: 中文列表展示真实译文和有界客户投影", async ({ adminPage }) => {
    await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();

    await expect(
      customers.pageRoot
        .locator(".vxe-table--main-wrapper .vxe-table--header-wrapper")
        .getByText("客户编号", { exact: true }),
    ).toBeVisible();
    await expect(
      customers.pageRoot
        .locator(".vxe-table--main-wrapper .vxe-table--header-wrapper")
        .getByText("实名状态", { exact: true }),
    ).toBeVisible();
    await expect(
      customers.customerRow("CUS_0123456789abcdef0123456789abcdef"),
    ).toContainText("上海云舟科技");
    await expect(
      customers.customerRow("CUS_0123456789abcdef0123456789abcdef"),
    ).toContainText("已实名");
    await expect(customers.pageRoot).not.toContainText("plugin.idc-core");
    await customers.screenshot("customer-management-first-page");
  });

  test("TC-1b: 名称前缀按原值提交并重置数字页", async ({ adminPage }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.nextPage();
    await expect(customers.pageIndicator).toHaveText("2");

    await customers.searchNamePrefix("上海%云_");
    await expect(customers.pageIndicator).toHaveText("1");
    const request = state.listRequests.at(-1);
    expect(request?.searchParams.get("namePrefix")).toBe("上海%云_");
    expect(request?.searchParams.get("cursor")).toBeNull();
    await customers.screenshot("customer-management-filtered");
  });

  test("TC-1c: 官方数字分页使用准确总数与服务端页码", async ({ adminPage }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();

    await expect(customers.previousButton).toHaveClass(/is--disabled/u);
    await customers.nextPage();
    await expect(customers.pageIndicator).toHaveText("2");
    expect(state.listRequests.at(-1)?.searchParams.get("page")).toBe("2");
    expect(state.listRequests.at(-1)?.searchParams.get("limit")).toBe("10");
    await expect(customers.pageRoot.locator(".vxe-pager--total")).toContainText("20");
    await expect(customers.pageRoot.locator(".vxe-pager--sizes")).toBeVisible();
    await expect(
      customers.customerRow("CUS_00000000000000000000000000000009"),
    ).toContainText("分页客户 9");

    await customers.previousPage();
    await expect(customers.pageIndicator).toHaveText("1");
    expect(state.listRequests.at(-1)?.searchParams.get("cursor")).toBeNull();
  });

  test("TC-1d: 客户详情 展示分群与运营计数且不暴露法定身份", async ({
    adminPage,
  }) => {
    await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();
    await customers.openOverview("CUS_0123456789abcdef0123456789abcdef");

    const drawer = customers.overviewDrawer;
    await expect(
      drawer.getByText("上海云舟科技", { exact: true }),
    ).toBeVisible();
    await expect(drawer).toContainText("备注");
    await expect(drawer).toContainText("重点维护");
    await expect(drawer).toContainText("优先客户");
    await expect(drawer).not.toContainText("身份证");
    await expect(drawer).not.toContainText("统一社会信用代码");
    await customers.screenshot("customer-management-overview");
  });

  test("TC-1e: 分组候选使用有界接口并提交业务编号筛选", async ({
    adminPage,
  }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();

    await customers.selectSegment("group", "重点维护");
    const groupRequest = state.segmentRequests.find(
      (request) => request.searchParams.get("kind") === "group",
    );
    expect(groupRequest?.searchParams.get("kind")).toBe("group");
    expect(groupRequest?.searchParams.get("limit")).toBe("25");
    expect(state.listRequests.at(-1)?.searchParams.get("groupNo")).toBe(
      "GRP_0123456789abcdef0123456789abcdef",
    );
    await expect(
      customers.customerRow("CUS_0123456789abcdef0123456789abcdef"),
    ).toContainText("上海云舟科技");
  });

  test("TC-1f: 批量标签只提交所选客户并隐藏幂等键", async ({ adminPage }) => {
    const state = await installCustomerManagementMock(adminPage);
    const customers = new CustomerManagementPage(adminPage);
    await customers.goto();

    await customers.selectCustomer("CUS_0123456789abcdef0123456789abcdef");
    await customers.selectCustomer("CUS_abcdef0123456789abcdef0123456789");
    await customers.openTagBatch();

    const dialog = adminPage.getByRole("dialog");
    await expect(
      dialog.getByTestId("customer-tag-batch-summary"),
    ).toContainText("2");
    await dialog.getByTestId("customer-tag-batch-tags").click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("优先客户", { exact: true })
      .click();
    await expect(dialog.getByTestId("customer-tag-batch-reason")).toHaveCount(0);
    const tagInput = dialog.getByTestId("customer-tag-batch-tags").locator("input");
    await tagInput.fill("新输入标签");await tagInput.press("Enter");
    await dialog.getByTestId("customer-tag-batch-summary").click();
    await customers.screenshot("customer-inline-batch-tags");
    await dialog.getByRole("button", { name: /确\s*认/u }).click();

    await expect.poll(() => state.batchRequests.length).toBe(1);
    const request = state.batchRequests[0];
    expect(request?.action).toBe("assign");
    expect(request?.customerNos).toEqual([
      "CUS_0123456789abcdef0123456789abcdef",
      "CUS_abcdef0123456789abcdef0123456789",
    ]);
    expect(request?.tagNos).toEqual(["TAG_0123456789abcdef0123456789abcdef"]);
    expect(request?.tagNames).toEqual(["新输入标签"]);
    expect(request?.reason).toBeUndefined();
    expect(request?.idempotencyKey).toMatch(/^customer-tag-[0-9a-f-]{36}$/);
    await expect(dialog).toBeHidden();
  });
});

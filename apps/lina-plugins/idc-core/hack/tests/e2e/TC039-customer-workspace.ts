import { customerTest as test } from "../support/customer-admin-session";
import { expect } from "@host-tests/fixtures/auth";
import { MainLayout } from "@host-tests/pages/MainLayout";
import { CustomerWorkspacePage } from "../pages/CustomerWorkspacePage";
import {
  installCustomerWorkspaceMock,
  workspaceCustomerNo,
} from "../support/customer-workspace-mock";

test.describe("TC-39 客户钱包、产品和API概览", () => {
  test.afterEach(async ({ adminPage }) => {
    await adminPage.unrouteAll({ behavior: "wait" });
  });
  test("TC-39d: 三条待开通产品和长编号反复切换保持可读列宽", async ({ adminPage }) => {
    await installCustomerWorkspaceMock(adminPage);
    const number = "ORI_f86ab14998d562b9e504edd3dad3cefe";
    await adminPage.route("**/x/idc-core/api/v1/admin/customers/*/products**", async route => {
      await route.fulfill({ json: { code: 0, message: "success", data: {
        total: 3,
        items: ["pending_payment", "paid", "paid"].map((status, index) => ({
          number: index === 0 ? number : `ORI_${index}${"a".repeat(32)}`,
          nameZhCn: "一次性支付协议验收", nameEnUs: "One-time product",
          information: "", informationEnUs: "", deliveryKind: "pending",
          pluginId: "", productType: "other", amountMinor: index === 0 ? "200" : "100",
          currencyCode: "CNY", periodCount: 1, periodUnit: "one_time",
          createdAt: 1791772800000, expiresAt: 0, status,
        })),
      } } });
    });
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await view.openOverview(workspaceCustomerNo);
    for (let index = 0; index < 3; index++) {
      await adminPage.setViewportSize({ width: index === 1 ? 900 : 1280, height: 800 });
      await view.tab("产品");
      const firstRow = view.products.locator(".vxe-table--main-wrapper .vxe-body--row").first();
      await expect(firstRow).toBeVisible();
      await expect.poll(async () => firstRow.locator("td").first().evaluate(el => el.getBoundingClientRect().width)).toBeGreaterThanOrEqual(220);
      await expect.poll(async () => firstRow.locator("td").nth(2).evaluate(el => el.getBoundingClientRect().width)).toBeGreaterThanOrEqual(150);
      await expect(firstRow).toContainText("CNY 2.00");
      await expect(firstRow.getByRole("button", { name: number, exact: true })).toHaveCSS("white-space", "nowrap");
      await expect(view.products).toContainText("产品名称");
      await view.screenshot(`workspace-sparse-products-${index}`);
      await view.tab("客户概览");
    }
  });
  test("TC-39a: 逐币种钱包、实际收支与产品分页，备注和配置使用对应流程", async ({
    adminPage,
  }) => {
    const state = await installCustomerWorkspaceMock(adminPage);
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await expect(view.customerRow(workspaceCustomerNo)).toContainText(
      "CNY 170.00",
    );
    await expect(view.customerRow(workspaceCustomerNo)).toContainText(
      "USD 30.00",
    );
    await expect(view.customerRow(workspaceCustomerNo)).toContainText("赠送金");
    await view.openOverview(workspaceCustomerNo);
    const statistics = adminPage.getByTestId("customer-statistics");
    const productStatistics = adminPage.getByTestId(
      "customer-product-statistics",
    );
    await expect(productStatistics.getByRole("heading")).toHaveText(
      "产品/服务",
    );
    await expect(productStatistics).toContainText("即将到期");
    await expect(productStatistics).toContainText("已到期");
    await expect(
      adminPage.getByTestId("customer-category-cloudServer"),
    ).toHaveText("22云服务器");
    await expect(adminPage.getByTestId("customer-category-ssl")).toHaveText(
      "1SSL证书",
    );
    await expect(productStatistics).not.toContainText("魔方云");
    await expect(statistics).toContainText("总收入");
    await expect(statistics).toContainText("CNY 120.00");
    await expect(statistics).toContainText("USD 50.00");
    await expect(statistics).toContainText("CNY 15.00");
    await expect(statistics).toContainText("CNY 70.00");
    await view.screenshot("workspace-overview");
    await view.showProductStatistics();
    await view.screenshot("workspace-product-statistics");
    const width = (await view.overviewDrawer.boundingBox())?.width;
    await view.viewProductsFromOverview();
    await expect(view.products).toContainText("单域名SSL证书");
    await expect(view.products).toContainText("4核 / 8GB内存");
    await expect(view.products).toContainText("使用中");
    await expect(view.products).toContainText("金额 / 周期");
    await expect(view.products).toContainText("订购 / 到期");
    await expect(
      view.products
        .getByRole("button", { name: "SVC_SERVER", exact: true })
        .first(),
    ).toBeVisible();
    expect((await view.overviewDrawer.boundingBox())?.width).toBe(width);
    const firstProduct = view.products.locator(".vxe-table--main-wrapper .vxe-body--row").first();
    const nameCell = firstProduct.locator(".product-cell").first();
    const cellBounds = await nameCell.boundingBox();
    const rowBounds = await firstProduct.boundingBox();
    expect(cellBounds && rowBounds && cellBounds.y >= rowBounds.y && cellBounds.y + cellBounds.height <= rowBounds.y + rowBounds.height).toBe(true);
    await view.screenshot("workspace-products");
    await view.editServer();
    await expect(view.editor).toContainText("调整配置");
    await expect(view.editor).toContainText("处理器");
    await expect(view.editor).toContainText("4核");
    await view.saveNote("客户服务器备注");
    await expect.poll(() => state.metadata.length).toBe(1);
    expect(state.metadata[0]).toMatchObject({
      remark: "客户服务器备注",
      expectedVersion: 8,
    });
    expect(state.metadata[0]).not.toHaveProperty("amountMinor");
    await view.previewChange();
    await expect(view.editor).toContainText("CNY 20.00");
    await expect(view.editor).toContainText("升级需客户支付账单后生效。");
    await view.screenshot("workspace-server-quote");
    await view.confirmChange(false);
    expect(state.checkouts).toHaveLength(0);
    await view.confirmChange();
    await expect.poll(() => state.checkouts.length).toBe(1);
    expect(state.quotes[0]).toMatchObject({
      currencyCode: "CNY",
      revision: "revision-current",
      selections: [{ optionNo: "OPT_4", quantity: 1 }],
    });
    await expect(view.editor).toBeHidden();
    await view.nextProducts();
    await expect
      .poll(() => state.productPages.at(-1)?.searchParams.get("page"))
      .toBe("2");
    await expect(
      view.products.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(10);
  });
  test("TC-39b: 全部调用分页，确认和失败重置，密钥仅本次展示", async ({
    adminPage,
  }) => {
    const state = await installCustomerWorkspaceMock(adminPage);
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await view.openOverview(workspaceCustomerNo);
    await view.tab("API概览");
    await expect(view.api).toContainText("APP_CUSTOMER");
    await expect(view.api).toContainText("查询支持功能");
    await expect(view.api).toContainText("查询商品");
    await view.screenshot("workspace-api");
    await view.nextCalls();
    await view.nextCalls();
    await expect
      .poll(() => state.callPages.at(-1)?.searchParams.get("page"))
      .toBe("3");
    await expect(
      view.api.locator(".vxe-table--main-wrapper .vxe-body--row"),
    ).toHaveCount(3);
    await view.resetSecret(false);
    expect(state.resets).toHaveLength(0);
    state.resetFailures = 1;
    await view.resetSecret();
    await expect.poll(() => state.resets.length).toBe(1);
    await expect(adminPage.getByTestId("customer-api-secret")).toHaveCount(0);
    await view.screenshot("workspace-api-reset-error");
    // The failed confirmation remains available for a deliberate retry.
    await view.retrySecretConfirm();
    await expect(adminPage.getByTestId("customer-api-secret")).toHaveText(
      "new-secret-visible-once",
    );
    expect(state.resets[1]).toEqual({ tokenId: 42 });
    await view.screenshot("workspace-api-secret");
    await view.tab("产品");
    await view.tab("API概览");
    await expect(adminPage.getByTestId("customer-api-secret")).toHaveCount(0);
  });
  test("TC-39c: 插件停用隐藏操作，加载失败可重试，英文窄屏稳定", async ({
    adminPage,
  }) => {
    const state = await installCustomerWorkspaceMock(adminPage, {
      channel: false,
      resource: false,
    });
    const view = new CustomerWorkspacePage(adminPage);
    await view.goto();
    await view.openOverview(workspaceCustomerNo);
    await expect(
      view.overviewDrawer.getByRole("tab", { name: "API概览", exact: true }),
    ).toHaveCount(0);
    state.productFailures = 1;
    await view.tab("产品");
    await expect(view.products).toContainText("加载失败");
    await view.retryProducts();
    await expect(view.products).toContainText("云服务器");
    await expect(
      view.products.getByTestId("customer-product-edit-SVC_SERVER"),
    ).toHaveCount(0);
    await view.screenshot("workspace-products-plugin-disabled");
    await view.closeOverview();
    const layout = new MainLayout(adminPage);
    await layout.switchLanguage("English");
    await adminPage.setViewportSize({ width: 900, height: 800 });
    await view.openOverview(workspaceCustomerNo);
    await expect(
      adminPage.getByTestId("customer-product-statistics").getByRole("heading"),
    ).toHaveText("Products & services");
    await view.tab("Products");
    await expect(view.products).toContainText("Amount / cycle");
    await expect(view.products).toContainText("Ordered / expires");
    await expect(view.products).toContainText("Product information");
    await expect(view.products).not.toContainText("plugin.idc-core");
    await view.screenshot("workspace-products-en-narrow");
  });
});

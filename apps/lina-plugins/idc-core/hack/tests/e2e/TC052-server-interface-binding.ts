// Verify the product form consumes server-owned interface candidates without duplicating credentials.
import { expect, test } from "@host-tests/fixtures/auth";
import { config } from "@host-tests/fixtures/config";
import { CatalogManagementPage } from "../pages/CatalogManagementPage";
import { installCatalogAutomationMock } from "../support/catalog-automation-mock";
import { installCatalogManagementMock } from "../support/catalog-mock";

test.describe("TC-52 商品选择服务器自动化接口", () => {
  test("TC-52a: 商品选择服务器接口并只提交统一接口引用", async ({ adminPage }) => {
    await adminPage.goto(config.baseURL);
    await installCatalogManagementMock(adminPage);
    const state = await installCatalogAutomationMock(adminPage);
    let candidateQueries = 0;
    await adminPage.route("**/admin/catalog/automation/providers?*", async route => {
      await route.fulfill({ json: { code: 0, data: { items: [{ displayName: "智简魔方云连接器", selectionToken: "current-server-provider" }], total: 1 } } });
    });
    await adminPage.route("**/admin/catalog/automation/interfaces?*", async route => {
      const url = new URL(route.request().url());
      expect(url.searchParams.get("providerSelectionToken")).toBe("current-server-provider");
      expect(url.searchParams.get("newBusinessState")).toBe("accepting");
      candidateQueries += 1;
      await route.fulfill({ json: { code: 0, data: { items: [{
        canDelete: false, changedAt: 1_787_705_660_000, createdAt: 1_787_705_600_000,
        displayName: "南京魔方", interfaceInstanceNo: "server-interface-reference", newBusinessState: "accepting", version: 3,
      }], total: 1 } } });
    });
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    await catalog.openFulfillmentBinding();
    await expect(catalog.fulfillmentBindingModal()).toContainText("自动化接口");
    await catalog.selectFulfillmentBindingOption("provider", "智简魔方云连接器");
    await expect.poll(() => candidateQueries).toBe(1);
    await catalog.selectFulfillmentBindingOption("interface", /南京魔方/u);
    await expect(catalog.fulfillmentBindingModal()).toContainText("南京魔方");
    await expect(catalog.fulfillmentBindingModal()).not.toContainText("plugin.idc-core");
    await catalog.screenshot("server-interface-binding-candidate");
    await catalog.confirmFulfillmentBinding();
    await expect.poll(() => state.bindingRequests.filter(request => request.method === "PUT")).toEqual([
      { body: { interfaceInstanceNo: "server-interface-reference" }, method: "PUT" },
    ]);
    await expect(catalog.fulfillmentBindingModal()).toBeHidden();
    expect(state.interfaceRequests.filter(request => request.method !== "GET")).toHaveLength(0);
  });

});

// UI-only failed connector validation keeps the merchant draft and permits a
// deliberate retry. The fixture never writes a real product or supplier.
import { test, expect } from "../support/merchant-auth";
import {
  installCatalogManagementMock,
  mockProductDetail,
} from "../support/catalog-mock";
import { MerchantOfferPage } from "../pages/MerchantOfferPage";

test("TC-79a: 校验插件失败给出可操作原因，输入保留且可以原样重试", async ({
  adminPage,
}) => {
  await installCatalogManagementMock(adminPage);
  const detail: any = mockProductDetail("product-manual-service");
  detail.version.status = "draft";
  detail.fulfillmentBinding = {
    interfaceInstanceNo: "certificate-fixture",
    interfaceDisplayName: "证书接口",
    evidenceVerified: true,
    interfaceNewBusinessState: "accepting",
  };
  const field = {
    key: "local_note",
    type: "single_line",
    label: { zhCN: "服务备注", enUS: "Service note" },
  };
  let configuration: any = {
    version: 1,
    merchantValues: [],
    productDeclaration: { version: "v1", fields: [] },
    customerDeclaration: { version: "v1", fields: [field] },
    customerFieldSelections: [
      { fieldKey: field.key, localField: field, informationOnly: true },
    ],
    customerConfigurableFields: [],
    productResolvedCandidates: {},
    customerResolvedCandidates: {},
  };
  let fail = true;
  const writes: any[] = [];
  await adminPage.route(
    "**/admin/catalog/products/product-manual-service",
    (route) => route.fulfill({ json: { code: 0, data: detail } }),
  );
  await adminPage.route(
    "**/admin/catalog/product-versions/*/configuration",
    async (route) => {
      if (route.request().method() === "PUT") {
        const body = route.request().postDataJSON();
        writes.push(body);
        if (fail) {
          await route.fulfill({
            json: {
              code: 50,
              errorCode: "IDC_PRODUCT_AUTOMATION_VALIDATION_UNAVAILABLE",
              messageKey: "error.idc.product.automation.validation.unavailable",
              messageParams: { reason: "invocation_failed" },
              message:
                "对接插件暂时无法完成商品配置校验，请稍后重试或检查插件状态。",
              data: null,
            },
          });
          return;
        }
        configuration = {
          ...configuration,
          version: 2,
          customerFieldSelections: body.customerFieldSelections,
          customerDeclaration: {
            version: "v1",
            fields: body.customerFieldSelections.map(
              (row: any) => row.localField,
            ),
          },
        };
      }
      await route.fulfill({ json: { code: 0, data: { configuration } } });
    },
  );
  const view = new MerchantOfferPage(adminPage);
  await view.goto();
  await view.openProductDetail("product-manual-service");
  await view.selectEditSection("购买配置");
  await view.chooseField("local_note");
  await view.input("配置名称（中文）").fill("未保存的客户资料");
  await view.save();
  await expect(
    adminPage.getByText(
      "对接插件暂时无法完成商品配置校验，请稍后重试或检查插件状态。",
      { exact: true },
    ),
  ).toBeVisible();
  await expect(view.input("配置名称（中文）")).toHaveValue("未保存的客户资料");
  await expect(
    adminPage.getByTestId("catalog-customer-options-save"),
  ).toBeEnabled();
  expect(writes).toHaveLength(1);
  await view.screenshot("provider-validation-error-keeps-draft");
  fail = false;
  const response = adminPage.waitForResponse(
    (r) => r.request().method() === "PUT" && r.url().endsWith("/configuration"),
  );
  await view.save();
  expect((await (await response).json()).code).toBe(0);
  await expect(
    adminPage.getByTestId("catalog-customer-options-save"),
  ).toBeEnabled();
  await expect(view.input("配置名称（中文）")).toHaveValue("未保存的客户资料");
  expect(writes).toHaveLength(2);
  expect(writes[0]).toEqual(writes[1]);
  await expect(
    adminPage.getByText(
      "对接插件暂时无法完成商品配置校验，请稍后重试或检查插件状态。",
      { exact: true },
    ),
  ).toBeHidden();
  await view.screenshot("provider-validation-retry-keeps-draft");
});

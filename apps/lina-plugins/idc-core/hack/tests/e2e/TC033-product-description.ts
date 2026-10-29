import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { CatalogManagementPage } from "../pages/CatalogManagementPage";
import { installCatalogManagementMock, mockProductDetail } from "../support/catalog-mock";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";

test("TC-33: 商品介绍双语编辑、冲突保留和重载", async ({ adminPage }) => {
  await installCatalogManagementMock(adminPage);
  let current = { descriptionVersion: 1, nameZhCn: "人工迁移服务", nameEnUs: "Manual migration", summaryZhCn: "", summaryEnUs: "" };
  let conflict = true;
  await adminPage.route("**/x/idc-core/api/v1/admin/catalog/products/product-manual-service/description", async (route) => {
    const body = route.request().postDataJSON();
    if (conflict) { await route.fulfill({ json: { code: 409, message: "商品已被其他人修改，请刷新后再试", data: null } }); return; }
    expect(body.version).toBe(1);
    current = { ...current, ...body, descriptionVersion: 2 };
    await route.fulfill({ json: { code: 0, data: { version: 2 }, message: "" } });
  });
  await adminPage.route("**/x/idc-core/api/v1/admin/catalog/products/product-manual-service", async (route) => {
    const data = mockProductDetail("product-manual-service");
    data.product = { ...data.product, productNo: "product-manual-service", ...current };
    await route.fulfill({ json: { code: 0, data, message: "" } });
  });
  const catalog = new CatalogManagementPage(adminPage);
  const screenshot = async (name: string) => {
    await adminPage.getByText(/加载菜单中/).waitFor({ state: "hidden", timeout: 15_000 });
    const now = new Date();
    const directory = resolve(process.cwd(), "../../temp", now.toISOString().slice(0, 10).replaceAll("-", ""));
    await mkdir(directory, { recursive: true });
    await adminPage.screenshot({ path: resolve(directory, `${now.toISOString().slice(11, 19).replaceAll(":", "")}-${name}.png`), animations: "disabled" });
  };
  await catalog.goto();
  await catalog.openProductDetail("product-manual-service");
  await screenshot("product-description-loaded");
  await catalog.selectEditSection("基本资料");
  await expect(adminPage.getByTestId("catalog-description-editor")).toContainText("中文商品名称");
  await expect(adminPage.getByTestId("catalog-description-editor")).toContainText("英文商品介绍");
  await adminPage.getByTestId("catalog-description-summary-zh").fill("专业迁移服务");
  await adminPage.getByTestId("catalog-description-summary-en").fill("Professional migration service");
  await screenshot("product-description-editor");
  await adminPage.getByTestId("catalog-description-save").click();
  await expect(adminPage.getByText("商品已被其他人修改，请刷新后再试")).toBeVisible();
  await expect(adminPage.getByTestId("catalog-description-summary-zh")).toHaveValue("专业迁移服务");
  await screenshot("product-description-conflict");
  await expect(adminPage.getByText("商品已被其他人修改，请刷新后再试")).toBeHidden();
  conflict = false;
  await adminPage.getByTestId("catalog-description-save").click();
  await expect(adminPage.getByText("商品介绍已保存")).toBeVisible();
  await expect(catalog.detailDrawer).toContainText("专业迁移服务");
  await expect(adminPage.getByTestId("catalog-description-editor")).toBeVisible();
  await screenshot("product-description-saved");
});

import { test, expect } from "../support/catalog-auth";

import { workspacePath } from "../../../../../../hack/tests/fixtures/config";
import { CatalogManagementPage } from "../pages/CatalogManagementPage";
import {
  installCatalogManagementMock,
  products,
} from "../support/catalog-mock";

const forbiddenServerText =
  /cloud_server|bare_metal|CPU|内存|操作系统|磁盘|带宽|资源池|线路|原始 JSON/iu;

test.describe("TC-2 通用商品目录", () => {
  test("TC-2a: 商品列表同时展示证书、手机和人工服务且没有服务器专用字段", async ({
    adminPage,
  }) => {
    await installCatalogManagementMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();

    await expect(catalog.productRow("product-ssl")).toContainText(
      "通配符 SSL 证书",
    );
    await expect(catalog.productRow("product-phone")).toContainText("商务手机");
    await expect(catalog.productRow("product-manual-service")).toContainText(
      "人工迁移服务",
    );
    const tableHeight = await catalog.pageRoot
      .locator(".vxe-table")
      .first()
      .evaluate((element) => element.getBoundingClientRect().height);
    expect(tableHeight).toBeGreaterThan(300);
    await expect(catalog.pageRoot).not.toContainText(forbiddenServerText);
    await expect(catalog.pageRoot).not.toContainText("plugin.idc-core");
    await catalog.screenshot("catalog-general-products");

    const pagedRequests: URL[] = [];
    const certificatePages = Array.from({ length: 11 }, (_, index) => ({
      ...products[0],
      productNo: `product-page-${index + 1}`,
      nameZhCn: `分页证书 ${index + 1}`,
    }));
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/catalog/products**",
      async (route) => {
        const request = route.request();
        const url = new URL(request.url());
        if (
          request.method() !== "GET" ||
          !url.pathname.endsWith("/admin/catalog/products")
        ) {
          await route.fallback();
          return;
        }
        pagedRequests.push(url);
        if (
          !url.searchParams.has("categoryNo") &&
          !url.searchParams.has("cursor")
        ) {
          await route.fallback();
          return;
        }
        await route.fulfill({
          json: {
            code: 0,
            data: {
              nextCursor: "",
              products: certificatePages.slice(
                (Number(url.searchParams.get("page") || 1) - 1) *
                  Number(url.searchParams.get("limit") || 10),
                Number(url.searchParams.get("page") || 1) *
                  Number(url.searchParams.get("limit") || 10),
              ),
              total: certificatePages.length,
            },
            message: "success",
          },
        });
      },
    );
    await adminPage.getByTestId("catalog-search-category").click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("证书服务", { exact: true })
      .click();
    await catalog.pageRoot
      .getByRole("button", { name: /查\s*询|搜\s*索/iu })
      .click();
    await expect
      .poll(() => pagedRequests.at(-1)?.searchParams.get("categoryNo"))
      .toBe("ctg_00000000000000000000000000000002");
    await expect(catalog.nextButton).toBeEnabled();
    await catalog.nextButton.click();
    await expect
      .poll(() => pagedRequests.at(-1)?.searchParams.get("page"))
      .toBe("2");
    expect(pagedRequests.at(-1)?.searchParams.get("categoryNo")).toBe(
      "ctg_00000000000000000000000000000002",
    );
    await catalog.pageRoot
      .getByRole("button", { name: /重\s*置|Reset/iu })
      .click();
    await expect
      .poll(() => pagedRequests.at(-1)?.searchParams.get("categoryNo"))
      .toBeNull();
    expect(pagedRequests.at(-1)?.searchParams.get("cursor")).toBeNull();
    await expect(catalog.pageRoot).toContainText("1 个币种 · 1 条计费规则");
  });

  test("TC-2b: 分类使用树形层级并可把新分类放到搜索到的上级分类下", async ({
    adminPage,
  }) => {
    const state = await installCatalogManagementMock(adminPage);
    await adminPage.goto(workspacePath("/idc/catalog/categories"));
    const pageRoot = adminPage.getByTestId("catalog-category-page");
    await pageRoot.waitFor({ state: "visible" });

    const rootRow = pageRoot
      .locator(".vxe-body--row", { hasText: "数字产品" })
      .first();
    await expect(rootRow).toBeVisible();
    await expect(pageRoot.getByText("证书服务", { exact: true })).toHaveCount(
      0,
    );
    await rootRow.locator(".vxe-cell--tree-btn").click();
    await expect(pageRoot.getByText("证书服务", { exact: true })).toBeVisible();

    await adminPage
      .getByTestId("catalog-category-edit-ctg_00000000000000000000000000000001")
      .click();
    const editModal = adminPage.getByTestId("catalog-category-edit-modal");
    await editModal.waitFor({ state: "visible" });
    await adminPage.getByTestId("catalog-category-edit-parent").click();
    const editDropdown = adminPage.locator(".ant-select-dropdown:visible");
    await expect(
      editDropdown.getByText("数字产品", { exact: true }),
    ).toHaveCount(0);
    await expect(
      editDropdown.getByText("证书服务", { exact: true }),
    ).toHaveCount(0);
    await expect(
      editDropdown.getByText("实物商品", { exact: true }),
    ).toBeVisible();
    await adminPage.keyboard.press("Escape");
    await adminPage.keyboard.press("Escape");
    await editModal.waitFor({ state: "hidden" });

    await adminPage.getByTestId("catalog-category-create-open").click();
    const modal = adminPage.getByTestId("catalog-category-create-modal");
    await modal.waitFor({ state: "visible" });
    await adminPage.getByTestId("catalog-category-create-parent").click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("数字产品", { exact: true })
      .click();
    await modal.getByTestId("catalog-category-create-name-zh").fill("安全产品");
    await modal
      .getByTestId("catalog-category-create-name-en")
      .fill("Security products");
    await adminPage
      .getByRole("dialog")
      .filter({ has: modal })
      .getByRole("button", { name: "创建分类" })
      .click();
    await modal.waitFor({ state: "hidden" });

    const request = state.categoryRequests.find(
      (item) => item.method === "POST",
    );
    expect(request?.body).toMatchObject({
      nameEnUs: "Security products",
      nameZhCn: "安全产品",
      parentCategoryNo: "ctg_00000000000000000000000000000001",
    });
    expect(request?.body).not.toHaveProperty("type");
  });

  test("TC-2c: 新建人工服务商品只提交通用商品字段", async ({ adminPage }) => {
    const state = await installCatalogManagementMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductCreate();

    await adminPage.getByTestId("catalog-product-create-category").click();
    await adminPage
      .locator(".ant-select-dropdown:visible")
      .getByText("专业服务", { exact: true })
      .click();
    await catalog.productCreateModal
      .getByTestId("catalog-product-create-slug")
      .fill("remote-migration-service");
    await catalog.productCreateModal
      .getByTestId("catalog-product-create-name-zh")
      .fill("远程迁移服务");
    await catalog.productCreateModal
      .getByTestId("catalog-product-create-name-en")
      .fill("Remote migration service");
    await catalog.productCreateModal
      .getByTestId("catalog-product-create-summary-zh")
      .fill("由工程师人工处理迁移");
    await catalog.productCreateModal
      .getByTestId("catalog-product-create-summary-en")
      .fill("Migration fulfilled by an engineer");
    await catalog.productCreateModal
      .getByTestId("catalog-product-create-inventory-tracking")
      .click();
    await catalog.productCreateModal
      .getByTestId("catalog-product-create-sellable-quantity")
      .fill("12");
    const dialog = adminPage
      .getByRole("dialog")
      .filter({ has: catalog.productCreateModal });
    await dialog.getByRole("button", { name: "创建商品草稿" }).click();
    await catalog.productCreateModal.waitFor({ state: "hidden" });
    await catalog.waitForDataSettled();

    const request = state.productCreateRequests[0]?.body as Record<
      string,
      unknown
    >;
    expect(request).toMatchObject({
      categoryNo: "ctg_00000000000000000000000000000004",
      nameEnUs: "Remote migration service",
      nameZhCn: "远程迁移服务",
      publicSlug: "remote-migration-service",
      inventoryTrackingEnabled: true,
      sellableQuantity: 12,
    });
    for (const field of [
      "type",
      "productType",
      "configurationMode",
      "cpu",
      "memory",
      "resourcePoolNo",
    ]) {
      expect(request).not.toHaveProperty(field);
    }
    await expect(catalog.productRow("product-created")).toContainText(
      "远程迁移服务",
    );
  });

  test("TC-2c-2: 草稿商品可直接编辑简单库存", async ({ adminPage }) => {
    const state = await installCatalogManagementMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-manual-service");
    const inventory = catalog.detailDrawer.getByTestId(
      "catalog-product-inventory",
    );
    await expect(inventory).toContainText("可售数量");
    await inventory.getByRole("button", { name: "编辑库存" }).click();
    const dialog = adminPage.getByRole("dialog", { name: "编辑库存" });
    await dialog.getByTestId("catalog-inventory-quantity").fill("40");
    await dialog.getByRole("button", { name: /确\s*认|Confirm/iu }).click();
    await expect(dialog).toBeHidden();
    expect(state.inventoryRequests[0]?.body).toMatchObject({
      expectedVersion: 1,
      sellableQuantity: 40,
      trackingEnabled: true,
    });
    await expect(inventory).toContainText("40");
  });

  test("TC-2d: 证书和手机详情只展示通用资料、版本与价格", async ({
    adminPage,
  }) => {
    const state = await installCatalogManagementMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();

    await catalog.openProductDetail("product-ssl");
    await expect(catalog.detailDrawer).toContainText("通配符 SSL 证书");
    await expect(catalog.detailDrawer).toContainText("证书服务");
    await catalog.selectEditSection("价格与税务");
    await expect(
      catalog.detailDrawer.getByTestId("catalog-detail-pricing"),
    ).toContainText("¥199.00");
    await expect(
      catalog.detailDrawer.getByTestId("catalog-detail-tax"),
    ).toContainText("6%");
    await expect(catalog.detailDrawer).not.toContainText(forbiddenServerText);
    await adminPage.keyboard.press("Escape");
    await catalog.detailDrawer.waitFor({ state: "hidden" });

    await catalog.openProductDetail("product-phone");
    await expect(catalog.detailDrawer).toContainText("商务手机");
    await expect(catalog.detailDrawer).toContainText("实物商品");
    await expect(catalog.detailDrawer).not.toContainText(forbiddenServerText);
    expect(state.detailRequests).toHaveLength(2);
    await catalog.screenshot("catalog-phone-detail");
  });

  test("TC-2d-1: 超大金额按十进制字符串精确展示", async ({ adminPage }) => {
    await installCatalogManagementMock(adminPage);
    await adminPage.route(
      "**/x/idc-core/api/v1/admin/catalog/products/product-ssl",
      async (route) => {
        await route.fulfill({
          json: {
            code: 0,
            data: {
              fulfillmentBinding: null,
              fulfillmentPolicy: null,
              inventory: {
                sellableQuantity: 25,
                trackingEnabled: true,
                version: 1,
              },
              billingRules: [
                {
                  billingRuleNo: "billing-huge",
                  chargeType: "one_time",
                  maximumRenewalCycles: null,
                  meterUnit: null,
                  minimumPurchaseCycles: 1,
                  periodCount: 1,
                  periodUnit: "once",
                  pricePhase: "new_purchase",
                  quantityMode: "fixed",
                  ruleCode: "huge-price",
                  status: "active",
                  tierMode: null,
                  trialQuantity: 0,
                  usageCollectionCount: null,
                  usageCollectionUnit: null,
                  usageMaximumQuantity: null,
                  usageMinimumQuantity: null,
                  usageMissingPolicy: null,
                  usageRoundingMode: null,
                  usageRoundingQuantity: null,
                },
              ],
              prices: [
                {
                  amountMinor: "900719925474099312345",
                  billingRuleNo: "billing-huge",
                  currencyCode: "CNY",
                  maximumQuantity: null,
                  minimumQuantity: 1,
                  priceNo: "price-huge",
                  renewalAmountMinor: null,
                  setupFeeMinor: "0",
                  status: "active",
                },
              ],
              product: {
                categoryNameEnUs: "Certificate services",
                categoryNameZhCn: "证书服务",
                categoryNo: "ctg_00000000000000000000000000000002",
                createdAt: 1_786_161_600_000,
                nameEnUs: "Wildcard SSL certificate",
                nameZhCn: "通配符 SSL 证书",
                productNo: "product-ssl",
                publicSlug: "wildcard-ssl",
                sortOrder: 10,
                status: "active",
                summaryEnUs: "Certificate product",
                summaryZhCn: "证书商品",
                updatedAt: 1_786_166_400_000,
              },
              taxPolicies: [],
              version: {
                changeSummary: "",
                contractPolicyCode: "none",
                contractPolicyVersion: 1,
                contractTemplateNo: "",
                createdAt: 1_786_161_600_000,
                publishedAt: 1_786_166_400_000,
                requiresContract: false,
                retiredAt: null,
                source: "manual",
                sourceVersion: "",
                status: "published",
                versionNo: "version-ssl-1",
                versionNumber: 1,
              },
            },
            message: "success",
          },
        });
      },
    );
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-ssl");
    await catalog.selectEditSection("价格与税务");
    await expect(
      catalog.detailDrawer.getByTestId("catalog-detail-pricing"),
    ).toContainText("¥9,007,199,254,740,993,123.45");
  });

  test("TC-2d-2: 商品版本成本使用标准弹窗且只展示通用字段", async ({
    adminPage,
  }) => {
    const state = await installCatalogManagementMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.openProductDetail("product-phone");

    await catalog.selectEditSection('价格与税务');
    const costs = catalog.detailDrawer.getByTestId("catalog-product-costs");
    await expect(costs).toContainText("商品版本成本");
    await expect(costs).toContainText("CNY");
    await expect(costs).toContainText("¥88.00");
    await expect(costs).not.toContainText(
      /资源池|供应商|服务器|绑定编号|审计/iu,
    );

    await costs.getByRole("button", { name: "添加成本" }).click();
    const dialog = adminPage.getByRole("dialog", { name: "添加成本" });
    await expect(dialog).toBeVisible();
    await expect(dialog).toContainText("币种");
    await expect(dialog).toContainText("周期单位");
    await expect(dialog).toContainText("周期数量");
    await expect(dialog).toContainText("单位成本");
    await expect(dialog).not.toContainText(/资源池|供应商|服务器|审批|审计/iu);
    expect(
      state.productVersionCostRequests.some((item) => item.method === "GET"),
    ).toBeTruthy();
    await adminPage.waitForTimeout(500);
    await catalog.screenshot("catalog-product-cost-modal");
    await dialog.getByTestId("catalog-cost-unit-amount").fill("99.50");
    await dialog.getByRole("button", { name: /确\s*认|Confirm/iu }).click();
    await expect(dialog).toBeHidden();
    await expect(costs).toContainText("¥99.50");
    const createRequest = state.productVersionCostRequests.find(
      (item) => item.method === "POST",
    );
    expect(createRequest?.body).toMatchObject({
      currencyCode: "CNY",
      unitCostMinor: "9950",
    });

    await costs.getByRole("button", { name: "编辑" }).first().click();
    const editDialog = adminPage.getByRole("dialog", { name: "编辑成本" });
    await editDialog.getByTestId("catalog-cost-unit-amount").fill("100.00");
    await editDialog.getByRole("button", { name: /确\s*认|Confirm/iu }).click();
    await expect(editDialog).toBeHidden();
    const updateRequest = state.productVersionCostRequests.find(
      (item) => item.method === "PUT",
    );
    expect(updateRequest?.body).toMatchObject({
      expectedVersion: 1,
      unitCostMinor: "10000",
    });
  });

  test("TC-2e: 商品设置没有独立发布与预览步骤", async ({adminPage}) => {
    const state=await installCatalogManagementMock(adminPage,{publicationReady:false});
    const catalog=new CatalogManagementPage(adminPage);await catalog.goto();
    await expect(adminPage.getByTestId('catalog-product-publish-product-manual-service')).toHaveCount(0);
    await catalog.openProductDetail('product-manual-service');
    await expect(catalog.detailDrawer).toContainText('每项设置保存后立即生效');
    await expect(adminPage.getByTestId('catalog-editor-step-publication')).toHaveCount(0);
    expect(state.previewRequests).toHaveLength(0);expect(state.publishRequests).toHaveLength(0);
  });

  test("TC-2f: 英文页面完整显示通用商品文案", async ({
    adminPage,
    mainLayout,
  }) => {
    await installCatalogManagementMock(adminPage);
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await mainLayout.switchLanguage("English");

    await expect(catalog.productRow("product-ssl")).toContainText(
      "Wildcard SSL certificate",
    );
    await expect(catalog.productRow("product-phone")).toContainText(
      "Business phone",
    );
    await expect(catalog.productRow("product-manual-service")).toContainText(
      "Manual migration service",
    );
    await expect(catalog.productRow("product-ssl")).toContainText(
      "Currencies: 1 · Billing rules: 1",
    );
    await expect(catalog.pageRoot).not.toContainText("plugin.idc-core");
    await expect(catalog.pageRoot).not.toContainText(
      /Server type|Bare metal|Resource pool/iu,
    );
  });

  test("TC-2g: 真实商品详情与库存编辑不再误报目录服务不可用", async ({
    adminPage,
  }) => {
    const catalog = new CatalogManagementPage(adminPage);
    await catalog.goto();
    await catalog.searchNamePrefix("SSL 证书");

    const productNo = "prd_00000000000000000000000000000012";
    await expect(catalog.productRow(productNo)).toContainText("SSL 证书");
    await catalog.openProductDetail(productNo);
    await expect(catalog.detailDrawer).toContainText("SSL 证书");
    await expect(catalog.body).not.toContainText("商品目录管理服务暂时不可用");

    const inventory = catalog.detailDrawer.getByTestId(
      "catalog-product-inventory",
    );
    const editInventory = inventory.getByRole("button", { name: "编辑库存" });
    await expect(editInventory).toBeVisible();
    await editInventory.click();
    await expect(
      adminPage.getByRole("dialog", { name: "编辑库存" }),
    ).toBeVisible();
    await expect(catalog.body).not.toContainText("商品目录管理服务暂时不可用");
  });
});

// Local product fields must remain usable while supplier-dependent data is
// pending or unavailable. All responses and edits here are browser fixtures.
import type { Page } from "@host-tests/support/playwright";
import { test, expect } from "../support/merchant-auth";
import {
  installCatalogManagementMock,
  mockProductDetail,
} from "../support/catalog-mock";
import { CatalogWorkspacePage } from "../pages/CatalogWorkspacePage";

function pendingRead() {
  let release!: () => void;
  const promise = new Promise<void>((resolve) => {
    release = resolve;
  });
  return { promise, release };
}
function configuration(cpu: string) {
  const field = {
    key: "cpu",
    type: "integer",
    label: { zhCN: "处理器核心数", enUS: "CPU cores" },
    minimum: "1",
    maximum: "128",
    required: true,
  };
  return {
    version: 1,
    merchantValues: [{ fieldKey: "cpu", integerValue: cpu }],
    productDeclaration: { version: "v1", fields: [field] },
    customerDeclaration: { version: "v1", fields: [] },
    customerConfigurableFields: [],
    customerFieldSelections: [],
    productResolvedCandidates: {},
    customerResolvedCandidates: {},
  };
}
async function fixture(page: Page) {
  await installCatalogManagementMock(page);
  const first: any = mockProductDetail("product-manual-service"),
    second: any = mockProductDetail("product-phone");
  for (const detail of [first, second]) {
    detail.product.descriptionVersion = 1;
    detail.version.status = "draft";
    detail.fulfillmentBinding = {
      interfaceInstanceNo: "progressive-interface",
      interfaceDisplayName: "隔离测试上游",
      evidenceVerified: true,
      interfaceNewBusinessState: "accepting",
    };
    await page.route(
      "**/admin/catalog/products/" + detail.product.productNo,
      (route) => route.fulfill({ json: { code: 0, data: detail } }),
    );
  }
  await page.route(
    "**/admin/catalog/product-versions/*/configuration/evaluation",
    (route) => {
      const input = route.request().postDataJSON();
      return route.fulfill({
        json: {
          code: 0,
          data: {
            version: 1,
            evaluation: {
              valid: true,
              values: input.merchantValues,
              issues: [],
              fieldStates: {
                cpu: { visible: true, required: true, disabled: false },
              },
              candidates: {},
            },
          },
        },
      });
    },
  );
  await page.route("**/admin/catalog/product-versions/*/costs", (route) =>
    route.fulfill({
      json: {
        code: 0,
        data: {
          costs: [
            {
              costNo: "fixture-existing-cost",
              currencyCode: "CNY",
              periodUnit: "month",
              periodCount: 1,
              unitCostMinor: "500",
              effectiveFrom: 1_786_161_600_000,
              effectiveUntil: null,
            },
          ],
        },
      },
    }),
  );
  return { first, second, view: new CatalogWorkspacePage(page) };
}

test.describe("TC-68 商品本地资料先显示", () => {
  test("TC-68a: 上游读取未完成仍能填写基本资料，配置到达不清除输入", async ({
    adminPage,
  }) => {
    test.setTimeout(30_000);
    const f = await fixture(adminPage),
      pending = pendingRead();
    let reads = 0;
    await adminPage.route(
      "**/admin/catalog/product-versions/*/configuration",
      async (route) => {
        reads++;
        await pending.promise;
        await route.fulfill({
          json: { code: 0, data: { configuration: configuration("2") } },
        });
      },
    );
    try {
      await f.view.goto();
      await f.view.openProductDetail("product-manual-service");
      await expect(f.view.descriptionName).toBeVisible();
      await f.view.descriptionName.fill("配置等待时已填写的商品名");
      await f.view.selectEditSection("交付设置");
      await expect(f.view.settingsLoading).toBeVisible();
      await expect(f.view.productParameters).toHaveCount(0);
      expect(reads).toBe(1);
      await f.view.screenshot("product-local-fields-supplier-pending");
      pending.release();
      await f.view.selectEditSection("购买配置");
      await expect(adminPage.getByTestId("catalog-option-child-value")).toHaveValue("2");
      await f.view.selectEditSection("基本资料");
      await expect(f.view.descriptionName).toHaveValue(
        "配置等待时已填写的商品名",
      );
      expect(reads).toBe(1);
      await f.view.screenshot("product-local-fields-after-supplier-read");
    } finally {
      pending.release();
    }
  });

  test("TC-68b: 英文窄屏读取失败不冒充空成本，单独重试保留基本资料", async ({
    adminPage,
    mainLayout,
  }) => {
    const f = await fixture(adminPage);
    let reads = 0;
    await adminPage.route(
      "**/admin/catalog/product-versions/*/configuration",
      (route) => {
        if (++reads === 1)
          return route.fulfill({
            status: 503,
            json: {
              code: 50,
              message: "Supplier choices are temporarily unavailable",
            },
          });
        return route.fulfill({
          json: { code: 0, data: { configuration: configuration("2") } },
        });
      },
    );
    await mainLayout.switchLanguage("English");
    await adminPage.setViewportSize({ width: 390, height: 844 });
    await f.view.goto();
    await f.view.openProductDetail("product-manual-service");
    await f.view.descriptionName.fill("Preserve this local draft");
    for (const section of [
      "basic",
      "fulfillment",
      "configuration",
    ]) {
      await expect(
        adminPage.getByTestId("catalog-editor-tab-" + section),
      ).toBeInViewport({ ratio: 1 });
    }
    await f.view.selectEditSection("交付设置");
    await expect(f.view.settingsFailed).toContainText(
      "Saved configuration could not be read. Your product information and prices remain available.",
    );
    await expect(f.view.settingsRetry).toHaveText("Reload");
    await f.view.selectEditSection("价格与税务");
    await expect(f.view.productCosts).not.toContainText("No cost setting");
    await expect(
      f.view.productCosts.getByRole("button", {
        name: "Add cost",
        exact: true,
      }),
    ).toHaveCount(0);
    await f.view.showProductCosts();
    await f.view.screenshot("product-supplier-failure-costs-english-mobile");
    await f.view.selectEditSection("交付设置");
    await f.view.settingsRetry.click();
    await f.view.selectEditSection("购买配置");
    await expect(adminPage.getByTestId("catalog-option-child-value")).toHaveValue("2");
    await f.view.selectEditSection("基本资料");
    await expect(f.view.descriptionName).toHaveValue(
      "Preserve this local draft",
    );
    expect(reads).toBe(2);
    await f.view.selectEditSection("价格与税务");
    await f.view.showProductCosts();
    await expect(f.view.productCosts).toContainText("CNY 5.00");
    await expect(
      f.view.productCosts.getByRole("button", {
        name: "Add cost",
        exact: true,
      }),
    ).toBeVisible();
    await f.view.showProductCosts();
    await f.view.screenshot(
      "product-supplier-retry-cost-restored-english-mobile",
    );
  });

  test("TC-68c: 另一商品已经打开时，迟到的上游配置不能覆盖当前商品", async ({
    adminPage,
  }) => {
    const f = await fixture(adminPage),
      pending = pendingRead();
    await adminPage.route(
      "**/admin/catalog/product-versions/*/configuration",
      async (route) => {
        const first = route
          .request()
          .url()
          .includes("/" + f.first.version.versionNo + "/");
        if (first) await pending.promise;
        await route.fulfill({
          json: {
            code: 0,
            data: { configuration: configuration(first ? "2" : "8") },
          },
        });
      },
    );
    try {
      await f.view.goto();
      await f.view.openProductDetail("product-manual-service");
      await f.view.closeProductDetail();
      await f.view.openProductDetail("product-phone");
      await f.view.selectEditSection("交付设置");
      await f.view.selectEditSection("购买配置");
      await expect(adminPage.getByTestId("catalog-option-child-value")).toHaveValue("8");
      const lateResponse = adminPage.waitForResponse((response) =>
        response
          .url()
          .includes("/" + f.first.version.versionNo + "/configuration"),
      );
      pending.release();
      await lateResponse;
      await f.view.selectEditSection("购买配置");
      await expect(adminPage.getByTestId("catalog-option-child-value")).toHaveValue("8");
      await expect(
        f.view.detailDrawer.getByRole("heading", {
          name: "商务手机",
          exact: true,
        }),
      ).toBeVisible();
      await f.view.screenshot("product-late-supplier-response-isolated");
    } finally {
      pending.release();
    }
  });
  test("TC-68d: 明确展示上游同步失败及旧目录，恢复后自动清除且保留输入", async ({adminPage}) => {
    test.setTimeout(45000);
    const f=await fixture(adminPage);
    let phase:'pending'|'failed'|'retained'|'ready'='pending';
    await adminPage.route('**/admin/catalog/product-versions/*/configuration',r=>{
      const config={...configuration('2'),upstreamOptionsStatus:phase,productResolvedCandidates:phase==='pending'||phase==='failed'?{region:[]}: {}};
      return r.fulfill({json:{code:0,data:{configuration:config}}});
    });
    await f.view.goto();await f.view.openProductDetail('product-manual-service');
    const status=adminPage.getByTestId('catalog-upstream-sync-status');
    await expect(status).toBeVisible();await expect(status).not.toContainText('接口同步失败');
    await f.view.selectEditSection('购买配置');
    const cpu=adminPage.getByTestId("catalog-option-child-value");await cpu.fill('4');
    phase='failed';await expect(status).toContainText('上游接口同步失败',{timeout:10000});await expect(cpu).toHaveValue('4');
    await status.scrollIntoViewIfNeeded();await f.view.screenshot('supplier-sync-failure-visible');
    phase='retained';await expect(status).toContainText('正在显示上次成功同步的选项',{timeout:10000});await expect(cpu).toHaveValue('4');
    await status.scrollIntoViewIfNeeded();await f.view.screenshot('supplier-sync-retained-options-visible');
    phase='ready';await expect(status).toHaveCount(0,{timeout:10000});await expect(cpu).toHaveValue('4');
    await expect(adminPage.getByTestId('catalog-customer-options-save')).toBeEnabled();
  });

});

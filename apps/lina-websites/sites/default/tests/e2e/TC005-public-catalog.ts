// This case verifies published catalog discovery never presents a summary as a quote.
import { expect, test } from "@playwright/test";

import { PublicCatalogPage } from "../pages/PublicCatalogPage";
import {
  mockPublicCatalog,
  type PublicCatalogMockState,
} from "../support/catalog-mock";
import { capturePublicScreenshot } from "../support/screenshot";

test.describe("TC-5 公开销售目录", () => {
  let mockState: PublicCatalogMockState;

  test.beforeEach(async ({ page }) => {
    mockState = await mockPublicCatalog(page);
  });

  test("TC-5a: 中文目录只展示安全销售投影", async ({ page }) => {
    const catalog = new PublicCatalogPage(page);
    await catalog.goto();

    await expect(catalog.products).toContainText("日常手机套餐");
    await expect(catalog.products).toContainText("库存可用");
    await expect(catalog.catalog).toContainText("请进入商品详情选择配置并确认报价");
    await expect(catalog.products).not.toContainText("opaque");
    await expect(catalog.products).not.toContainText("Provider");
    await expect(catalog.products).not.toContainText("¥");
    await capturePublicScreenshot(page, "public-catalog-zh");
  });

  test("TC-5b: 币种筛选由服务端查询承担", async ({ page }) => {
    const catalog = new PublicCatalogPage(page);
    const request = page.waitForRequest((candidate) =>
      candidate.url().includes("/public/catalog/products?") &&
      new URL(candidate.url()).searchParams.get("currency") === "USD",
    );
    await catalog.goto();
    await catalog.chooseCurrency("USD");
    await request;
    await expect(catalog.products).toContainText("USD");
  });

  test("TC-5c: 手机商品详情不伪造价格", async ({ page }) => {
    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/en-US/products/everyday-phone");

    await expect(catalog.detail).toContainText("Everyday smartphone package");
    await expect(catalog.detail).toContainText("Smart devices");
    await expect(catalog.detail).not.toContainText("$99");
  });

  test("TC-5e: 详情只展示公开活动并按设备、活动、币种去重曝光", async ({ page }) => {
    const catalog = new PublicCatalogPage(page);
    await catalog.goto("/products/everyday-phone");

    await expect(catalog.campaigns).toContainText("日常手机套餐活动");
    await expect.poll(() => mockState.recordedExposureCount).toBe(1);
    const firstDevice = mockState.exposureAttempts.at(0)?.deviceId;
    expect(firstDevice).toHaveLength(36);

    await page.reload({ waitUntil: "domcontentloaded" });
    await expect(catalog.campaigns).toContainText("日常手机套餐活动");
    await expect.poll(() => mockState.exposureAttempts.length).toBeGreaterThanOrEqual(2);
    expect(mockState.recordedExposureCount).toBe(1);
    expect(mockState.exposureAttempts.at(-1)?.deviceId).toBe(firstDevice);

    await catalog.chooseCampaignCurrency("USD");
    await expect.poll(() => mockState.recordedExposureCount).toBe(2);
    expect(mockState.exposureAttempts.at(-1)).toMatchObject({
      currencyCode: "USD",
      deviceId: firstDevice,
    });
    await capturePublicScreenshot(page, "public-campaign-detail");
  });

  test("TC-5d: 移动端目录无横向溢出", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    const catalog = new PublicCatalogPage(page);
    await catalog.goto();
    expect(await catalog.hasHorizontalOverflow()).toBe(false);
    await capturePublicScreenshot(page, "public-catalog-mobile");
  });
  test("TC-5f: 停用币种从筛选和详情移除，恢复后重新可选", async ({ page }) => {
    const catalog = new PublicCatalogPage(page);
    mockState.availableCurrencies = ["CNY"];
    await catalog.goto();
    await expect(catalog.products).toContainText("日常手机套餐");
    await expect(catalog.products).not.toContainText("USD");
    await catalog.openCurrencyChoices();
    await expect(catalog.currencyChoices).toContainText("CNY");
    await expect(catalog.currencyChoices).not.toContainText("USD");
    await capturePublicScreenshot(page,"public-currency-disabled");
    await catalog.goto("/products/everyday-phone");
    await expect(catalog.detail).toContainText("CNY");
    await expect(catalog.detail).not.toContainText("USD");
    mockState.availableCurrencies = ["CNY","USD"];
    await catalog.goto();
    await catalog.chooseCurrency("USD");
    await expect(catalog.products).toContainText("USD");
    await expect(catalog.products).not.toContainText("托管 SSL 证书");
    await capturePublicScreenshot(page,"public-currency-restored");
  });

});

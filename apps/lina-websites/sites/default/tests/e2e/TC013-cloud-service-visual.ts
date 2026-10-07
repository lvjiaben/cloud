import { expect, test } from "@playwright/test";

import { PublicSitePage } from "../pages/PublicSitePage";
import { capturePublicScreenshot } from "../support/screenshot";

test.describe("TC013 官网企业云服务视觉", () => {
  test("TC013a: 中文桌面端使用深蓝页头、直角控件和清晰服务入口", async ({
    page,
  }) => {
    const site = new PublicSitePage(page);
    await site.goto("/");

    await expect(page.locator(".home-service-index")).toContainText("商品");
    await expect(page.locator(".home-service-index")).toContainText("动态");
    await expect(page.locator(".home-service-index")).toContainText("指南");
    await expect(page.locator(".site-header")).toHaveCSS(
      "background-color",
      "rgb(7, 26, 54)",
    );
    await expect(site.customerCenter).toHaveCSS("border-radius", "0px");
    await expect(site.customerCenter).toHaveCSS(
      "background-color",
      "rgb(22, 119, 255)",
    );
    await expect(site.customerCenter).toHaveCSS("color", "rgb(255, 255, 255)");
    await expect(page.locator(".home-hero")).toHaveCSS(
      "border-top-width",
      "3px",
    );
    await capturePublicScreenshot(page, "public-cloud-visual-zh-desktop");
  });

  test("TC013b: 英文移动端保持同一视觉且没有横向溢出", async ({
    page,
  }) => {
    await page.setViewportSize({ height: 844, width: 390 });
    const site = new PublicSitePage(page);
    await site.goto("/en-US");

    await expect(page.locator(".home-service-index")).toContainText(
      "Products",
    );
    expect(await site.hasHorizontalOverflow()).toBe(false);
    await expect(site.customerCenter).toBeVisible();
    await capturePublicScreenshot(page, "public-cloud-visual-en-mobile");
  });
});

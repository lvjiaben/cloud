import { test, expect } from "../support/source-site";
import { NavigationPage } from "../pages/NavigationPage";

test.use({
  baseURL: "http://localhost:6680",
  viewport: { width: 1440, height: 1000 },
  reducedMotion: "reduce",
  headless: true,
});

const category = {
  categoryNo: "badge-category",
  nameZhCN: "测试分类",
  nameEnUS: "Test category",
};
const products = [
  { publicSlug: "server-new", nameZhCN: "入门云主机", nameEnUS: "Basic Cloud", summaryZhCN: "需要显示标识", categoryNo: category.categoryNo, availability: "available" },
  { publicSlug: "server-old", nameZhCN: "标准云主机", nameEnUS: "Standard Cloud", summaryZhCN: "固定显示标识", categoryNo: category.categoryNo, availability: "available" },
  { publicSlug: "server-default", nameZhCN: "高级云主机", nameEnUS: "Advanced Cloud", summaryZhCN: "无需后台标识字段", categoryNo: category.categoryNo, availability: "available" },
  { publicSlug: "server-long", nameZhCN: "湖北襄阳BGP（封海外）面向复杂业务场景的超长名称弹性云服务器", nameEnUS: "Elastic server for demanding workloads", summaryZhCN: "长名称仍完整显示标识", categoryNo: category.categoryNo, availability: "available" },
];

test.describe("TC-100 官网全部商品固定 NEW 标识", () => {
  test.beforeEach(async ({ page }) => {
    await page.route("**/x/idc-portal/api/v1/public/brand", (route) => route.fulfill({
      json: { code: 0, data: { brand: { siteName: "测试官网", navigation: [{ label: "产品与服务", href: "/products.html" }] } } },
    }));
    await page.route("**/x/idc-core/api/v1/public/catalog/products**", async (route) => {
      const url = new URL(route.request().url());
      const slug = url.pathname.split("/").at(-1) || "";
      if (slug && slug !== "products") {
        const product = products.find((item) => item.publicSlug === slug);
        await route.fulfill({ json: { code: 0, data: { product, categories: [category] } } });
        return;
      }
      const categoryNo = url.searchParams.get("categoryNo");
      const rows = categoryNo ? products.filter((item) => item.categoryNo === categoryNo) : products;
      await route.fulfill({ json: { code: 0, data: { categories: [category], products: rows, nextCursor: "" } } });
    });
  });

  test("TC-100a: 横向菜单全部商品固定 NEW，不依赖后台开关", async ({ page }, info) => {
    const site = new NavigationPage(page);
    await site.open("/");
    await site.hoverTitle("产品与服务");
    const panel = site.panel();
    await expect(panel.locator(".Submenu-item")).toHaveCount(4);

    const enabled = panel.locator(".Submenu-item").nth(0);
    await expect(enabled.locator(".Submenu-title")).toHaveText(products[0]!.nameZhCN);
    await expect(enabled.locator(".Submenu-tag")).toHaveText("NEW");
    await expect(enabled.locator(".Submenu-desc")).toHaveText("需要显示标识");
    await expect(enabled).toHaveAttribute("href", "http://localhost:6680/products.html?product=server-new");

    for (const index of [1, 2]) {
      await expect(panel.locator(".Submenu-item").nth(index).locator(".Submenu-tag")).toHaveText("NEW");
    }
    const long = panel.locator(".Submenu-item").nth(3);
    await expect(long.locator(".Submenu-tag")).toHaveText("NEW");
    const alignment = await long.locator(".Submenu-item-title").evaluate((row) => {
      const name = row.querySelector(".Submenu-title")!.getBoundingClientRect();
      const badgeElement = row.querySelector(".Submenu-tag")!;
      const badge = badgeElement.getBoundingClientRect();
      return { badgeCenter: badge.top + badge.height / 2, nameCenter: name.top + name.height / 2, whiteSpace: getComputedStyle(badgeElement).whiteSpace };
    });
    expect(alignment.whiteSpace).toBe("nowrap");
    expect(Math.abs(alignment.badgeCenter - alignment.nameCenter)).toBeLessThanOrEqual(1);
    await page.screenshot({ path: info.outputPath("horizontal-menu.png"), animations: "disabled" });
  });

  test("TC-100b: 左侧及商品卡固定 NEW，手机长名称不溢出", async ({ page }, info) => {
    await page.goto("/products.html?product=server-new");
    await expect(page.locator("[data-products] .product-card .lina-product-new-badge")).toHaveText("NEW");
    const menu = page.locator("[data-categories]");
    const enabled = menu.locator('[data-catalog-product="server-new"]');
    const disabled = menu.locator('[data-catalog-product="server-old"]');
    const missing = menu.locator('[data-catalog-product="server-default"]');
    await expect(enabled.locator(".lina-product-new-badge")).toHaveText("NEW");
    const referenceStyle = await enabled.locator("h5").evaluate((title) => {
      const badge = title.querySelector(".lina-product-new-badge")!;
      const style = getComputedStyle(badge);
      return {
        rowHeight: title.getBoundingClientRect().height,
        rowPadding: `${getComputedStyle(title).paddingTop} ${getComputedStyle(title).paddingBottom}`,
        badgeHeight: badge.getBoundingClientRect().height,
        badgeBackground: style.backgroundColor,
        badgeColor: style.color,
        badgeFontSize: style.fontSize,
      };
    });
    expect(referenceStyle).toEqual({
      rowHeight: 40,
      rowPadding: "0px 0px",
      badgeHeight: 18,
      badgeBackground: "rgb(255, 77, 79)",
      badgeColor: "rgb(255, 255, 255)",
      badgeFontSize: "12px",
    });
    await expect(disabled.locator(".lina-product-new-badge")).toHaveText("NEW");
    await expect(missing.locator(".lina-product-new-badge")).toHaveText("NEW");
    for (const row of [enabled, disabled, missing]) {
      expect(await row.evaluate((link) => link.getBoundingClientRect().height)).toBe(40);
    }
    await page.screenshot({ path: info.outputPath("sidebar-desktop.png"), animations: "disabled" });

    await page.setViewportSize({ width: 390, height: 844 });
    await page.reload();
    const long = menu.locator('[data-catalog-product="server-long"]');
    await expect(long.locator(".lina-product-new-badge")).toHaveText("NEW");
    const geometry = await long.evaluate((link) => {
      const name = link.querySelector(".lina-product-name")!.getBoundingClientRect();
      const badge = link.querySelector(".lina-product-new-badge")!.getBoundingClientRect();
      const row = link.querySelector("h5")!.getBoundingClientRect();
      return { badgeRight: badge.right, linkRight: link.getBoundingClientRect().right, nameRight: name.right, badgeLeft: badge.left, nameHeight: name.height, rowHeight: row.height, badgeCenter: badge.top + badge.height / 2, rowCenter: row.top + row.height / 2, nameWhiteSpace: getComputedStyle(link.querySelector(".lina-product-name")!).whiteSpace, scrollWidth: link.scrollWidth, clientWidth: link.clientWidth };
    });
    expect(geometry.badgeRight).toBeLessThanOrEqual(geometry.linkRight + 1);
    expect(geometry.nameRight).toBeLessThanOrEqual(geometry.badgeLeft);
    expect(geometry.nameHeight).toBeGreaterThan(40);
    expect(geometry.rowHeight).toBeGreaterThan(40);
    expect(Math.abs(geometry.badgeCenter - geometry.rowCenter)).toBeLessThanOrEqual(1);
    expect(geometry.nameWhiteSpace).toBe("normal");
    expect(geometry.scrollWidth).toBeLessThanOrEqual(geometry.clientWidth + 1);
    await page.screenshot({ path: info.outputPath("sidebar-mobile.png"), animations: "disabled" });
  });
});

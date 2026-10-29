import { test, expect } from "../support/fixtures";
test.describe("TC-1 官网展示与响应式交互", () => {
  test("TC-1a: 轮播与分类切换展示对应方案", async ({ site, page }) => {
    await site.open();
    await expect(page.locator("#catalog-products")).toContainText(
      "企业云服务器",
    );
    await site.screenshot("desktop-home");
    await site.slide(1);
    await expect(page.locator("#hero-title")).toContainText("上云更简单");
    await expect(page.locator("#hero-link")).toHaveAttribute(
      "href",
      "/products.html",
    );
    await page.locator("#catalog-tabs").getByRole("link", {name:"云计算",exact:true}).click();
    await expect(page.locator("#catalog-products")).toContainText("企业云服务器");
    await expect(page).toHaveURL(/category=CAT_A/);
    await expect(page.locator(".demo-card")).toHaveCount(0);
    await site.screenshot("category");
  });
  test("TC-1b: 手机菜单可展开关闭且页面无溢出", async ({ site, page }) => {
    await site.mobile();
    await site.open();
    await site.menu();
    await expect(page.locator(".header-tools .login")).toBeVisible();
    await expect(page.locator(".header-tools .login")).toHaveAttribute(
      "href",
      "http://127.0.0.1:6032/auth/login",
    );
    await expect(
      page
        .locator(".header-tools")
        .getByRole("link", { name: "控制台", exact: true }),
    ).toBeVisible();
    await expect(page.locator(".header-tools .register")).toBeVisible();
    await site.group("产品");
    await expect(
      page.locator("#header-nav").getByRole("link", { name: "全部产品" }),
    ).toBeVisible();
    await site.screenshot("mobile-menu");
    await site.key("Escape");
    await expect(page.locator("#header-nav button")).toBeFocused();
    await expect(page.locator("#header-nav button")).toHaveAttribute(
      "aria-expanded",
      "false",
    );
    await site.key("Escape");
    await expect(page.locator(".menu-toggle")).toHaveAttribute(
      "aria-expanded",
      "false",
    );
    await expect(page.locator(".menu-toggle")).toBeFocused();
    await expect(page.locator(".header-tools .login")).toBeHidden();
    await expect
      .poll(() =>
        page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      )
      .toBe(true);
    await site.screenshot("mobile-home");
  });
  test("TC-1c: 服务页使用真实商品并在官网配置", async ({site,page})=>{
    await site.open("/services.html?category=CAT_A");
    await expect(page.getByRole("heading",{level:1})).toHaveText("产品与服务");
    await expect(page.locator("#catalog-products")).toContainText("企业云服务器");
    await expect(page.locator(".live-product").first().getByRole("link")).toHaveAttribute("href","/configure.html?product=cloud-one");
    await expect(page.locator("main")).not.toContainText("799");
    await site.screenshot("service-catalog");
  });
  test("TC-1d: 节点切换与帮助详情可读", async ({ site, page }) => {
    await site.open("/datacenters.html");
    await site.location("美国 · 洛杉矶");
    await expect(page.locator("#location-detail")).toContainText(
      "北美业务场景",
    );
    await site.screenshot("datacenters");
    await site.open("/help.html");
    await site.question("产品价格如何确定？");
    await expect(page.locator("details[open]")).toContainText("确认订单时的报价");
  });
  test("TC-1e: 全部声明页面及移动布局可访问", async ({ site, page }) => {
    await site.mobile();
    for (const path of [
      "products",
      "services",
      "datacenters",
      "news",
      "announcements",
      "knowledge",
      "downloads",
      "help",
      "about",
      "contact",
    ]) {
      await site.open(`/${path}.html`);
      await expect(page.locator("h1")).toHaveCount(1);
      await expect
        .poll(() =>
          page.evaluate(
            () => document.documentElement.scrollWidth <= window.innerWidth,
          ),
        )
        .toBe(true);
    }
    await expect(page.locator("main [data-phone]")).toHaveText("400-000-1234");
    await site.screenshot("mobile-contact");
  });
});

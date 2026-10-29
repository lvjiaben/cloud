import { test, expect, brand } from "../support/fixtures";

test.describe("TC-6 导航缺省与轮播自动播放", () => {
  test("TC-6a: 空导航品牌先返回仍保留菜单及真实分类", async ({ site, page }) => {
    let releaseCatalog!: () => void;
    const gate = new Promise<void>((resolve) => { releaseCatalog = resolve; });
    await page.route("**/catalog/products?**", async (route) => {
      await gate;
      await route.fallback();
    });
    await page.route("**/public/brand", (route) => route.fulfill({json: {
      code: 0, data: { brand: { ...brand, navigation: [] } },
    }}));
    await site.open();
    await expect(page.locator(".brand [data-brand]")).toHaveText("星河云");
    await expect(page.locator("#header-nav").getByRole("button", { name: "产品", exact: true })).toBeVisible();
    releaseCatalog();
    await expect(page.locator("#catalog-products")).toContainText("企业云服务器");
    await site.group("产品");
    await expect(page.locator("#header-nav").getByRole("link", { name: "云计算", exact: true })).toBeVisible();
    await site.screenshot("restored-desktop-navigation");
    await site.navigationLink("云计算");
    await expect(page).toHaveURL(/services.html\?category=CAT_A/);
    await expect(page.locator("#catalog-category")).toHaveValue("CAT_A");
    await expect(page.locator("#catalog-products")).toContainText("企业云服务器");
  });

  test("TC-6b: 分类先返回且品牌稍后加载，手机英文菜单仍完整", async ({ site, page }) => {
    let releaseBrand!: () => void;
    const gate = new Promise<void>((resolve) => { releaseBrand = resolve; });
    await page.route("**/public/brand", async (route) => {
      await gate;
      await route.fulfill({json: { code: 0, data: { brand: { ...brand, navigation: [] } } }});
    });
    await page.route("**/catalog/products?**", (route) => route.fulfill({json: {
      code: 0, data: { products: [], categories: [{categoryNo: "CAT_EN", nameZhCN: "云计算", nameEnUS: "Cloud computing"}] },
    }}));
    await site.mobile();
    await site.open("/contact.html?lang=en-US");
    await expect(page.locator("#footer-categories")).toContainText("Cloud computing");
    releaseBrand();
    await expect(page.locator(".brand [data-brand]")).toHaveText("星河云");
    await site.menu();
    await site.group("Products");
    await expect(page.locator("#header-nav").getByRole("link", { name: "Cloud computing", exact: true })).toBeVisible();
    await expect(page.locator("#header-nav").getByRole("link", { name: "Data centers", exact: true })).toBeVisible();
    await site.screenshot("restored-mobile-english-navigation");
    await site.navigationLink("Cloud computing");
    await expect(page).toHaveURL(/category=CAT_EN&lang=en-US/);
    await expect(page.locator("h1")).toHaveText("Products and services");
  });

  test("TC-6c: 分类失败时基础菜单可用，重试后补齐真实分类", async ({ site, page }) => {
    let failed = true;
    await page.route("**/public/brand", (route) => route.fulfill({json: {code: 0, data: {brand: {...brand, navigation: []}}}}));
    await page.route("**/catalog/products?**", (route) => failed
      ? route.fulfill({status: 503, json: {code: 503}}) : route.fallback());
    await site.open();
    await expect(page.locator("#catalog-status")).toContainText("暂时无法加载");
    await site.group("产品");
    await expect(page.locator("#header-nav").getByRole("link", {name: "全部产品", exact: true})).toBeVisible();
    await expect(page.locator("#header-nav")).not.toContainText("云计算");
    failed = false;
    await site.retry("catalog-status");
    await expect(page.locator("#catalog-products")).toContainText("企业云服务器");
    await site.group("产品");
    await expect(page.locator("#header-nav").getByRole("link", {name: "云计算", exact: true})).toBeVisible();
  });

  test("TC-6d: 鼠标停在横幅时自动轮播，手动切换后继续且没有播放按钮", async ({site, page}) => {
    await page.emulateMedia({reducedMotion: "no-preference"});
    await page.clock.install({ time: new Date("2026-10-23T00:00:00Z") });
    await page.clock.pauseAt(new Date("2026-10-23T00:00:01Z"));
    await site.open();
    await expect(page.locator(".brand [data-brand]")).toHaveText("星河云");
    await site.hoverHero();
    await page.clock.runFor(6500);
    await expect(page.locator('[data-slide="1"]')).toHaveAttribute("aria-pressed", "true");
    await expect(page.locator("#hero-title")).toContainText("上云更简单");
    await site.nextSlide();
    await expect(page.locator('[data-slide="2"]')).toHaveAttribute("aria-pressed", "true");
    await page.clock.runFor(6500);
    await expect(page.locator('[data-slide="3"]')).toHaveAttribute("aria-pressed", "true");
    await page.clock.runFor(6500);
    await expect(page.locator('[data-slide="0"]')).toHaveAttribute("aria-pressed", "true");
    await expect(page.locator(".pause")).toHaveCount(0);
    await page.clock.runFor(6500);
    await expect(page.locator('[data-slide="1"]')).toHaveAttribute("aria-pressed", "true");
    await site.screenshot("autoplay-resumed");
  });

  test("TC-6e: 减少动态效果时手机仍定时切换且手动切换不遮挡标题", async ({site, page}) => {
    await site.mobile();
    await page.clock.install({ time: new Date("2026-10-23T00:00:00Z") });
    await page.clock.pauseAt(new Date("2026-10-23T00:00:01Z"));
    await site.open();
    await expect(page.locator(".brand [data-brand]")).toHaveText("星河云");
    await page.clock.runFor(6500);
    await expect(page.locator('[data-slide="1"]')).toHaveAttribute("aria-pressed", "true");
    await site.nextSlide();
    await expect(page.locator('[data-slide="2"]')).toHaveAttribute("aria-pressed", "true");
    await expect.poll(() => page.evaluate(() => {
      const hero = document.querySelector(".hero")!;
      const title = document.querySelector("#hero-title")!.getBoundingClientRect();
      const header = document.querySelector(".site-header")!.getBoundingClientRect();
      return hero.scrollTop === 0 && title.top >= header.bottom;
    })).toBe(true);
    await site.screenshot("mobile-autoplay");
  });
  test("TC-6f: 小屏桌面英文菜单与客户入口均在可视范围内", async ({site, page}) => {
    await page.route("**/public/brand", (route) => route.fulfill({json: {code: 0, data: {brand: {...brand, navigation: []}}}}));
    for (const width of [1024, 1200, 1280]) {
      await page.setViewportSize({width, height: 900});
      await site.open("/?lang=en-US");
      await expect(page.locator(".brand [data-brand]")).toHaveText("星河云");
      await expect(page.locator("#catalog-products")).toContainText("企业云服务器");
      if (width <= 1200) await site.menu();
      await expect(page.locator("#header-nav").getByRole("button", {name: "Products", exact: true})).toBeVisible();
      await expect(page.locator(".header-tools .register")).toBeInViewport();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    }
    await site.screenshot("small-desktop-english-navigation");
  });
  test("TC-6g: 键盘阅读轮播链接时暂停，移出后恢复", async ({site, page}) => {
    await page.clock.install({ time: new Date("2026-10-23T00:00:00Z") });
    await page.clock.pauseAt(new Date("2026-10-23T00:00:01Z"));
    await site.open();
    await expect(page.locator(".brand [data-brand]")).toHaveText("星河云");
    await site.focusHeroWithKeyboard();
    await page.clock.runFor(6500);
    await expect(page.locator('[data-slide="0"]')).toHaveAttribute("aria-pressed", "true");
    await site.focusLanguage();
    await page.clock.runFor(6500);
    await expect(page.locator('[data-slide="1"]')).toHaveAttribute("aria-pressed", "true");
  });

});

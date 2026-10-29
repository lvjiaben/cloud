import { test, expect, brand, products, article } from "../support/fixtures";
test.describe("TC-2 公开数据接入与异常恢复", () => {
  test("TC-2i: 有跳转目标的父导航仍能独立展开子项", async ({ site, page }) => {
    await page.route("**/public/brand", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            brand: {
              ...brand,
              navigation: [
                {
                  label: { zhCN: "产品目录" },
                  href: "/products.html",
                  children: [
                    { label: { zhCN: "服务方案" }, href: "/services.html" },
                  ],
                },
              ],
            },
          },
        },
      }),
    );
    await site.mobile();
    await site.open();
    await site.menu();
    await expect(
      page.locator("#header-nav").getByRole("link", { name: "服务方案" }),
    ).toBeHidden();
    await site.group("展开产品目录");
    await expect(
      page.locator("#header-nav").getByRole("link", { name: "服务方案" }),
    ).toBeVisible();
    await expect(
      page.locator("#header-nav").getByRole("link", { name: "产品目录" }),
    ).toHaveAttribute("href", "/products.html");
    await site.key("Escape");
    await expect(
      page.locator("#header-nav").getByRole("link", { name: "服务方案" }),
    ).toBeHidden();
  });
  test("TC-2a: 品牌导航与真实商品跳转", async ({ site, page }) => {
    await site.open("/products.html");
    await expect(page.locator(".brand")).toContainText("星河云");
    await expect(page.locator("#catalog-products")).toContainText(
      "企业云服务器",
    );
    await expect(
      page.locator(".live-product").first().getByRole("link"),
    ).toHaveAttribute(
      "href",
      "/configure.html?product=cloud-one",
    );
    await expect(page.locator("#footer-nav")).toHaveText("技术文档");
    await expect(page.locator("[data-default-footer]").first()).toBeHidden();
    await expect(page.locator("#catalog-products")).not.toContainText("799");
  });
  test("TC-2b: 商品服务端筛选分页及失败重试", async ({ site, page }) => {
    let fail = true;
    const seen: string[] = [];
    await page.route("**/catalog/products?**", async (route) => {
      const url = new URL(route.request().url());
      seen.push(url.search);
      if (fail) return route.fulfill({ status: 503, json: { code: 503 } });
      const next = url.searchParams.has("cursor");
      await route.fulfill({
        json: {
          code: 0,
          data: {
            products: [products[next ? 1 : 0]],
            categories: [{ categoryNo: "CAT_A", nameZhCN: "云计算" }],
            nextCursor: next ? "" : "NEXT",
          },
        },
      });
    });
    await site.open("/products.html");
    await expect(page.locator("#catalog-status")).toContainText("暂时无法加载");
    await expect(page.locator(".live-product")).toHaveCount(0);
    await site.screenshot("catalog-error");
    fail = false;
    await site.retry("catalog-status");
    await expect(page.locator(".live-product")).toHaveCount(1);
    await site.moreProducts();
    await expect(page.locator(".live-product")).toHaveCount(2);
    expect(seen.some((q) => q.includes("cursor=NEXT"))).toBe(true);
    await site.productSearch("企业");
    await expect
      .poll(() =>
        seen.some((q) => new URLSearchParams(q).get("namePrefix") === "企业"),
      )
      .toBe(true);
    await expect(page.locator(".live-product")).toHaveCount(1);
    await site.screenshot("catalog-filtered");
  });
  test("TC-2c: 资讯列表搜索分页与详情正文", async ({ site, page }) => {
    const queries: string[] = [];
    await page.route("**/content/items?**", async (route) => {
      const url = new URL(route.request().url());
      queries.push(url.search);
      await route.fulfill({
        json: {
          code: 0,
          data: {
            items: [article],
            nextCursor: url.searchParams.has("cursor") ? "" : "MORE",
          },
        },
      });
    });
    await site.open("/news.html");
    await expect(page.locator(".content-row")).toHaveCount(1);
    await site.moreContent();
    await expect(page.locator(".content-row")).toHaveCount(2);
    expect(queries.some((q) => q.includes("cursor=MORE"))).toBe(true);
    await site.contentSearch("升级");
    await expect
      .poll(() =>
        queries.some((q) => new URLSearchParams(q).get("query") === "升级"),
      )
      .toBe(true);
    await site.open("/article.html?kind=news&id=CT_001");
    await expect(page.locator("#article-title")).toHaveText(article.title);
    await expect(page.locator("#article-body")).toHaveText(article.body);
    await expect(page.locator("#download")).toBeHidden();
    await site.screenshot("article");
  });
  test("TC-2d: 空导航使用默认入口且不输出危险链接", async ({ site, page }) => {
    await page.route("**/public/brand", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            brand: {
              ...brand,
              navigation: [],
              footerNavigation: [
                {
                  label: { zhCN: "危险链接" },
                  external: true,
                  href: "javascript:alert(1)",
                },
                {
                  label: { zhCN: "危险客户链接" },
                  customer: true,
                  href: "javascript:alert(1)",
                },
              ],
            },
          },
        },
      }),
    );
    await site.open();
    await expect(page.locator("#header-nav").getByRole("button", { name: "产品", exact: true })).toBeVisible();
    await expect(page.locator("#footer-nav a")).toHaveCount(0);
  });
  test("TC-2e: 内容失败重试和空态不伪造文章", async ({ site, page }) => {
    let fail = true;
    await page.route("**/content/items?**", (route) =>
      route.fulfill(
        fail
          ? { status: 503, json: { code: 503 } }
          : { json: { code: 0, data: { items: [], nextCursor: "" } } },
      ),
    );
    await site.open("/news.html");
    await expect(page.locator("#content-status")).toContainText("暂时无法加载");
    await expect(page.locator(".content-row")).toHaveCount(0);
    await site.screenshot("content-error");
    fail = false;
    await site.retry("content-status");
    await expect(page.locator("#content-status")).toContainText("暂无相关内容");
    await expect(page.locator("#content-more")).toBeHidden();
  });
  test("TC-2f: 下载票据失败后可恢复且文件来自公开接口", async ({
    site,
    page,
  }) => {
    let fail = true;
    let fileFail = true;
    await page.route("**/content/items/DL_001", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            item: {
              ...article,
              type: "download",
              contentNo: "DL_001",
              currentFileNo: "FILE_001",
            },
          },
        },
      }),
    );
    await page.route("**/content/items/DL_001/download-tickets", (route) =>
      route.fulfill(
        fail
          ? { status: 503, json: { code: 503 } }
          : { json: { code: 0, data: { token: "TOKEN_001" } } },
      ),
    );
    await page.route("**/content/downloads/TOKEN_001", (route) =>
      route.fulfill(
        fileFail
          ? { status: 503, json: { code: 503 } }
          : {
              status: 200,
              contentType: "application/octet-stream",
              headers: {
                "content-disposition": 'attachment; filename="guide.txt"',
              },
              body: "published download",
            },
      ),
    );
    await site.open("/article.html?kind=downloads&id=DL_001");
    await expect(page.locator("#download")).toBeVisible();
    await site.download();
    await expect(page.locator("#article-status")).toContainText("暂时无法下载");
    await site.screenshot("download-error");
    fail = false;
    await site.retry("article-status");
    await expect(page.locator("#article-status")).toContainText("暂时无法下载");
    await expect(page).toHaveURL(/article\.html/u);
    await expect(page.locator("#download")).toBeEnabled();
    await site.screenshot("download-file-error");
    fileFail = false;
    const downloaded = page.waitForEvent("download");
    await site.retry("article-status");
    expect((await downloaded).suggestedFilename()).toBe("guide.txt");
  });
  test("TC-2g: 不存在的详情与跨类型详情不展示错误内容", async ({
    site,
    page,
  }) => {
    await site.open("/article.html?kind=news&id=MISSING");
    await expect(page.locator("#article-status")).toContainText(
      "内容不存在或已下架",
    );
    await expect(page.locator("#article")).toBeHidden();
    await site.open("/article.html?kind=knowledge&id=CT_001");
    await expect(page.locator("#article-status")).toContainText(
      "内容不存在或已下架",
    );
    await expect(page.locator("#article")).toBeHidden();
  });
  test("TC-2h: 模板独立标题及接口文本安全显示", async ({ site, page }) => {
    await page.route("**/public/brand", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            brand: {
              ...brand,
              heroTitle: { zhCN: "为企业连接未来" },
              heroDescription: { zhCN: "定制首页介绍" },
            },
          },
        },
      }),
    );
    await page.route("**/catalog/products?**", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: {
            products: [
              { ...products[0], nameZhCN: "<img src=x onerror=alert(1)>" },
            ],
            categories: [],
            nextCursor: "",
          },
        },
      }),
    );
    await site.open();
    await expect(page.locator("#hero-title")).toHaveText("为企业连接未来");
    await expect(page.locator("#hero-description")).toHaveText("定制首页介绍");
    await expect(page.locator("#catalog-products")).toContainText(
      "<img src=x onerror=alert(1)>",
    );
    await expect(page.locator("#catalog-products img")).toHaveCount(0);
  });
});

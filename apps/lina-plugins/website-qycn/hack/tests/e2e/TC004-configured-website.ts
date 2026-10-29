import { test, expect, brand } from "../support/fixtures";
test.describe("TC-4 双语、公共资料与真实分类", () => {
  test("TC-4a: 公共配置覆盖品牌、公司、备案与伙伴，英文跨页保持", async ({
    site,
    page,
  }) => {
    let current = {
      ...brand,
      siteName: { zhCN: "星河云", enUS: "Galaxy Cloud" },
      companyName: "星河有限公司",
      logoUrl: "/media/world-map.jpg",
      icpNumber: "粤ICP备123号",
      icpUrl: "https://records.example/icp",
      policeNumber: "粤公网安备456号",
      policeUrl: "https://records.example/police",
      templateConfig: {
        partners: "真实伙伴一\n真实伙伴二",
        partnersen: "Partner One\nPartner Two",
      },
      navigation: [
        { label: { zhCN: "产品", enUS: "Products" }, href: "/products.html" },
      ],
    };
    await page.route("**/public/brand", (route) =>
      route.fulfill({ json: { code: 0, data: { brand: current } } }),
    );
    await site.open();
    await expect(page.locator(".brand")).toContainText("星河云");
    await expect(page.locator(".brand img")).toHaveAttribute(
      "src",
      "/media/world-map.jpg",
    );
    await expect(page.locator("[data-company]")).toHaveText("星河有限公司");
    await expect(
      page.getByRole("link", { name: "粤ICP备123号" }),
    ).toHaveAttribute("href", "https://records.example/icp");
    await expect(
      page.getByRole("link", { name: "粤公网安备456号" }),
    ).toHaveAttribute("rel", "noopener noreferrer");
    await expect(page.locator("#partner-grid")).toHaveText(
      "真实伙伴一真实伙伴二",
    );
    await page.getByLabel("Language", { exact: true }).selectOption("en-US");
    await expect(page.locator("html")).toHaveAttribute("lang", "en-US");
    await expect(page.locator(".brand")).toContainText("Galaxy Cloud");
    await expect(page.locator("#partner-grid")).toHaveText(
      "Partner OnePartner Two",
    );
    await page
      .locator("#header-nav")
      .getByRole("link", { name: "Products", exact: true })
      .click();
    await expect(page).toHaveURL(/lang=en-US/);
    await expect(
      page.getByRole("button", { name: "Search products", exact: true }),
    ).toBeVisible();
    await expect(page.getByPlaceholder("Enter product name")).toBeVisible();
    await site.screenshot("english-catalog");
    current = {
      ...current,
      companyName: "新公司",
      icpUrl: "javascript:alert(1)",
      templateConfig: { partners: "", partnersen: "" },
    };
    await site.open("/?lang=en-US");
    await expect(page.locator("[data-company]")).toHaveText("新公司");
    await expect(page.locator("#partners")).toBeHidden();
    await expect(page.locator("#registration-links a")).toHaveCount(1);
  });
  test("TC-4b: 分类及双语商品由接口决定，错误不回退假商品", async ({
    site,
    page,
  }) => {
    const queries: string[] = [];
    let failed = false;
    await page.route("**/catalog/products?**", (route) => {
      const url = new URL(route.request().url());
      queries.push(url.search);
      if (failed) return route.fulfill({ status: 503, json: { code: 503 } });
      const category = url.searchParams.get("categoryNo") || "CAT_A";
      return route.fulfill({
        json: {
          code: 0,
          data: {
            products: [
              {
                publicSlug: category,
                nameZhCN: category === "CAT_A" ? "安全商品" : "计算商品",
                nameEnUS:
                  category === "CAT_A" ? "Security product" : "Compute product",
                availability: "available",
              },
            ],
            categories: [
              {
                categoryNo: "CAT_A",
                nameZhCN: "安全产品",
                nameEnUS: "Security",
              },
              {
                categoryNo: "CAT_B",
                nameZhCN: "计算产品",
                nameEnUS: "Compute",
              },
            ],
            nextCursor: "",
          },
        },
      });
    });
    await site.open("/services.html?lang=en-US");
    await page
      .locator("#catalog-tabs")
      .getByRole("link", { name: "Compute", exact: true })
      .click();
    await expect(page.locator("#catalog-products")).toHaveText(
      /Compute product/,
    );
    expect(queries.some((query) => query.includes("categoryNo=CAT_B"))).toBe(
      true,
    );
    failed = true;
    await page
      .locator("#catalog-tabs")
      .getByRole("link", { name: "Security", exact: true })
      .click();
    await expect(page.locator("#catalog-status")).toContainText(
      "Products could not be loaded. Please retry.",
    );
    await expect(page.locator(".live-product")).toHaveCount(0);
    await expect(page.locator("main")).not.toContainText("799");
    await site.screenshot("english-catalog-error");
  });
});

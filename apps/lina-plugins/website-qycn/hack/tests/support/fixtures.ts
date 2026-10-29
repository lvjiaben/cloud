// Test-owned anonymous fixtures are never shipped to the public website.
import { test as base, expect } from "@playwright/test";
import { WebsitePage } from "../pages/WebsitePage";
export const brand = {
  siteName: { zhCN: "星河云" },
  footer: { zhCN: "星河科技" },
  seoTitle: { zhCN: "星河云官方网站" },
  contactPhone: "400-000-1234",
  contactEmail: "support@example.test",
  templateEntry: { assetBase: "/x-assets/website-qycn/v0.1.13/" },
  navigation: [
    {
      label: { zhCN: "产品" },
      group: true,
      children: [{ label: { zhCN: "全部产品" }, href: "/products.html" }],
    },
    { label: { zhCN: "技术资讯" }, href: "/knowledge.html" },
    { label: { zhCN: "联系我们" }, href: "/contact.html" },
  ],
  footerNavigation: [{ label: { zhCN: "技术文档" }, href: "/knowledge.html" }],
};
export const products = [
  {
    publicSlug: "cloud-one",
    nameZhCN: "企业云服务器",
    summaryZhCN: "真实目录中的企业算力",
    categoryNo: "CAT_A",
    categoryNameZhCN: "云计算",
    availability: "available",
  },
  {
    publicSlug: "cloud-two",
    nameZhCN: "待确认服务器",
    summaryZhCN: "需要人工确认配置",
    categoryNo: "CAT_A",
    categoryNameZhCN: "云计算",
    availability: "manual_confirmation",
  },
];
export const article = {
  contentNo: "CT_001",
  type: "news",
  title: "云产品服务升级公告",
  summary: "产品服务升级说明",
  body: "第一段：服务升级内容。\n第二段：查看最新产品配置。",
  publishedAt: 1792598400000,
};
export const test = base.extend<{ site: WebsitePage }>({
  site: async ({ page }, use) => {
    await page.route("**/x/**", async (route) => {
      const url = new URL(route.request().url());
      let data: unknown;
      if (url.pathname.endsWith("/public/brand")) data = { brand };
      else if (url.pathname.endsWith("/catalog/products"))
        data = {
          products,
          categories: [{ categoryNo: "CAT_A", nameZhCN: "云计算" }],
          nextCursor: "",
        };
      else if (url.pathname.endsWith("/content/items"))
        data = {
          items: url.searchParams.get("type") === "news" ? [article] : [],
          nextCursor: "",
        };
      else if (url.pathname.endsWith("/content/items/CT_001"))
        data = { item: article };
      else return route.fulfill({ status: 404, json: { code: 404 } });
      await route.fulfill({ json: { code: 0, data } });
    });
    await use(new WebsitePage(page));
  },
});
export { expect };

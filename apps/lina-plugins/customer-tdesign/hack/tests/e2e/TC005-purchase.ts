import { test, expect } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-5 官网选购衔接", () => {
  test("TC-5a 配置深链接与购物车交给配置的官网，凭据不进入 URL", async ({consolePage:page}) => {
    const posts: URLSearchParams[] = [];
    await page.route("https://website.example/__website/session", async r => {
      expect(r.request().method()).toBe("POST");
      expect(r.request().url()).not.toContain("isolated-ui-fixture");
      posts.push(new URLSearchParams(r.request().postData() || ""));
      await r.fulfill({contentType:"text/html; charset=utf-8",body:"<h1>官网选购</h1>"});
    });
    const view = new CustomerPage(page);
    await view.open("/account/products/cloud-one/configure?currency=USD&quoteItemNo=QI-1");
    await expect(page.getByRole("heading",{name:"官网选购"})).toBeVisible();
    expect(posts[0].get("returnTo")).toBe("/configure.html?product=cloud-one&currency=USD&quoteItemNo=QI-1");
    expect(posts[0].get("accessToken")).toBe("isolated-ui-fixture");
    await view.open("/account/cart");
    await expect(page.getByRole("heading",{name:"官网选购"})).toBeVisible();
    expect(posts[1].get("returnTo")).toBe("/cart.html");
  });
  test("TC-5b 官网未配置明确提示，不回退到中心购物车",async({consolePage:page})=>{
    await page.route("**/public/customer-center/configuration",r=>r.fulfill({json:{code:0,data:{selectedPlugin:"customer-tdesign",config:{}}}}));
    const view=new CustomerPage(page);
    await view.open("/account/cart");
    await expect(page.getByText("官网入口尚未配置，请联系商家。",{exact:true})).toBeVisible();
    await expect(page.getByRole("button",{name:"确认下单"})).toHaveCount(0);
    await view.screenshot("website-unconfigured");
  });
});

import { test, expect } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-1 导航与自适应", () => {
  test("TC-1a 刷新只出现完整客户菜单，手机可打开导航", async ({
    consolePage: page,
  }) => {
    let release: () => void = () => {};
    const pending = new Promise<void>((r) => (release = r));
    await page.route("**/customer/external-menus", async (r) => {
      await pending;
      await r.fulfill({ json: { code: 0, data: { items: [] } } });
    });
    const view = new CustomerPage(page);
    await view.open();
    await expect(page.getByText("正在加载菜单")).toBeVisible();
    await expect(
      page.getByRole("navigation").getByText("账户资料"),
    ).toHaveCount(0);
    await expect(page.getByText("系统管理", { exact: true })).toHaveCount(0);
    release();
    await expect(
      page.getByRole("navigation").getByText("账户资料"),
    ).toBeVisible();
    await expect(page.getByRole("heading", { name: "账户概览" })).toBeVisible();
    await expect(view.main).toContainText("CNY 520.00");
    await view.screenshot("overview-desktop");
    await page.reload();
    await expect(
      page.getByRole("navigation").getByText("账户资料"),
    ).toBeVisible();
    await page.setViewportSize({ width: 390, height: 844 });
    await expect
      .poll(() =>
        page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      )
      .toBe(true);
    await view.screenshot("overview-mobile");
    await view.button("打开菜单").click();
    await expect(
      page.locator(".t-drawer").getByRole("link", { name: "账户资料" }),
    ).toBeVisible();
    await view.screenshot("mobile-menu");
  });
  test("TC-1b 菜单读取失败可重试，不显示旧菜单", async ({
    consolePage: page,
  }) => {
    let failing = true;
    await page.route("**/customer/external-menus", (r) =>
      r.fulfill(
        failing
          ? { status: 503, json: { code: 503 } }
          : { json: { code: 0, data: { items: [] } } },
      ),
    );
    const view = new CustomerPage(page);
    await view.open();
    await expect(page.getByText("菜单加载失败")).toBeVisible();
    await expect(
      page.getByRole("navigation").getByText("账户资料"),
    ).toHaveCount(0);
    await view.screenshot("navigation-error");
    failing = false;
    await page
      .getByRole("navigation")
      .getByRole("button", { name: "重试" })
      .click();
    await expect(
      page.getByRole("navigation").getByText("账户资料"),
    ).toBeVisible();
  });
  test("TC-1c 一级标题与二级菜单由后台数据决定，刷新后按新名称和顺序显示",async({consolePage:page})=>{
    let title="产品与服务";
    await page.route("**/account/product-directories?*",route=>route.fulfill({json:{code:0,data:{total:5,items:[
      {directoryNo:"overview",nameZhCn:"总览",navigationType:"system",targetUrl:"/account",sortOrder:1,icon:"lucide:layout-dashboard"},
      {directoryNo:"services",nameZhCn:title,navigationType:"system",targetUrl:"/account/servers",sortOrder:2,icon:"lucide:cloud"},
      {directoryNo:"cloud",nameZhCn:"云服务器",navigationType:"product",parentDirectoryNo:"services",sortOrder:20,icon:"lucide:server"},
      {directoryNo:"finance",nameZhCn:"费用",navigationType:"system",targetUrl:"/account/billing",sortOrder:3,icon:"lucide:wallet"},
      {directoryNo:"orders",nameZhCn:"订单",navigationType:"system",targetUrl:"/account/orders",parentDirectoryNo:"finance",sortOrder:10,icon:"lucide:file-text"},
    ]}}}));
    const view=new CustomerPage(page);await view.open();
    const nav=page.getByRole("navigation",{name:"主导航"});
    await expect(nav.locator(".nav-section-title")).toHaveText(["产品与服务","费用"]);
    await expect(nav.getByRole("link",{name:"云服务器",exact:true})).toBeVisible();
    await expect(nav.getByRole("link",{name:"订单",exact:true})).toBeVisible();
    await expect(nav.getByRole("button")).toHaveCount(0);
    await view.screenshot("shared-menu-tree");
    title="商家产品分组";await view.reload();
    await expect(nav.locator(".nav-section-title").first()).toHaveText("商家产品分组");
    await expect(nav).not.toContainText("产品与服务");await expect(nav.getByRole("link",{name:"云服务器",exact:true})).toBeVisible();
  });

});

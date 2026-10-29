import { test, expect } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-10 官网登录边界", () => {
  test("TC-10a 只接受配置的官网与允许返回路径", async ({
    consolePage: page,
  }) => {
    let sent = 0;
    await page.route("https://**/__website/session", (r) => {
      sent++;
      return r.fulfill({ status: 204 });
    });
    const view = new CustomerPage(page);
    await view.open(
      "/auth/website-session?websiteOrigin=https://evil.example&probe=1&returnTo=/cart.html",
    );
    await expect(
      page.getByText("官网来源不匹配。", { exact: true }),
    ).toBeVisible();
    await view.open(
      "/auth/website-session?websiteOrigin=https://website.example&probe=1&returnTo=//evil.example/cart.html",
    );
    await expect(
      page.getByText("返回地址不可用。", { exact: true }),
    ).toBeVisible();
    expect(sent).toBe(0);
  });
  test("TC-10b 未登录保留官网返回意图，登录后安全交接", async ({
    apiPage: page,
  }) => {
    await page.route(
      "**/x/idc-notify/api/v1/public/account/password-logins",
      (r) =>
        r.fulfill({
          json: { code: 0, data: { accessToken: "signed-in-test" } },
        }),
    );
    await page.route("https://website.example/__website/session", (r) => {
      const body = new URLSearchParams(r.request().postData() || "");
      expect(body.get("accessToken")).toBe("signed-in-test");
      expect(body.get("returnTo")).toBe("/cart.html");
      return r.fulfill({
        contentType: "text/html; charset=utf-8",
        body: "<h1>官网购物车</h1>",
      });
    });
    const view = new CustomerPage(page);
    await view.open(
      "/auth/website-session?websiteOrigin=https://website.example&login=1&returnTo=/cart.html",
    );
    await expect(
      page.getByRole("heading", { name: "登录用户中心" }),
    ).toBeVisible();
    await page.getByLabel("账号", { exact: true }).fill("test");
    await page.getByLabel("密码", { exact: true }).fill("test-password");
    await view.button("登录用户中心").click();
    await expect(
      page.getByRole("heading", { name: "官网购物车" }),
    ).toBeVisible();
  });
});

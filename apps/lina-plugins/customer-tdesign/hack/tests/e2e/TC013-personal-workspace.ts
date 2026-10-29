import { test, expect } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-13 个人资料与充值入口", () => {
  test("TC-13a 资料编辑失败保留，保存重读，头像同步顶栏", async ({
    consolePage: page,
  }) => {
    let profile = {
      username: "account-test",
      nickname: "原昵称",
      phone: "13800138000",
      email: "old@example.com",
      avatar: "",
    };
    let fail = true,
      uploads = 0;
    await page.route("**/api/v1/user/profile", (r) => {
      if (r.request().method() === "PUT") {
        if (fail)
          return r.fulfill({
            status: 503,
            json: { code: 503, message: "资料保存失败，请重试" },
          });
        expect(r.request().postDataJSON()).toEqual({ nickname: "新昵称" });
        profile = { ...profile, ...r.request().postDataJSON() };
      }
      return r.fulfill({ json: { code: 0, data: profile } });
    });
    await page.route("**/api/v1/user/info", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            username: profile.username,
            realName: profile.nickname,
            avatar: profile.avatar,
          },
        },
      }),
    );
    await page.route(
      "**/x/idc-core/api/v1/account/customer/avatar-uploads",
      (r) => {
        uploads++;
        expect(r.request().postData()).not.toContain('name="scene"');
        return r.fulfill({
          json: { code: 0, data: { url: "/api/v1/uploads/avatar-test.png" } },
        });
      },
    );
    await page.route("**/api/v1/user/profile/avatar", (r) => {
      profile.avatar = r.request().postDataJSON().avatar;
      return r.fulfill({ json: { code: 0, data: {} } });
    });
    await page.route("**/api/v1/uploads/avatar-test.png", (r) =>
      r.fulfill({
        contentType: "image/png",
        body: Buffer.from(
          "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+a9S8AAAAASUVORK5CYII=",
          "base64",
        ),
      }),
    );
    await page.route("**/account/customer/profile", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            item: {
              displayName: "测试客户",
              customerCode: 10001,
              type: "individual",
              status: "active",
              countryCode: "CN",
              preferredLocale: "zh-CN",
              version: 1,
            },
          },
        },
      }),
    );
    await page.route("**/account/customer/contacts", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/profile");
    await view.button("编辑个人资料").click();
    const dialog = page.getByRole("dialog", { name: "编辑个人资料" });
    await dialog
      .getByRole("textbox", { name: "昵称", exact: true })
      .fill("新昵称");
    await view.button("保存个人资料").click();
    await expect(dialog.getByText("资料保存失败，请重试")).toBeVisible();
    await expect(
      dialog.getByRole("textbox", { name: "昵称", exact: true }),
    ).toHaveValue("新昵称");
    await expect(
      dialog.getByRole("textbox", { name: "电子邮箱", exact: true }),
    ).toHaveCount(0);
    await expect(
      dialog.getByRole("textbox", { name: "手机号", exact: true }),
    ).toHaveCount(0);
    fail = false;
    await view.button("保存个人资料").click();
    await expect(dialog).toHaveCount(0);
    await expect(view.main).toContainText("ol***@example.com");
    await expect(view.main).toContainText("138 **** 8000");
    await page
      .locator('.profile-avatar input[type="file"]')
      .setInputFiles({
        name: "avatar.png",
        mimeType: "image/png",
        buffer: Buffer.from("avatar-test"),
      });
    await expect(page.locator(".avatar-button img")).toHaveAttribute(
      "src",
      "/api/v1/uploads/avatar-test.png",
    );
    expect(uploads).toBe(1);
    await expect(page.locator(".account-name")).toHaveText("新昵称");
    expect(
      await page.locator(".avatar-button").evaluate((button) => {
        const a = button.querySelector(".t-avatar")!.getBoundingClientRect(),
          n = button.querySelector(".account-name")!.getBoundingClientRect();
        return n.left - a.right;
      }),
    ).toBeGreaterThanOrEqual(8);
    await view.screenshot("personal-profile-desktop");
    await page.setViewportSize({ width: 390, height: 844 });
    await expect
      .poll(() =>
        page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      )
      .toBe(true);
    await view.screenshot("personal-profile-mobile");
  });
  test("TC-13b 概览标记不可充值仍有入口，无渠道明确说明并可反复打开", async ({
    consolePage: page,
  }) => {
    await page.route("**/billing/overview", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            overview: {
              balances: [],
              openInvoiceCount: 0,
              openReceivables: [],
              rechargeAvailable: false,
            },
          },
        },
      }),
    );
    await page.route("**/account/payment-channels*", (r) =>
      r.fulfill({ json: { code: 0, data: { channels: [] } } }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/billing");
    await page
      .locator(".topbar")
      .getByRole("link", { name: "充值", exact: true })
      .click();
    const dialog = page.getByRole("dialog", { name: "账户充值" });
    await expect(dialog).toBeVisible();
    await expect(
      dialog.getByText("暂时没有可用的充值方式，请稍后再试或联系商家。", {
        exact: true,
      }),
    ).toBeVisible();
    await expect(
      dialog.getByRole("button", { name: "确认充值" }),
    ).toBeDisabled();
    await dialog.getByRole("button", { name: "关闭弹窗" }).click();
    await page
      .locator(".topbar")
      .getByRole("link", { name: "充值", exact: true })
      .click();
    await expect(dialog).toBeVisible();
    await view.screenshot("recharge-no-channels");
  });
});

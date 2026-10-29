import { test, expect } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";

test.describe("TC-15 登录联系方式自动改绑", () => {
  test("TC-15a 验证旧邮箱和新邮箱后自动保存，失败保留输入", async ({
    consolePage: page,
  }) => {
    let email = "owner@example.com",
      writes = 0,
      wrong = true;
    const sends: any[] = [];
    await page.route("**/user/profile", async (route) => {
      if (route.request().method() === "PUT") {
        expect(route.request().headers()["x-contact-verification"]).toBe(
          "nver_old.nver_new",
        );
        expect(route.request().postDataJSON()).toEqual({
          email: "new@example.com",
        });
        email = "new@example.com";
        writes++;
        return route.fulfill({ json: { code: 0 } });
      }
      return route.fulfill({
        json: {
          code: 0,
          data: { username: "安全账户", email, phone: "13800138000" },
        },
      });
    });
    await page.route("**/account/security/contact-codes", async (route) => {
      const input = route.request().postDataJSON();
      sends.push(input);
      if (input.oldVerification)
        expect(input).toMatchObject({
          kind: "email",
          oldVerification: "nver_old",
          destination: "new@example.com",
        });
      else {
        expect(input).not.toHaveProperty("destination");
        expect(input).not.toHaveProperty("email");
      }
      await route.fulfill({
        json: {
          code: 0,
          data: {
            verificationNo: input.oldVerification ? "nver_new" : "nver_old",
            maskedDestination: input.oldVerification
              ? "ne***@example.com"
              : "ow***@example.com",
            deliveryState: "accepted",
            expiresAt: Date.now() + 600000,
            retryAfterAt: Date.now() + 60000,
          },
        },
      });
    });
    await page.route("**/contact-codes/*/verification", async (route) => {
      if (wrong) {
        wrong = false;
        return route.fulfill({
          status: 400,
          json: { code: 400, message: "验证码不正确，请重新输入" },
        });
      }
      await route.fulfill({ json: { code: 0 } });
    });
    const view = new CustomerPage(page);
    await view.open("/account/security");
    await view.button("更换电子邮箱").click();
    const dialog = page.getByRole("dialog", { name: "更换电子邮箱" });
    await expect(
      dialog.getByRole("button", { name: "验证并继续" }),
    ).toBeDisabled();
    await dialog.getByRole("button", { name: "获取验证码" }).click();
    await expect(dialog).toContainText("验证码已发送至 ow***@example.com");
    await expect(
      dialog.getByRole("button", { name: /秒后重发/ }),
    ).toBeDisabled();
    await dialog.getByPlaceholder("6 位验证码").fill("111111");
    await dialog.getByRole("button", { name: "验证并继续" }).click();
    await expect(dialog).toContainText("验证码不正确，请重新输入");
    expect(writes).toBe(0);
    await expect(dialog.getByPlaceholder("6 位验证码")).toHaveValue("111111");
    await dialog.getByPlaceholder("6 位验证码").fill("123456");
    await dialog.getByRole("button", { name: "验证并继续" }).click();
    await expect(dialog).toContainText("身份验证已通过");
    await dialog.getByPlaceholder("请输入新邮箱").fill("new@example.com");
    await dialog.getByRole("button", { name: "获取验证码" }).click();
    await dialog.getByPlaceholder("6 位验证码").fill("654321");
    await view.screenshot("contact-change-new-email");
    await page.setViewportSize({ width: 390, height: 844 });
    await expect(
      dialog.getByRole("button", { name: "确认更换" }),
    ).toBeInViewport();
    await view.screenshot("contact-change-new-mobile");
    await page.setViewportSize({ width: 1440, height: 960 });
    await dialog.getByRole("button", { name: "确认更换" }).click();
    await expect(dialog).toContainText("电子邮箱已更新");
    expect(writes).toBe(1);
    expect(sends).toHaveLength(2);
    await view.screenshot("contact-change-success");
    await dialog.getByRole("button", { name: "完成", exact: true }).click();
    await expect(view.main).toContainText("ne***@example.com");
    await page.reload();
    await expect(view.main).toContainText("ne***@example.com");
  });

  test("TC-15b 无可验证方式时使用现有工单，手机布局完整", async ({
    consolePage: page,
  }) => {
    let calls = 0;
    await page.route("**/user/profile", (route) =>
      route.fulfill({
        json: {
          code: 0,
          data: { username: "未绑定账户", email: "", phone: "" },
        },
      }),
    );
    await page.route("**/account/security/contact-codes", async (route) => {
      calls++;
      await route.abort();
    });
    const view = new CustomerPage(page);
    await page.setViewportSize({ width: 390, height: 844 });
    await view.open("/account/security");
    await view.button("绑定手机号码").click();
    const dialog = page.getByRole("dialog", { name: "绑定手机号码" });
    await expect(dialog).toContainText("当前没有可验证的联系方式");
    await expect(
      dialog.getByRole("button", { name: "获取验证码" }),
    ).toHaveCount(0);
    await view.screenshot("contact-change-manual-mobile");
    await dialog.getByRole("link", { name: "提交工单" }).click();
    await expect(page).toHaveURL(/account\/support\/new/);
    expect(calls).toBe(0);
  });
  test("TC-15c 首次绑定手机先验证邮箱，保存失败后保留号码并可重试", async ({
    consolePage: page,
  }) => {
    let phone = "",
      writes = 0;
    const sends: unknown[] = [];
    await page.route("**/user/profile", async (route) => {
      if (route.request().method() === "PUT") {
        expect(route.request().headers()["x-contact-verification"]).toBe(
          "nver_phone_old.nver_phone_new",
        );
        expect(route.request().postDataJSON()).toEqual({
          phone: "13900139000",
        });
        writes++;
        if (writes === 1)
          return route.fulfill({
            status: 500,
            json: { code: 500, message: "保存失败，请重试" },
          });
        phone = "13900139000";
        return route.fulfill({ json: { code: 0 } });
      }
      return route.fulfill({
        json: {
          code: 0,
          data: { username: "绑定手机客户", email: "owner@example.com", phone },
        },
      });
    });
    await page.route("**/account/security/contact-codes", async (route) => {
      const input = route.request().postDataJSON();
      sends.push(input);
      expect(input.kind).toBe("phone");
      if (input.oldVerification)
        expect(input).toMatchObject({
          oldVerification: "nver_phone_old",
          destination: "13900139000",
        });
      else expect(input).not.toHaveProperty("destination");
      return route.fulfill({
        json: {
          code: 0,
          data: {
            verificationNo: input.oldVerification
              ? "nver_phone_new"
              : "nver_phone_old",
            maskedDestination: input.oldVerification
              ? "139****9000"
              : "ow***@example.com",
            deliveryState: "delivered",
            retryAfterAt: Date.now() + 60000,
            expiresAt: Date.now() + 600000,
          },
        },
      });
    });
    await page.route("**/contact-codes/*/verification", (route) =>
      route.fulfill({ json: { code: 0 } }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/security");
    await view.button("绑定手机号码").click();
    const dialog = page.getByRole("dialog", { name: "绑定手机号码" });
    await expect(dialog).toContainText("请验证已绑定的邮箱");
    await dialog
      .getByRole("button", { name: "获取验证码", exact: true })
      .click();
    await dialog.getByPlaceholder("6 位验证码").fill("123456");
    await dialog.getByRole("button", { name: "验证并继续" }).click();
    await dialog
      .getByPlaceholder("请输入手机号，国际号码需带国家区号")
      .fill("13900139000");
    await dialog
      .getByRole("button", { name: "获取验证码", exact: true })
      .click();
    await dialog.getByPlaceholder("6 位验证码").fill("654321");
    await dialog.getByRole("button", { name: "确认更换", exact: true }).click();
    await expect(dialog).toContainText("保存失败，请重试");
    await expect(
      dialog.getByPlaceholder("请输入手机号，国际号码需带国家区号"),
    ).toHaveValue("13900139000");
    await expect(dialog.getByPlaceholder("6 位验证码")).toHaveValue("654321");
    expect(phone).toBe("");
    await view.screenshot("contact-phone-save-failed");
    await dialog.getByRole("button", { name: "确认更换", exact: true }).click();
    await expect(dialog).toContainText("手机号码已更新");
    expect(writes).toBe(2);
    expect(sends).toHaveLength(2);
    await dialog.getByRole("button", { name: "完成", exact: true }).click();
    await page.reload();
    await expect(view.main).toContainText("139 **** 9000");
  });
  test("TC-15d 通道未配置时明确提示，不假装发送成功或放开验证", async ({
    consolePage: page,
  }) => {
    let saves = 0;
    await page.route("**/user/profile", async (route) => {
      if (route.request().method() === "PUT") saves++;
      return route.fulfill({
        json: {
          code: 0,
          data: {
            username: "待配置通知客户",
            email: "owner@example.com",
            phone: "",
          },
        },
      });
    });
    await page.route("**/account/security/contact-codes", (route) =>
      route.fulfill({
        status: 400,
        json: {
          code: 400,
          errorCode: "IDC_NOTIFY_CHANNEL_UNAVAILABLE",
          message: "通知渠道当前不可用",
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/security");
    await view.button("更换电子邮箱").click();
    const dialog = page.getByRole("dialog", { name: "更换电子邮箱" });
    await dialog
      .getByRole("button", { name: "获取验证码", exact: true })
      .click();
    await expect(dialog).toContainText("验证码服务暂未开通，请联系网站客服");
    await expect(
      dialog.getByRole("button", { name: "获取验证码", exact: true }),
    ).toBeEnabled();
    await expect(dialog.getByRole("button", { name: /秒后重发/ })).toHaveCount(
      0,
    );
    await expect(
      dialog.getByRole("button", { name: "验证并继续" }),
    ).toBeDisabled();
    await expect(dialog).not.toContainText("验证码已发送至");
    await expect(dialog.getByPlaceholder("请输入新邮箱")).toHaveCount(0);
    expect(saves).toBe(0);
    await view.screenshot("contact-channel-unconfigured");
  });
});

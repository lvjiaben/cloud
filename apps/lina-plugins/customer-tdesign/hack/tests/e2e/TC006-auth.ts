import { test, expect } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-6 登录与首次密码", () => {
  test("TC-6a 登录失败可以重试，成功恢复内部深链接", async ({
    apiPage: page,
  }) => {
    let fail = true;
    await page.route("**/api/v1/config/public/frontend", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            auth: { registerEnabled: false, forgetPasswordEnabled: true },
          },
        },
      }),
    );
    await page.route(
      "**/x/idc-notify/api/v1/public/account/password-logins",
      (r) =>
        r.fulfill(
          fail
            ? { status: 401, json: { code: 1, message: "用户名或密码不正确" } }
            : { json: { code: 0, data: { accessToken: "fresh" } } },
        ),
    );
    await page.route("**/initial-password/status", (r) =>
      r.fulfill({ json: { code: 0, data: { required: false } } }),
    );
    const view = new CustomerPage(page);
    await view.open("/auth/login?redirect=%2Faccount%2Forders");
    await page
      .getByRole("textbox", { name: "账号", exact: true })
      .fill("test-customer");
    await page.getByLabel("密码", { exact: true }).fill("test-password");
    await view.button("登录用户中心").click();
    await expect(page.getByText("用户名或密码不正确")).toBeVisible();
    await expect(page.getByRole("link", { name: "注册账户" })).toHaveCount(0);
    fail = false;
    await view.button("登录用户中心").click();
    await expect(page).toHaveURL(/\/account\/orders$/);
    await expect(page.getByRole("heading", { name: "我的订单" })).toBeVisible();
    expect(
      await page.evaluate(
        () =>
          JSON.parse(sessionStorage.getItem("customer-tdesign.session")!)
            .accessToken,
      ),
    ).toBe("fresh");
  });
  test("TC-6b 初始密码阻止进入业务，先保存宿主密码再解除门禁", async ({
    consolePage: page,
  }) => {
    let required = true;
    const calls: string[] = [];
    await page.route("**/initial-password/status", (r) =>
      r.fulfill({ json: { code: 0, data: { required } } }),
    );
    await page.route("**/user/profile", (r) => {
      if (r.request().method() === "GET")
        return r.fulfill({
          json: {
            code: 0,
            data: { username: "验收账户", email: "test@example.com" },
          },
        });
      calls.push("host");
      expect(r.request().postDataJSON()).toEqual({
        password: "new-password-2026",
      });
      return r.fulfill({ json: { code: 0, data: {} } });
    });
    await page.route("**/account/initial-password", (r) => {
      calls.push("gate");
      required = false;
      return r.fulfill({ json: { code: 0, data: { required: false } } });
    });
    const view = new CustomerPage(page);
    await view.open();
    await expect(
      page.getByRole("heading", { name: "修改初始密码" }),
    ).toBeVisible();
    await expect(page.getByRole("navigation")).toHaveCount(0);
    await page.getByLabel("密码", { exact: true }).fill("new-password-2026");
    await page
      .getByLabel("再次输入新密码", { exact: true })
      .fill("new-password-2026");
    await view.button("修改初始密码").click();
    await expect(page.getByRole("heading", { name: "账户概览" })).toBeVisible();
    expect(calls).toEqual(["host", "gate"]);
  });
  test("TC-6c 重置密码校验确认值与链接，失败不发送更新", async ({
    apiPage: page,
  }) => {
    let calls = 0;
    await page.route("**/config/public/frontend", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            auth: { forgetPasswordEnabled: true, registerEnabled: false },
          },
        },
      }),
    );
    await page.route("**/api/v1/auth/reset-password", (r) => {
      calls++;
      return r.fulfill({ json: { code: 0, data: {} } });
    });
    const view = new CustomerPage(page);
    await view.open("/auth/reset-password");
    await page.getByLabel("密码", { exact: true }).fill("new-password-2026");
    await page
      .getByLabel("再次输入新密码", { exact: true })
      .fill("different-password");
    await view.button("设置新密码").click();
    await expect(page.getByText("两次输入的新密码不一致")).toBeVisible();
    await page
      .getByLabel("再次输入新密码", { exact: true })
      .fill("new-password-2026");
    await view.button("设置新密码").click();
    await expect(
      page.getByText("重置链接不完整，请从邮件中重新打开"),
    ).toBeVisible();
    expect(calls).toBe(0);
    await view.screenshot("auth-reset-invalid");
    await view.open("/auth/reset-password?token=valid-reset-token");
    await page.getByLabel("密码", { exact: true }).fill("new-password-2026");
    await page
      .getByLabel("再次输入新密码", { exact: true })
      .fill("new-password-2026");
    await view.button("设置新密码").click();
    await expect(page).toHaveURL(/auth\/login$/);
    expect(calls).toBe(1);
    await page.setViewportSize({ width: 390, height: 844 });
    await view.screenshot("auth-login-mobile");
  });
  test("TC-6d 注册验证和开户失败可重试，不重复创建登录账户", async ({
    apiPage: page,
  }) => {
    let created = 0,
      redeemed = 0,
      verified = false;
    await page.route("**/api/v1/config/public/frontend", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            auth: { registerEnabled: true, forgetPasswordEnabled: true },
          },
        },
      }),
    );
    const path = "**/x/idc-notify/api/v1/public/account/onboarding/";
    await page.route(path + "human-challenges", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            challengeNo: "challenge",
            accessiblePrompt: "请输入七加一的结果",
            expiresAt: Date.now() + 60000,
          },
        },
      }),
    );
    await page.route(path + "human-challenge-completions", (r) => {
      expect(r.request().postDataJSON().answer).toBe("8");
      return r.fulfill({
        json: { code: 0, data: { proofToken: "test-proof" } },
      });
    });
    await page.route(path + "verification-codes", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            verificationNo: "verification",
            maskedDestination: "t***@example.invalid",
            expiresAt: Date.now() + 60000,
          },
        },
      }),
    );
    await page.route(path + "intents", (r) => {
      if (!verified)
        return r.fulfill({
          status: 400,
          json: { code: 400, message: "验证码不正确" },
        });
      return r.fulfill({
        json: { code: 0, data: { token: "verified-intent" } },
      });
    });
    await page.route("**/api/v1/auth/register", (r) => {
      created++;
      expect(r.request().postDataJSON()).toMatchObject({
        username: "newcustomer",
        email: "test@example.invalid",
        password: "new-password-2026",
      });
      return r.fulfill({ json: { code: 0, data: { userId: 100 } } });
    });
    await page.route(
      "**/x/idc-notify/api/v1/public/account/password-logins",
      (r) =>
        r.fulfill({
          json: { code: 0, data: { accessToken: "registered-session" } },
        }),
    );
    await page.route("**/x/idc-core/api/v1/account/onboarding/redeem", (r) => {
      redeemed++;
      expect(r.request().postDataJSON()).toMatchObject({
        hostRegistrationUserId: 100,
        token: "verified-intent",
      });
      return r.fulfill(
        redeemed === 1
          ? {
              status: 503,
              json: { code: 503, message: "开户暂未完成，请重试" },
            }
          : { json: { code: 0, data: { customerNo: "customer-test" } } },
      );
    });
    await page.route("**/initial-password/status", (r) =>
      r.fulfill({ json: { code: 0, data: { required: false } } }),
    );
    const view = new CustomerPage(page);
    await view.open("/auth/register");
    await page.getByLabel("用户名", { exact: true }).fill(" newcustomer ");
    await page
      .getByLabel("电子邮箱", { exact: true })
      .fill("test@example.invalid");
    await page.getByLabel("密码", { exact: true }).fill("new-password-2026");
    await page
      .getByLabel("再次输入新密码", { exact: true })
      .fill("new-password-2026");
    await page
      .getByText("我使用本人邮箱注册，并同意服务条款及隐私政策", {
        exact: true,
      })
      .click();
    await view.button("下一步").click();
    await expect(page.getByText("请输入七加一的结果")).toBeVisible();
    await page.getByLabel("人机验证答案").fill("8");
    await view.button("下一步").click();
    await page.getByLabel("邮箱验证码", { exact: true }).fill("000000");
    await view.button("完成注册").click();
    await expect(page.getByText("验证码不正确")).toBeVisible();
    expect(created).toBe(0);
    verified = true;
    await page.getByLabel("邮箱验证码", { exact: true }).fill("123456");
    await view.button("完成注册").click();
    await expect(page.getByText("开户暂未完成，请重试")).toBeVisible();
    expect(created).toBe(1);
    await view.screenshot("auth-registration-retry");
    await view.button("完成注册").click();
    await expect(page).toHaveURL(/\/account$/);
    expect(created).toBe(1);
    expect(redeemed).toBe(2);
    expect(
      await page.evaluate(() =>
        sessionStorage.getItem("customer-tdesign.onboarding"),
      ),
    ).toBeNull();
  });
  test("TC-6e 注册拒绝数字和邮箱用户名，合法独立用户名继续验证", async ({
    apiPage: page,
  }) => {
    let challenges = 0;
    await page.route("**/config/public/frontend", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            auth: { registerEnabled: true, forgetPasswordEnabled: true },
          },
        },
      }),
    );
    await page.route("**/onboarding/human-challenges", (r) => {
      challenges++;
      expect(r.request().postDataJSON().username).toBe("customer_123");
      return r.fulfill({
        json: {
          code: 0,
          data: {
            challengeNo: "CHL",
            accessiblePrompt: "请输入一加二的结果",
            expiresAt: Date.now() + 60000,
          },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/auth/register");
    await page
      .getByLabel("电子邮箱", { exact: true })
      .fill("current@example.com");
    await page.getByLabel("密码", { exact: true }).fill("example-password");
    await page
      .getByLabel("再次输入新密码", { exact: true })
      .fill("example-password");
    await page
      .getByText("我使用本人邮箱注册，并同意服务条款及隐私政策", {
        exact: true,
      })
      .click();
    await view.button("下一步").click();
    await expect(page.getByText("请输入用户名", { exact: true })).toBeVisible();
    expect(challenges).toBe(0);
    for (const account of [
      "12345",
      "１２３４５",
      "13800138000",
      "+8613800138000",
      "name@example.com",
    ]) {
      await page.getByLabel("用户名", { exact: true }).fill(account);
      await view.button("下一步").click();
      await expect(
        page.getByText("用户名不能是纯数字、手机号或邮箱", { exact: true }),
      ).toBeVisible();
      expect(challenges).toBe(0);
    }
    await view.screenshot("unified-login-invalid-registration");
    await page.getByLabel("用户名", { exact: true }).fill("customer_123");
    await view.button("下一步").click();
    await expect(page.getByText("请输入一加二的结果")).toBeVisible();
    expect(challenges).toBe(1);
  });
});

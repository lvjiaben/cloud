// Customer-owned UI authentication contracts. Real host/SPI is verified separately.
import { test, expect, type Page } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
const prefix = "**/x/idc-notify/api/v1/public/account/";
const recovery = prefix + "password-recovery/";
async function settings(page: Page) {
  await page.route("**/config/public/frontend", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: { auth: { registerEnabled: false, forgetPasswordEnabled: true } },
      },
    }),
  );
}
async function challengeRoutes(page: Page) {
  await page.route(recovery + "human-challenges", (r) =>
    r.fulfill({
      json: {
        code: 0,
        data: {
          challengeNo: "CHL-1",
          kind: "accessible",
          providerCode: "local",
          accessiblePrompt: "1 + 2 = ?",
          expiresAt: Date.now() + 120000,
        },
      },
    }),
  );
  await page.route(recovery + "human-challenge-completions", (r) => {
    expect(r.request().postDataJSON().answer).toBe("3");
    return r.fulfill({ json: { code: 0, data: { proofToken: "proof" } } });
  });
}
async function identify(page: Page, phone = false) {
  const view = new CustomerPage(page);
  await view.open("/auth/forget-password");
  if (phone) await page.getByText("通过手机找回", { exact: true }).click();
  await page
    .getByRole("textbox", { name: phone ? "手机号" : "电子邮箱", exact: true })
    .fill(phone ? "13800138000" : "current@example.com");
  await view.button("下一步").click();
  await page.getByRole("textbox", { name: "人机验证答案" }).fill("3");
  await view.button("获取验证码").click();
  return view;
}
test.describe("TC-17 联系方式登录与找回", () => {
  for (const [label, kind, account] of [
    ["用户名", "username", "independent_customer"],
    ["电子邮箱", "email", "current@example.com"],
    ["手机号", "phone", "13800138000"],
    ["国际手机号", "phone", "+8613800138000"],
  ]) {
    test(`TC-17a ${label}统一账号框识别且错误可重试`, async ({
      apiPage: page,
    }) => {
      await settings(page);
      let calls = 0;
      await page.route(prefix + "password-logins", (r) => {
        expect(r.request().postDataJSON()).toEqual({
          kind,
          account,
          password: "example-password",
        });
        calls++;
        return r.fulfill(
          calls === 1
            ? {
                status: 401,
                json: { code: 61, message: "账户信息或密码不正确" },
              }
            : {
                json: {
                  code: 0,
                  data: { accessToken: `same-customer-${kind}` },
                },
              },
        );
      });
      const view = new CustomerPage(page);
      await view.open("/auth/login");
      await expect(page.getByRole("radio")).toHaveCount(0);
      if (kind === "username")
        await view.screenshot("unified-account-login-desktop");
      await expect(
        page.getByPlaceholder("用户名 / 邮箱 / 手机号", { exact: true }),
      ).toBeVisible();
      await page
        .getByRole("textbox", { name: "账号", exact: true })
        .fill(account);
      await page.getByLabel("密码", { exact: true }).fill("example-password");
      await view.button("登录用户中心").click();
      await expect(
        page.getByText("账户信息或密码不正确", { exact: true }),
      ).toBeVisible();
      await view.button("登录用户中心").click();
      await expect.poll(() => calls).toBe(2);
      await expect(page).toHaveURL(/\/account$/);
      expect(
        await page.evaluate(
          () =>
            JSON.parse(sessionStorage.getItem("customer-tdesign.session")!)
              .accessToken,
        ),
      ).toBe(`same-customer-${kind}`);
      expect(calls).toBe(2);
    });
  }
  test("TC-17b 手机验证码找回、失败重试、独立旧会话清理", async ({
    apiPage: page,
  }) => {
    await settings(page);
    await challengeRoutes(page);
    let fail = true,
      resets = 0,
      cleanups = 0,
      hostResets = 0;
    await page.route("**/api/v1/auth/reset-password", (r) => {
      hostResets++;
      return r.fulfill({ json: { code: 0, data: {} } });
    });
    await page.route(recovery + "verification-codes", (r) => {
      expect(r.request().postDataJSON()).toMatchObject({
        kind: "phone",
        account: "13800138000",
        proofToken: "proof",
      });
      return r.fulfill({
        json: {
          code: 0,
          data: {
            verificationNo: "nver_recovery",
            deliveryState: "delivered",
            maskedDestination: "138****8000",
            expiresAt: Date.now() + 600000,
            retryAfterAt: Date.now() + 60000,
          },
        },
      });
    });
    await page.route(recovery + "password-resets", (r) => {
      resets++;
      expect(r.request().postDataJSON()).toMatchObject({
        kind: "phone",
        account: "13800138000",
        verificationNo: "nver_recovery",
        code: "123456",
        password: "new-password-2026",
      });
      return r.fulfill(
        fail
          ? { status: 409, json: { code: 1, message: "暂时未能保存，请重试" } }
          : {
              json: {
                code: 0,
                data: {
                  reset: true,
                  cleanupPending: true,
                  cleanupToken: "c".repeat(64),
                },
              },
            },
      );
    });
    await page.route(recovery + "session-cleanups", (r) => {
      cleanups++;
      const data = r.request().postDataJSON();
      expect(data.cleanupToken).toBe("c".repeat(64));
      expect(data.code).toBeUndefined();
      expect(data.password).toBeUndefined();
      return r.fulfill({
        json: { code: 0, data: { pending: cleanups === 1 } },
      });
    });
    const view = await identify(page, true);
    await page
      .getByRole("textbox", { name: "验证码", exact: true })
      .fill("123456");
    await page.getByLabel("新密码", { exact: true }).fill("new-password-2026");
    await page
      .getByLabel("再次输入新密码", { exact: true })
      .fill("new-password-2026");
    await view.button("保存新密码").click();
    await expect(
      page.getByText("暂时未能保存，请重试", { exact: true }),
    ).toBeVisible();
    fail = false;
    await view.button("保存新密码").click();
    await expect(page.getByText("新密码已保存", { exact: true })).toBeVisible();
    await view.button("重试退出旧登录").click();
    await expect(
      page.getByText("暂时未能全部退出，请稍后再试", { exact: true }),
    ).toBeVisible();
    await view.button("重试退出旧登录").click();
    await expect(view.button("重试退出旧登录")).toHaveCount(0);
    expect(resets).toBe(2);
    expect(cleanups).toBe(2);
    expect(hostResets).toBe(0);
    await view.screenshot("recovery-phone-saved");
  });
  test("TC-17c 通道未配置不显示发送成功或进入密码步骤", async ({
    apiPage: page,
  }) => {
    await settings(page);
    await challengeRoutes(page);
    let calls = 0;
    await page.route(recovery + "human-challenges", (r) => {
      calls++;
      return r.fulfill({
        json: {
          code: 0,
          data: {
            challengeNo: `CHL-${calls}`,
            kind: "accessible",
            providerCode: "local",
            accessiblePrompt: "1 + 2 = ?",
            expiresAt: Date.now() + 120000,
          },
        },
      });
    });
    await page.route(recovery + "verification-codes", (r) =>
      r.fulfill({
        status: 409,
        json: {
          code: 1,
          errorCode: "IDC_NOTIFY_CHANNEL_UNAVAILABLE",
          message: "通知渠道当前不可用",
        },
      }),
    );
    const view = await identify(page);
    await expect(
      page.getByText("验证码服务暂未开通，请联系网站客服", { exact: true }),
    ).toBeVisible();
    await expect(page.getByLabel("新密码", { exact: true })).toHaveCount(0);
    await expect.poll(() => calls).toBe(2);
    await view.screenshot("recovery-channel-unconfigured");
  });
});

for (const width of [320, 390]) {
  test(`TC-17d ${width}px手机登录和三步找回无横向溢出`, async ({
    apiPage: page,
  }) => {
    await page.setViewportSize({ width, height: 844 });
    await settings(page);
    await challengeRoutes(page);
    await page.route(recovery + "verification-codes", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            verificationNo: "nver_mobile",
            deliveryState: "accepted",
            maskedDestination: "c***@example.com",
            expiresAt: Date.now() + 600000,
            retryAfterAt: Date.now() + 60000,
          },
        },
      }),
    );
    await page.route(recovery + "password-resets", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            reset: true,
            cleanupPending: false,
            cleanupToken: "c".repeat(64),
          },
        },
      }),
    );
    const view = new CustomerPage(page);
    await view.open("/auth/login");
    await expect(
      page.getByRole("textbox", { name: "账号", exact: true }),
    ).toBeVisible();
    await expect
      .poll(() =>
        page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      )
      .toBe(true);
    await view.screenshot(`contact-login-mobile-${width}`);
    await identify(page);
    await page
      .getByRole("textbox", { name: "验证码", exact: true })
      .fill("123456");
    await page.getByLabel("新密码", { exact: true }).fill("new-password-2026");
    await page
      .getByLabel("再次输入新密码", { exact: true })
      .fill("new-password-2026");
    await expect
      .poll(() =>
        page.evaluate(
          () => document.documentElement.scrollWidth <= window.innerWidth,
        ),
      )
      .toBe(true);
    await view.screenshot(`recovery-form-mobile-${width}`);
    await view.button("保存新密码").click();
    await expect(page.getByText("新密码已保存", { exact: true })).toBeVisible();
  });
}

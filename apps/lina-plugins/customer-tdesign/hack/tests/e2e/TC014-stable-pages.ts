import { test, expect } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";

test.describe("TC-14 页面稳定性与登录方式保护", () => {
  test("TC-14a 慢页面准备期间保留原页面，数据就绪只切换一次", async ({
    consolePage: page,
  }) => {
    let release!: () => void,
      requested = false;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route("**/account/customer/profile", async (route) => {
      requested = true;
      await gate;
      await route.fulfill({
        json: {
          code: 0,
          data: {
            item: {
              displayName: "稳定页面客户",
              customerCode: 10002,
              type: "individual",
              status: "active",
              version: 1,
            },
          },
        },
      });
    });
    await page.route("**/account/customer/contacts", (route) =>
      route.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    const view = new CustomerPage(page);
    await view.open();
    await expect(
      page.getByRole("heading", { name: "账户概览", exact: true }),
    ).toBeVisible();
    await expect(view.main.locator(".content-skeleton")).toHaveCount(0);
    await page.evaluate(() => {
      const main = document.querySelector(".main-content")!;
      (window as any).badFrames = [];
      (window as any).changes = [];
      let previous = "";
      const sample = () => {
        const h = main.querySelector("h1")?.textContent || "";
        if (h !== previous) {
          (window as any).changes.push(h);
          previous = h;
        }
        if (!h || main.querySelector(".content-skeleton,.loading"))
          (window as any).badFrames.push(h);
        (window as any).raf = requestAnimationFrame(sample);
      };
      sample();
    });
    await page
      .locator("nav")
      .getByRole("link", { name: "账户资料", exact: true })
      .click();
    await expect.poll(() => requested).toBe(true);
    await expect(view.main.locator("h1")).toHaveText("账户概览");
    await expect(page.locator(".route-progress")).toBeVisible();
    await expect(page.locator(".route-content")).toHaveAttribute("inert", "");
    release();
    await expect(
      page.getByRole("heading", { name: "账户资料", exact: true }),
    ).toBeVisible();
    await expect(view.main).toContainText("稳定页面客户");
    await expect(page.locator(".route-progress")).toHaveCount(0);
    await expect(page.locator(".route-content")).not.toHaveAttribute(
      "inert",
      "",
    );
    const frames = await page.evaluate(() => {
      cancelAnimationFrame((window as any).raf);
      return {
        bad: (window as any).badFrames,
        changes: (window as any).changes,
      };
    });
    expect(frames.bad).toEqual([]);
    expect(frames.changes).toEqual(["账户概览", "账户资料"]);
    await view.screenshot("stable-profile-transition");
  });
  test("TC-14b 登录方式仅展示脱敏值，不伪造在线改绑成功，可进入人工核验", async ({
    consolePage: page,
  }) => {
    let contactWrites = 0;
    await page.route("**/user/profile", (route) => {
      if (route.request().method() !== "GET") contactWrites++;
      return route.fulfill({
        json: {
          code: 0,
          data: { username: "安全验收", email: "owner@example.com", phone: "" },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/security");
    await expect(view.main).toContainText("ow***@example.com");
    await view.button("绑定手机号码").click();
    const dialog = page.getByRole("dialog", { name: "绑定手机号码" });
    await expect(dialog).toContainText("请验证已绑定的邮箱");
    await expect(
      dialog.getByRole("button", { name: "获取验证码" }),
    ).toBeVisible();
    await view.screenshot("contact-binding-old-verification");
    await dialog.getByRole("button", { name: "取消", exact: true }).click();
    await view.button("更换电子邮箱").click();
    await expect(page.getByRole("dialog")).toContainText("请验证原邮箱");
    await page
      .getByRole("dialog")
      .getByRole("link", { name: "提交工单" })
      .click();
    await expect(page).toHaveURL(/account\/support\/new/);
    expect(contactWrites).toBe(0);
    await page.goBack();
    await expect(
      page.getByRole("heading", { name: "登录与安全", exact: true }),
    ).toBeVisible();
    await page.setViewportSize({ width: 390, height: 844 });
    await expect
      .poll(() =>
        page.evaluate(() => document.documentElement.scrollWidth <= innerWidth),
      )
      .toBe(true);
    await view.screenshot("account-security-mobile");
  });
  test("TC-14c 新页面请求失败时可重试，不清空外壳或显示假资料", async ({
    consolePage: page,
  }) => {
    let fail = true;
    await page.route("**/user/profile", (route) =>
      route.fulfill(
        fail
          ? { status: 503, json: { code: 503, message: "账户资料暂时不可用" } }
          : {
              json: {
                code: 0,
                data: { username: "重试后的账户", email: "safe@example.com" },
              },
            },
      ),
    );
    const view = new CustomerPage(page);
    await view.open("/account/security");
    await expect(
      view.main.getByText("账户资料暂时不可用").first(),
    ).toBeVisible();
    await expect(page.locator(".topbar")).toBeVisible();
    await expect(view.main).not.toContainText("重试后的账户");
    fail = false;
    await view.main
      .getByRole("button", { name: "重新加载", exact: true })
      .first()
      .click();
    await expect(view.main).toContainText("sa***@example.com");
    await expect(view.main.getByText("账户资料暂时不可用")).toHaveCount(0);
    await view.screenshot("account-security-recovered");
  });
  test("TC-14d 共用费用组件的页面切换也保留内容直到数据就绪", async ({
    consolePage: page,
  }) => {
    let release!: () => void;
    const gate = new Promise<void>((resolve) => {
      release = resolve;
    });
    await page.route("**/billing/transactions?*", async (route) => {
      await gate;
      await route.fulfill({
        json: { code: 0, data: { transactions: [], nextCursor: "" } },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/billing");
    await expect(
      view.main.getByRole("heading", { name: "余额与账单", exact: true }),
    ).toBeVisible();
    await view.main
      .getByRole("link", { name: "收支明细", exact: true })
      .first()
      .click();
    await expect(page.locator(".route-progress")).toBeVisible();
    await expect(view.main.locator("h1")).toHaveText("余额与账单");
    await expect(view.main.locator(".content-skeleton")).toHaveCount(0);
    release();
    await expect(
      view.main.getByRole("heading", { name: "收支明细", exact: true }),
    ).toBeVisible();
    await expect(page.locator(".route-progress")).toHaveCount(0);
    await expect(view.main.locator(".content-skeleton")).toHaveCount(0);
  });
});

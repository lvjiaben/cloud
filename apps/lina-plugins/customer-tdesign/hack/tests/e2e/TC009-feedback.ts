import { test, expect } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";
test.describe("TC-9 实际操作反馈回归", () => {
  test("TC-9a 菜单整行可点击，窄屏导航后关闭抽屉", async ({
    consolePage: page,
  }) => {
    await page.route("**/customer/profile", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            item: {
              displayName: "验收账户",
              customerCode: 1001,
              status: "active",
              type: "individual",
              verification: "unverified",
              version: 1,
            },
          },
        },
      }),
    );
    await page.route("**/customer/contacts", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    const view = new CustomerPage(page);
    await view.open();
    const link = page
      .getByRole("navigation")
      .getByRole("link", { name: "订单", exact: true });
    const box = await link.boundingBox();
    expect(box!.width).toBeGreaterThan(150);
    await link.click({ position: { x: box!.width - 6, y: box!.height / 2 } });
    await expect(page).toHaveURL(/account\/orders$/);
    await expect(link).toHaveAttribute("aria-current", "page");
    await view.screenshot("feedback-navigation");
    await page.setViewportSize({ width: 390, height: 844 });
    await view.button("打开菜单").click();
    await page
      .locator(".t-drawer")
      .getByRole("link", { name: "账户资料", exact: true })
      .click();
    await expect(page).toHaveURL(/account\/profile$/);
    await expect(page.locator(".t-drawer")).not.toBeVisible();
  });
  test("TC-9b 地址默认值生效，失败保留、再次新增不继承旧地址", async ({
    consolePage: page,
  }) => {
    let fail = true;
    const items: any[] = [];
    let saved: any;
    await page.route("**/customer/addresses", (r) => {
      if (r.request().method() === "GET")
        return r.fulfill({ json: { code: 0, data: { items } } });
      saved = r.request().postDataJSON();
      if (fail)
        return r.fulfill({
          status: 503,
          json: { code: 503, message: "保存未完成，请重试" },
        });
      items.push({
        ...saved,
        addressNo: "ADDR-test",
        status: "active",
        version: 1,
      });
      return r.fulfill({ json: { code: 0, data: { item: items[0] } } });
    });
    const view = new CustomerPage(page);
    await view.open("/account/addresses");
    await view.button("添加地址").click();
    const dialog = page.getByRole("dialog", { name: "添加地址" });
    await expect(dialog).toBeVisible();
    await expect(dialog.locator(".t-select input").nth(0)).toHaveValue(
      "联系地址",
    );
    await expect(dialog.locator(".t-select input").nth(1)).toHaveValue("中国");
    for (const [label, value] of [
      ["名称", "测试收件地址"],
      ["联系人", "测试用户"],
      ["联系电话", "13800000000"],
      ["省份", "浙江省"],
      ["城市", "杭州市"],
      ["详细地址", "测试街道 1 号"],
    ])
      await dialog.getByLabel(label, { exact: true }).fill(value);
    await dialog.getByText("设为该用途的默认地址", { exact: true }).click();
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog).toContainText("保存未完成，请重试");
    await expect(dialog.getByLabel("名称", { exact: true })).toHaveValue(
      "测试收件地址",
    );
    await view.screenshot("feedback-address-error");
    fail = false;
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    expect(saved).toMatchObject({
      countryCode: "CN",
      type: "contact",
      default: true,
    });
    await expect(view.main).toContainText("测试收件地址");
    await view.button("添加地址").click();
    await expect(dialog).toBeVisible();
    await expect(dialog.getByLabel("名称", { exact: true })).toHaveValue("");
    await expect(dialog.getByRole("checkbox")).not.toBeChecked();
    await page.setViewportSize({ width: 390, height: 844 });
    await dialog.getByRole("button", { name: "保存", exact: true }).focus();
    await page.keyboard.press("Tab");
    await expect(
      dialog.getByRole("button", { name: "关闭弹窗" }),
    ).toBeFocused();
    await page.keyboard.press("Shift+Tab");
    await expect(
      dialog.getByRole("button", { name: "保存", exact: true }),
    ).toBeFocused();
    await view.screenshot("feedback-address-mobile");
    const bounds = await dialog.boundingBox();
    expect(bounds!.x).toBeGreaterThanOrEqual(0);
    expect(bounds!.x + bounds!.width).toBeLessThanOrEqual(390);

    await dialog.getByRole("button", { name: "关闭弹窗" }).click();
    await expect(dialog).toHaveCount(0);
  });
  test("TC-9c 资料编辑预填正确，取消后可重开且保存重读结果", async ({
    consolePage: page,
  }) => {
    let item = {
      displayName: "原始名称",
      version: 3,
      countryCode: "CN",
      customerCode: 1001,
      type: "individual",
      status: "active",
      verification: "unverified",
    };
    await page.route("**/customer/profile", (r) => {
      if (r.request().method() === "PUT") {
        const v = r.request().postDataJSON();
        expect(v.expectedVersion).toBe(3);
        item = { ...item, displayName: v.displayName, version: 4 };
      }
      return r.fulfill({ json: { code: 0, data: { item } } });
    });
    await page.route("**/customer/contacts", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [] } } }),
    );
    const view = new CustomerPage(page);
    await view.open("/account/profile");
    await view.button("编辑").click();
    const dialog = page.getByRole("dialog", { name: "编辑账户资料" });
    await expect(dialog.getByRole("textbox")).toHaveValue("原始名称");
    await dialog.getByRole("textbox").fill("未保存草稿");
    await dialog.getByRole("button", { name: "关闭弹窗" }).click();
    await view.button("编辑").click();
    await expect(dialog.getByRole("textbox")).toHaveValue("原始名称");
    await dialog.getByRole("textbox").fill("更新后的名称");
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(dialog).toHaveCount(0);
    await expect(view.main).toContainText("更新后的名称");
    await view.screenshot("feedback-profile-saved");
  });
  test("TC-9d 修改密码先核对当前密码，失败不更新，成功退出旧会话", async ({
    consolePage: page,
  }) => {
    let updates = 0,
      valid = false;
    await page.route("**/auth/login", (r) =>
      r.fulfill(
        valid
          ? { json: { code: 0, data: { accessToken: "new-session" } } }
          : { status: 401, json: { code: 401, message: "当前密码不正确" } },
      ),
    );
    await page.route("**/user/profile", (r) => {
      if (r.request().method() === "GET") return r.fulfill({json:{code:0,data:{username:"验收账户",phone:"13800138000",email:"test@example.com"}}});
      updates++;
      expect(r.request().postDataJSON()).toEqual({
        password: "new-password-2026-with-more-than-32-characters",
      });
      return r.fulfill({ json: { code: 0, data: {} } });
    });
    const view = new CustomerPage(page);
    await view.open("/account/security");
    await page.getByLabel("当前密码", { exact: true }).fill("wrong-password");
    await page
      .getByLabel("新密码", { exact: true })
      .fill("new-password-2026-with-more-than-32-characters");
    await page
      .getByLabel("确认新密码", { exact: true })
      .fill("new-password-2026-with-more-than-32-characters");
    await view.button("保存新密码").click();
    await expect(
      page.getByText("当前密码不正确", { exact: true }),
    ).toBeVisible();
    expect(updates).toBe(0);
    valid = true;
    await page.getByLabel("当前密码", { exact: true }).fill("correct-password");
    await view.button("保存新密码").click();
    await expect(page).toHaveURL(/auth\/login/);
    expect(updates).toBe(1);
    await expect(
      page.getByText("密码已更新，请使用新密码登录。"),
    ).toBeVisible();
    expect(
      await page.evaluate(() =>
        sessionStorage.getItem("customer-tdesign.session"),
      ),
    ).toBeNull();
    await view.screenshot("feedback-security-success");
  });
  test("TC-9e 仅开通美元渠道时充值自动选择可用币种", async ({
    consolePage: page,
  }) => {
    await page.route("**/payment-channels*", (r) =>
      r.fulfill({
        json: {
          code: 0,
          data: {
            channels: [
              {
                channelNo: "USD-channel",
                displayName: "美元支付",
                currencies: ["USD"],
                modes: ["page"],
              },
            ],
          },
        },
      }),
    );
    await page.route("**/balance-recharges", (r) => {
      expect(r.request().postDataJSON()).toMatchObject({
        currencyCode: "USD",
        amountMinor: "10000",
        channelNo: "USD-channel",
      });
      return r.fulfill({
        json: {
          code: 0,
          data: {
            attemptNo: "PAY-test",
            redirectMethod: "GET",
            redirectUrl: "https://pay.example.invalid/",
            expiresAt: Date.now() + 60000,
          },
        },
      });
    });
    const view = new CustomerPage(page);
    await view.open("/account/billing?recharge=1");
    const dialog = page.getByRole("dialog", { name: "账户充值" });
    await expect(dialog).toBeVisible();
    await expect(dialog.locator(".t-select input").first()).toHaveValue("USD");
    await dialog.locator(".t-select").nth(1).click();
    await page.getByText("美元支付", { exact: true }).click();
    await dialog.getByRole("button", { name: "确认充值" }).click();
    await expect(dialog.getByText("请完成支付后返回查看结果")).toBeVisible();
    await expect(
      dialog.getByRole("link", { name: "打开支付页面" }),
    ).toHaveAttribute("href", "https://pay.example.invalid/");
    await view.screenshot("feedback-recharge-usd");
  });
  test("TC-9f 抬头连续编辑保留默认状态，银行账号留空不会清除", async ({
    consolePage: page,
  }) => {
    let item = {
      profileNo: "title-1",
      invoiceTitle: "测试企业",
      titleTypeCode: "enterprise",
      taxRegistrationNo: "91310000TEST",
      statusCode: "active",
      isDefault: true,
      bankAccountConfigured: true,
      bankAccountMasked: "****5678",
      version: 2,
    };
    await page.route("**/billing/invoice-profiles?*", (r) =>
      r.fulfill({ json: { code: 0, data: { items: [item], nextCursor: "" } } }),
    );
    await page.route("**/billing/invoice-profiles/title-1", (r) => {
      const input = r.request().postDataJSON();
      expect(input.expectedVersion).toBe(2);
      expect(input.bankAccountAction).toBe("keep");
      expect(input).not.toHaveProperty("isDefault");
      item = { ...item, invoiceTitle: input.invoiceTitle, version: 3 };
      return r.fulfill({ json: { code: 0, data: { item } } });
    });
    const view = new CustomerPage(page);
    await view.open("/account/billing/invoice-profiles");
    await view.button("编辑").click();
    const dialog = page.getByRole("dialog", { name: "编辑抬头" });
    await expect(dialog.locator("input").nth(1)).toHaveValue("测试企业");
    await dialog.getByRole("button", { name: "关闭弹窗" }).click();
    await view.button("编辑").click();
    await expect(dialog.locator("input").nth(1)).toHaveValue("测试企业");
    await dialog.locator("input").nth(1).fill("更新企业名称");
    await dialog.getByRole("button", { name: "保存", exact: true }).click();
    await expect(view.main).toContainText("更新企业名称");
    await expect(view.main.getByText("默认", { exact: true })).toBeVisible();
    await view.button("编辑").click();
    await expect(dialog.locator("input").nth(1)).toHaveValue("更新企业名称");
    await expect(dialog.locator("input").last()).toHaveValue("");
    await view.screenshot("feedback-invoice-title");
  });
});

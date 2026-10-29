import { test, expect } from "../support/fixtures";

const fields = [
  { fieldToken: "opaque-system", type: "single_select", presentation: "operating_system", labelZhCN: "操作系统", labelEnUS: "Operating system", visible: true, required: true, options: [
    { optionToken: "opaque-u22", labelZhCN: "Ubuntu 22.04 LTS", labelEnUS: "Ubuntu 22.04 LTS" },
    { optionToken: "opaque-u24", labelZhCN: "Ubuntu 24.04 LTS", labelEnUS: "Ubuntu 24.04 LTS" },
    { optionToken: "opaque-w22", labelZhCN: "Windows Server 2022", labelEnUS: "Windows Server 2022" },
    { optionToken: "opaque-w25", labelZhCN: "Windows Server 2025", labelEnUS: "Windows Server 2025" },
    { optionToken: "opaque-tencent", labelZhCN: "TencentOS-3", labelEnUS: "TencentOS-3" },
    { optionToken: "opaque-custom", labelZhCN: "专用镜像", labelEnUS: "Custom image" },
  ] },
  { fieldToken: "opaque-region", type: "single_select", presentation: "region", labelZhCN: "区域", labelEnUS: "Region", visible: true, required: true, options: [
    { optionToken: "opaque-xa", labelZhCN: "陕西西安", labelEnUS: "Xi’an" },
    { optionToken: "opaque-cd", labelZhCN: "四川成都", labelEnUS: "Chengdu" },
  ] },
  { fieldToken: "opaque-bandwidth", type: "integer", displayControl: "auto", labelZhCN: "带宽", labelEnUS: "Bandwidth", visible: true, minimumInteger: "20", maximumInteger: "205", quantityStep: "10", unit: "Mbps" },
];
const initial = [
  { fieldToken: "opaque-system", type: "single_select", optionTokens: ["opaque-u22"] },
  { fieldToken: "opaque-region", type: "single_select", optionTokens: ["opaque-xa"] },
  { fieldToken: "opaque-bandwidth", type: "integer", integerValue: "20" },
];
const policy = { hostnameEnabled: true, passwordEnabled: true, hostname: { length: 12, characters: ["lowercase", "digits"] }, password: { length: 16, characters: ["lowercase", "uppercase", "digits", "symbols"] } };

test.describe("TC-14 官网服务器配置组件", () => {
  for (const language of ["zh-CN", "en-US"]) {
    test(`TC-14${language === "zh-CN" ? "a" : "b"}: ${language} 分类、区域、滑条与单台凭据独立保存`, async ({ site, page }) => {
      const ordinary: any[] = [], logins: any[] = [];
      let rejectPrice = false, rejectLogin = false, adds = 0;
      await page.route("**/__website/session", route => route.fulfill({ json: { checked: true, accessToken: "test-session" } }));
      await page.route("**/x/idc-core/api/v1/**", async route => {
        const path = new URL(route.request().url()).pathname;
        const body = route.request().method() === "POST" ? route.request().postDataJSON() : undefined;
        let data: any;
        if (path.endsWith("/catalog/products/server-controls")) data = { product: { nameZhCN: "服务器配置", nameEnUS: "Server configuration", currencies: ["CNY"] } };
        else if (path.endsWith("/server-controls/purchase-configuration")) data = { ready: true, valid: true, configurationRevision: "R", fields, values: initial, serverLoginPolicy: policy };
        else if (path.endsWith("/server-controls/purchase-configuration/server-login")) {
          logins.push(body);
          expect(route.request().headers().authorization).toBe("Bearer test-session");
          if (rejectLogin) return route.fulfill({ status: 503, json: { code: 50 } });
          data = { serverLoginToken: "opaque-login-reference", hostname: body.hostname, passwordConfigured: Boolean(body.password) };
        } else if (path.endsWith("/server-controls/purchase-configuration/evaluate")) {
          ordinary.push(body);
          data = { ready: true, valid: true, configurationRevision: "R", fields, values: body.values, serverLoginPolicy: policy };
        } else if (path.endsWith("/server-controls/purchase-configuration/quote")) {
          ordinary.push(body);
          if (rejectPrice) return route.fulfill({ status: 503, json: { code: 50 } });
          const bandwidth = body.values.find((value: any) => value.fieldToken === "opaque-bandwidth").integerValue;
          data = { configurationRevision: "R", currencyCode: "CNY", terms: [{ billingTermToken: "monthly", minimumPurchaseCycles: 1, status: "final", labelZhCN: "月付", labelEnUS: "Monthly", totalAmount: String(Number(bandwidth) * 100) }] };
        } else if (path.endsWith("/account/commerce/carts")) data = { cart: { cartNo: "cart-one", version: 1, items: [] } };
        else if (path.endsWith("/account/commerce/cart-items")) {
          ordinary.push(body); adds++;
          return route.fulfill({ status: 503, json: { code: 50 } });
        } else return route.fallback();
        await route.fulfill({ json: { code: 0, data } });
      });
      await site.open(`/configure.html?product=server-controls&lang=${language}`);
      await expect(page.locator("#purchase-price")).toHaveText("CNY 20.00");
      await expect.poll(() => ordinary.length).toBeGreaterThan(1);
      const zh = language === "zh-CN";
      await expect(page.locator(".purchase-system-card")).toHaveCount(3);
      await expect(page.locator(".purchase-system-card").last()).toContainText(zh ? "其他系统" : "Other systems");
      await expect(page.locator(".purchase-system-card svg")).toHaveCount(3);
      await expect(page.locator(".purchase-region-options svg")).toHaveCount(2);
      await expect(page.locator(".purchase-slider-limits")).toHaveText("20 Mbps200 Mbps");
      await expect(page.locator('.purchase-slider input[type="number"]')).toHaveAttribute("max", "200");
      await expect(page.locator("#server-login")).toContainText(zh ? "选填，留空则自动生成。" : "Optional. Leave blank to generate automatically.");
      await site.screenshot(`server-controls-${language}-initial`);
      await site.chooseSystemVersion(zh ? "其他系统" : "Other systems", "opaque-tencent");
      await expect(page.locator(".purchase-system-card.is-selected")).toContainText(zh ? "其他系统" : "Other systems");
      await expect.poll(() => ordinary.filter(body => body.values).at(-1)?.values.find((value: any) => value.fieldToken === "opaque-system")?.optionTokens).toEqual(["opaque-tencent"]);
      await site.chooseSystemVersion("Windows", "opaque-w25");
      await site.chooseConfigurationRadio(zh ? "区域" : "Region", zh ? "四川成都" : "Chengdu");
      await site.stepConfiguration(zh ? "增加 · 带宽" : "Increase · Bandwidth");
      await expect(page.locator("#purchase-price")).toHaveText("CNY 30.00");
      const selected = ordinary.filter(body => body.values).at(-1);
      expect(selected.values).toEqual(expect.arrayContaining([
        { fieldToken: "opaque-system", type: "single_select", optionTokens: ["opaque-w25"] },
        { fieldToken: "opaque-region", type: "single_select", optionTokens: ["opaque-cd"] },
        { fieldToken: "opaque-bandwidth", type: "integer", integerValue: "30" },
      ]));
      rejectPrice = true;
      await site.configureNumber(zh ? "带宽" : "Bandwidth", "40");
      await expect(page.locator(".price-status")).toContainText(zh ? "价格暂时无法更新，请重试。" : "Could not update the price");
      await expect(page.locator(".purchase-add")).toBeDisabled();
      await expect(page.locator(".purchase-system-card.is-selected")).toContainText("Windows");
      await site.screenshot(`server-controls-${language}-price-failed`);
      rejectPrice = false;
      await site.retryConfigurationPrice(zh ? "重试" : "Retry");
      await expect(page.locator("#purchase-price")).toHaveText("CNY 40.00");
      await site.serverLoginAction(zh ? "随机生成" : "Generate password");
      const random = await page.locator("#server-password").inputValue();
      expect(random).toMatch(/^(?=.*[a-z])(?=.*[A-Z])(?=.*\d)(?=.*[!@#$%*_=+?\-])[A-Za-z\d!@#$%*_=+?\-]{16}$/);
      await site.serverLoginAction(zh ? "显示密码" : "Show password");
      await expect(page.locator("#server-password")).toHaveAttribute("type", "text");
      await site.serverLoginAction(zh ? "隐藏密码" : "Hide password");
      await site.configureServerLogin("host-one", "TestPassword7!");
      rejectLogin = true;
      await site.addConfiguredProduct(zh ? "加入购物车" : "Add to cart");
      await expect(page.locator(".purchase-form .status")).toContainText(zh ? "操作未完成，请刷新后重试。" : "Could not complete the operation");
      await expect(page.locator("#server-password")).toHaveValue("TestPassword7!");
      expect(adds).toBe(0);
      await site.screenshot(`server-controls-${language}-login-failed`);
      rejectLogin = false;
      await site.addConfiguredProduct(zh ? "加入购物车" : "Add to cart");
      await expect.poll(() => adds).toBe(1);
      await expect(page.locator("#server-password")).toHaveValue("");
      expect(logins.at(-1)).toMatchObject({ configurationRevision: "R", hostname: "host-one", password: "TestPassword7!" });
      const cartRequest = ordinary.at(-1);
      expect(cartRequest).toMatchObject({ serverLoginToken: "opaque-login-reference", quantity: 1 });
      expect(JSON.stringify(ordinary)).not.toContain("TestPassword7!");
      expect(await page.evaluate(() => JSON.stringify({ ...sessionStorage, ...localStorage }))).not.toContain("TestPassword7!");
      await site.addConfiguredProduct(zh ? "加入购物车" : "Add to cart");
      await expect.poll(() => adds).toBe(2);
      expect(logins.length).toBe(2); // Failed create and successful create; cart retry reuses the token.
      await site.open(`/configure.html?product=server-controls&lang=${language}`);
      await expect(page.locator("#purchase-price")).toHaveText("CNY 40.00");
      await expect(page.locator("#server-password")).toHaveAttribute("placeholder", zh ? "密码已设置，留空保留原密码" : "Password is set. Leave blank to keep it.");
      await site.addConfiguredProduct(zh ? "加入购物车" : "Add to cart");
      await expect.poll(() => adds).toBe(3);
      expect(logins).toHaveLength(2);
      await site.configureNumber(zh ? "带宽" : "Bandwidth", "190");
      await expect(page.locator("#purchase-price")).toHaveText("CNY 190.00");
      await site.stepConfiguration(zh ? "增加 · 带宽" : "Increase · Bandwidth");
      await expect(page.locator("#purchase-price")).toHaveText("CNY 200.00");
      await expect(page.locator(".purchase-stepper").getByRole("button", { name: zh ? "增加 · 带宽" : "Increase · Bandwidth" })).toBeDisabled();
      expect(await page.locator('.purchase-slider input[type="number"]').evaluate((input: HTMLInputElement) => input.validity.valid)).toBe(true);
      await site.mobile();
      await expect.poll(() => page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
      await site.screenshot(`server-controls-${language}-mobile`, true);
    });
  }
  test("TC-14c: 编辑主机名保留原密码引用，自填项目不能批量复制", async ({ site, page }) => {
    const logins: any[] = [], updates: any[] = [];
    const item = {
      cartItemNo: "item-one", publicSlug: "server-controls", configurationReference: "snapshot", version: 2, quantity: 1,
      hasServerLogin: true, serverLoginToken: "original-login", serverLoginHostname: "original-host", serverLoginPasswordConfigured: true,
      productNameZhCN: "独立服务器", periodUnitCode: "month", periodCount: 1, purchaseCycles: 1,
    };
    const quote = { quoteNo: "quote", cartVersion: 2, expiresAt: Date.now() + 60000, totalAmountMinor: "2000", currencyCode: "CNY", items: [{ cartItemNo: "item-one", totalAmountMinor: "2000", configurationFrozen: true }] };
    const cart = { cartNo: "cart", version: 2, currencyCode: "CNY", items: [item], latestQuote: quote };
    await page.route("**/__website/session", route => route.fulfill({ json: { checked: true, accessToken: "test-session" } }));
    await page.route("**/x/idc-core/api/v1/**", async route => {
      const path = new URL(route.request().url()).pathname;
      const body = route.request().method() !== "GET" ? route.request().postDataJSON() : undefined;
      let data: any;
      if (path.endsWith("/catalog/products/server-controls")) data = { product: { nameZhCN: "独立服务器", currencies: ["CNY"] } };
      else if (path.endsWith("/server-controls/purchase-configuration")) data = { ready: true, configurationRevision: "R", fields, values: initial, serverLoginPolicy: policy };
      else if (path.endsWith("/server-controls/purchase-configuration/evaluate")) data = { ready: true, valid: true, configurationRevision: "R", fields, values: body.values, serverLoginPolicy: policy };
      else if (path.endsWith("/server-controls/purchase-configuration/quote")) data = { configurationRevision: "R", currencyCode: "CNY", terms: [{ billingTermToken: "monthly", minimumPurchaseCycles: 1, status: "final", totalAmount: "2000" }] };
      else if (path.endsWith("/server-controls/purchase-configuration/server-login")) {
        logins.push(body);
        data = { serverLoginToken: "updated-login", hostname: body.hostname, passwordConfigured: true };
      } else if (path.endsWith("/account/commerce/cart-items/item-one")) {
        updates.push(body);
        return route.fulfill({ status: 503, json: { code: 50 } });
      } else if (path.endsWith("/account/commerce/cart") || path.endsWith("/account/commerce/guest-cart-merge")) data = { cart };
      else if (path.endsWith("/account/marketing/coupons")) data = { assets: [] };
      else return route.fallback();
      await route.fulfill({ json: { code: 0, data } });
    });
    await site.open("/configure.html?product=server-controls&cartItemNo=item-one&lang=zh-CN");
    await expect(page.locator("#purchase-price")).toHaveText("CNY 20.00");
    await expect(page.locator("#server-hostname")).toHaveValue("original-host");
    await expect(page.locator("#server-password")).toHaveAttribute("placeholder", "密码已设置，留空保留原密码");
    await site.addConfiguredProduct("保存修改");
    await expect.poll(() => updates.length).toBe(1);
    expect(logins).toHaveLength(0);
    expect(updates[0].purchase.serverLoginToken).toBe("original-login");
    await site.configureServerLogin("new-host", "");
    await site.addConfiguredProduct("保存修改");
    await expect.poll(() => updates.length).toBe(2);
    expect(logins).toEqual([{ configurationRevision: "R", hostname: "new-host", password: "", previousServerLoginToken: "original-login" }]);
    expect(updates[1].purchase.serverLoginToken).toBe("updated-login");
    expect(updates[1].quantity).toBe(1);
    await site.open("/cart.html?lang=zh-CN");
    await expect(page.locator(".cart-product")).toContainText("original-host");
    await expect(page.locator(".cart-quantity input")).toBeDisabled();
    await expect(page.locator(".quantity-more")).toBeDisabled();
    await expect(page.locator(".quantity-less")).toBeDisabled();
    await site.screenshot("server-controls-single-server-cart");
    item.quantity = 2;
    item.hasServerLogin = false;
    item.serverLoginToken = "";
    item.serverLoginHostname = "";
    item.serverLoginPasswordConfigured = false;
    await site.open("/configure.html?product=server-controls&cartItemNo=item-one&lang=zh-CN");
    await expect(page.locator("#purchase-price")).toHaveText("CNY 20.00");
    await site.configureServerLogin("individual-host", "");
    await site.addConfiguredProduct("保存修改");
    await expect(page.locator(".purchase-form .status")).toContainText("请先在购物车把该项数量改为一台");
    expect(logins).toHaveLength(1);
    expect(updates).toHaveLength(2);
  });
});

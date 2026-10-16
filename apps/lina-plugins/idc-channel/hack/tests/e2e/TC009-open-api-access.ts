import { expect, test } from "../../../../../../hack/tests/fixtures/auth";
import { mkdir } from "node:fs/promises";
import path from "node:path";

test.describe("TC009 customer-bound Open API access", () => {
 test("TC009b real authorization list uses the implemented controller and numeric pagination", async ({ adminPage }) => {
  const responsePromise = adminPage.waitForResponse(response => response.request().method() === "GET" && new URL(response.url()).pathname.endsWith("/admin/channel/accounts"));
  await adminPage.goto("/admin/idc/channel", { waitUntil: "domcontentloaded" });
  const response = await responsePromise;
  const result = await response.json();
  expect(response.ok()).toBe(true);
  expect(result.code).toBe(0);
  expect(result.data.page).toBe(1);
  expect(result.data.pageSize).toBe(10);
  expect(result.data.total).toBeGreaterThanOrEqual(0);
  expect(Array.isArray(result.data.list)).toBe(true);
  expect(result.data.list.length).toBeLessThanOrEqual(10);
  await expect(adminPage.getByText("客户授权", { exact: true })).toBeVisible();
  await expect(adminPage.locator(".vxe-pager--total").first()).toContainText(String(result.data.total));
  await expect(adminPage.getByText("Not Implemented", { exact: true })).toHaveCount(0);
  const directory = path.resolve(process.cwd(), "../../temp/20261012");
  await mkdir(directory, { recursive: true });
  await adminPage.screenshot({ path: path.join(directory, `${Date.now()}-openapi-real-list.png`) });
 });
 test("TC009a grant customer access, recover a failed write and display the secret once", async ({ adminPage }) => {
  let saved: Record<string, unknown> | undefined;
  let fail = true;
  let created = false;
  await adminPage.route("**/x/idc-core/api/v1/admin/customers?**", route => route.fulfill({ json: { code: 0, message: "", data: { customers: [{ customerNo: "customer_test", customerCode: "8001", displayName: "API 测试客户" }] } } }));
  await adminPage.route("**/x/idc-channel/api/v1/admin/channel/**", async route => {
   const url = new URL(route.request().url());
   const payload = route.request().method() === "POST" ? route.request().postDataJSON() : null;
   if (url.pathname.endsWith("/tokens") && payload) {
    saved = payload;
    if (fail) { await route.fulfill({ json: { code: 51, message: "授权保存失败，请检查客户状态后重试。", data: null } }); return; }
    created = true;
    await route.fulfill({ json: { code: 0, message: "", data: { appId: "app_test", token: "e2e-once-secret" } } }); return;
   }
   const list = created && url.pathname.endsWith("/accounts") ? [{ channelId: "app_test", customerNo: "customer_test", displayName: "测试商家系统", statusCode: "approved" }] : [];
   await route.fulfill({ json: { code: 0, message: "", data: { list, total: list.length, page: 1, pageSize: 10 } } });
  });
  const directory = path.resolve(process.cwd(), "../../temp/20261012/openapi"); await mkdir(directory, { recursive: true });
  await adminPage.goto("/admin/idc/channel", { waitUntil: "domcontentloaded" });
  await expect(adminPage.getByText("客户授权", { exact: true })).toBeVisible();
  await expect(adminPage.getByText("给客户开放 API，让对方系统使用该客户的身份下单和管理服务。", { exact: true })).toBeVisible();
  await adminPage.screenshot({ path: path.join(directory, `${Date.now()}-access.png`), fullPage: true });
  await adminPage.getByTestId("openapi-create").click();
  const drawer = adminPage.getByRole("dialog");
  await drawer.getByTestId("openapi-customer").click();
  await adminPage.getByText("API 测试客户 · #8001", { exact: true }).click();
  await drawer.getByTestId("openapi-name").fill("测试商家系统");
  await drawer.getByText("下单并扣余额", { exact: true }).click();
  await expect(drawer).not.toContainText("plugin.idc-channel");
  await adminPage.screenshot({ path: path.join(directory, `${Date.now()}-grant.png`), fullPage: true });
  await drawer.getByRole("button", { name: "生成 AppID 和密钥", exact: true }).click();
  await expect(drawer.getByText("操作失败。请检查客户已激活、可正常登录，且授权处于启用状态，然后重试。", { exact: true })).toBeVisible();
  await expect(drawer.getByTestId("openapi-name")).toHaveValue("测试商家系统");
  await expect(adminPage.getByText("授权保存失败，请检查客户状态后重试。", { exact: true })).toBeVisible();
  await adminPage.screenshot({ path: path.join(directory, `${Date.now()}-failure.png`), fullPage: true });
  await expect(adminPage.getByText("授权保存失败，请检查客户状态后重试。", { exact: true })).not.toBeVisible({ timeout: 10_000 });
  expect(saved?.customerNo).toBe("customer_test"); expect(saved?.scopes).toContain("order.create"); expect(saved?.scopes).not.toContain("*");
  fail = false;
  await drawer.getByRole("button", { name: "生成 AppID 和密钥", exact: true }).click();
  await expect(drawer.getByTestId("openapi-secret")).toHaveText("e2e-once-secret");
  await expect(drawer.getByText("授权已保存，请立即复制密钥", { exact: true })).toBeVisible();
  await adminPage.screenshot({ path: path.join(directory, `${Date.now()}-secret.png`), fullPage: true });
  await drawer.getByRole("button", { name: "我已保存，关闭", exact: true }).first().click();
  await expect(drawer).toBeHidden();
  await expect(adminPage.getByTestId("openapi-secret")).not.toBeVisible();
  await expect(adminPage.getByRole("row").filter({ hasText: "测试商家系统" })).toContainText("app_test");
  await expect(adminPage.getByRole("row").filter({ hasText: "测试商家系统" })).toContainText("启用");
  await adminPage.screenshot({ path: path.join(directory, `${Date.now()}-created-visible-in-grid.png`) });
  await adminPage.getByRole("tab", { name: "接入说明", exact: true }).click();
  await expect(adminPage.getByText("1. 填写连接信息", { exact: true })).toBeVisible();
  await expect(adminPage.getByText("2. 选商品、取得报价，再下单", { exact: true })).toBeVisible();
  await expect(adminPage.getByText("使用下方需要 AppID 和密钥认证的商品配置及计费周期接口。", { exact: false })).toBeAttached();
  await adminPage.setViewportSize({ width: 780, height: 900 });
  await adminPage.screenshot({ path: path.join(directory, `${Date.now()}-guide-narrow.png`), fullPage: true });
 });
});

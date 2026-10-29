// Isolated HTTP fixtures exercise the retained consent UI, without a real login or supplier write.
import { test, expect } from "@host-tests/fixtures/auth";
import { LoginPage } from "@host-tests/pages/LoginPage";
import { VerificationConsentPage } from "../pages/VerificationConsentPage";

test.describe("TC-8 认证授权记录", () => {
test("TC-8a: 授权记录收进认证页，按需读取并确认撤回，不再出现独立治理入口", async ({ page }, testInfo) => {
  test.setTimeout(60_000);
  const origin = process.env.CUSTOMER_E2E_BASE_URL || "http://127.0.0.1:6032";
  let reads = 0;
  let withdrawn = false;
  const writes: Record<string, unknown>[] = [];
  const consent = () => ({ consentNo: "PVC_fixture", consentTypeCode: "sensitive_personal_information", actionCode: withdrawn ? "withdrawn" : "granted", decidedAt: 1792627200000 });
  await page.route("**/api/v1/auth/login", r => r.fulfill({ json: { code: 0, data: { accessToken: "fixture-consent", refreshToken: "fixture-refresh", tenants: [] } } }));
  await page.route("**/api/v1/user/info", r => r.fulfill({ json: { code: 0, data: { id: 708, username: "consent-fixture", realName: "授权测试客户", roles: ["user"], permissions: [], menus: [], homePath: "/account/verification" } } }));
  await page.route("**/api/v1/plugins/dynamic", r => r.fulfill({ json: { code: 0, data: { list: [] } } }));
  await page.route("**/customer/external-menus", r => r.fulfill({ json: { code: 0, data: { items: [] } } }));
  await page.route("**/account/product-directories?*", r => r.fulfill({ json: { code: 0, data: { items: [], total: 0 } } }));
  await page.route("**/account/compliance/personal-verification", r => r.fulfill({ json: { code: 0, data: { policyVersion: "v1", item: null } } }));
  await page.route("**/account/compliance/privacy", r => { reads++; return reads === 1 ? r.fulfill({ status: 503, json: { code: 50, message: "Temporary failure" } }) : r.fulfill({ json: { code: 0, data: { consents: [consent()] } } }); });
  await page.route("**/account/compliance/privacy/consents/withdraw", r => { writes.push(r.request().postDataJSON()); withdrawn = true; return r.fulfill({ json: { code: 0, data: { consent: consent() } } }); });
  await page.route("**/account/initial-password/status", r => r.fulfill({ json: { code: 0, data: { required: false } } }));
  await page.route("**/account/customer/profile", r => r.fulfill({ json: { code: 0, data: { item: { displayName: "授权测试客户" } } } }));
  await page.goto(origin + "/auth/login");
  await new LoginPage(page).login("consent-fixture", "fixture-password");
  const verification = new VerificationConsentPage(page);
  await expect(verification.openButton).toBeVisible();
  expect(reads).toBe(0);
  await expect(page.locator('a[href="/account/privacy"]')).toHaveCount(0);
  await expect(page.locator('a[href^="/account/contracts"]')).toHaveCount(0);
  await verification.open();
  await expect(verification.panel).toContainText("授权记录加载失败");
  await verification.panel.getByRole("button", { name: "重新加载" }).click();
  await expect(verification.panel).toContainText("敏感个人信息");
  await expect(verification.panel).not.toContainText("申请导出");
  await expect(verification.panel).not.toContainText("申请删除");
  await verification.withdraw();
  expect(writes).toHaveLength(0);
  await verification.confirmWithdrawal();
  await expect(verification.panel).toContainText("已撤回");
  expect(writes).toHaveLength(1);
  expect(Object.keys(writes[0]).sort()).toEqual(["consentTypeCode", "idempotencyKey"]);
  await expect(page.locator(".ant-notification-notice, .ant-message-notice")).toHaveCount(0);
  await page.screenshot({ path: testInfo.outputPath(`${new Date().toISOString().slice(11,19).replaceAll(":", "")}-verification-consent-retained.png`), animations: "disabled" });
});
});

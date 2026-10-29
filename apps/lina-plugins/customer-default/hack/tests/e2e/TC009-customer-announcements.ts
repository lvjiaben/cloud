import type { Page } from "@host-tests/support/playwright";
import { test, expect } from "@host-tests/fixtures/auth";
import { LoginPage } from "@host-tests/pages/LoginPage";
import { CustomerAnnouncementsPage } from "../pages/CustomerAnnouncementsPage";
const origin = process.env.CUSTOMER_E2E_BASE_URL || "http://127.0.0.1:6032";
async function login(page: Page) {
  await page.route("**/api/v1/i18n/runtime/locales?*", r => r.fulfill({ json: { code: 0, data: { enabled: true, locale: "zh-CN", items: [{ locale: "zh-CN", nativeName: "简体中文", isDefault: true }, { locale: "en-US", nativeName: "English" }] } } }));
  await page.route("**/api/v1/i18n/runtime/messages?*", r => r.fulfill({ json: { code: 0, data: { messages: { fixtureReady: "loaded" } } } }));
  await page.route("**/api/v1/auth/login", r => r.fulfill({ json: { code: 0, data: { accessToken: "fixture-announcement", refreshToken: "fixture-refresh", tenants: [] } } }));
  await page.route("**/api/v1/user/info", r => r.fulfill({ json: { code: 0, data: { id: 709, username: "announcement-fixture", realName: "公告测试客户", roles: ["user"], permissions: [], menus: [], homePath: "/account/ssh-keys" } } }));
  await page.route("**/api/v1/plugins/dynamic", r => r.fulfill({ json: { code: 0, data: { list: [] } } }));
  await page.route("**/customer/external-menus", r => r.fulfill({ json: { code: 0, data: { items: [] } } }));
  await page.route("**/account/product-directories?*", r => r.fulfill({ json: { code: 0, data: { items: [], total: 0 } } }));
  await page.route("**/account/initial-password/status", r => r.fulfill({ json: { code: 0, data: { required: false } } }));
  await page.route("**/account/customer/profile", r => r.fulfill({ json: { code: 0, data: { item: { displayName: "公告测试客户" } } } }));
  await page.route("**/api/v1/user/profile", r => r.fulfill({ json: { code: 0, data: { id: 709, username: "announcement-fixture", realName: "公告测试客户" } } }));
  await page.route("**/account/ssh-public-keys", r => r.fulfill({ json: { code: 0, data: { items: [] } } }));
  await page.goto(origin + "/auth/login");
  await new LoginPage(page).login("announcement-fixture", "fixture-password");
}

test.describe("TC-9 客户公告", () => {
  test.setTimeout(60000);
  test("TC-9a: 顶部直接打开公告，分页与正文图片、已读状态正确，中英文切换正常", async ({ page }, testInfo) => {
    let read = false;
    const pages: string[] = [];
    const retired: string[] = [];
    page.on("request", r => { if (/account\/notifications|user\/message/.test(r.url())) retired.push(r.url()); });
    await page.route("**/x/content-notice/api/v1/notice/announcements?*", r => {
      const current = new URL(r.request().url()).searchParams.get("pageNum")!;
      pages.push(current);
      return r.fulfill({ json: { code: 0, data: { list: [{ id: current === "1" ? 321 : 322, title: current === "1" ? "维护公告" : "后续公告", read: current === "1" && read, createdAt: 1792640000000 }], total: 11 } } });
    });
    await page.route("https://example.test/announcement.png", r => r.fulfill({ contentType: "image/svg+xml", body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="120"><rect width="400" height="120" fill="#edf4ff"/><text x="24" y="68" font-size="24" fill="#1677ff">Service maintenance</text></svg>' }));
    await page.route("**/x/content-notice/api/v1/notice/announcements/321", r => r.fulfill({ json: { code: 0, data: { id: 321, title: "维护公告", read, createdAt: 1792640000000, content: '<p>今晚进行服务维护。</p><img alt="公告配图" src="https://example.test/announcement.png" onerror="window.announcementInjected=true"><script>window.announcementInjected=true</script>' } } }));
    await page.route("**/x/content-notice/api/v1/notice/announcements/321/read", r => { read = true; expect(r.request().method()).toBe("PUT"); return r.fulfill({ json: { code: 0, data: {} } }); });
    await login(page);
    const view = new CustomerAnnouncementsPage(page);
    expect(pages).toEqual([]);
    await view.open();
    await expect(view.list).toContainText("未读");
    await expect(page).toHaveURL(/\/account\/ssh-keys$/);
    await page.screenshot({ path: testInfo.outputPath("100000-announcement-list.png"), animations: "disabled" });
    await view.select("维护公告");
    await expect(view.detail).toContainText("今晚进行服务维护。");
    await expect(view.detail).toContainText("2026-10-22");
    await expect(view.detail.getByRole("img", { name: "公告配图" })).toBeVisible();
    expect(await page.evaluate(() => (window as any).announcementInjected)).toBeUndefined();
    await page.screenshot({ path: testInfo.outputPath("100001-announcement-detail.png"), animations: "disabled" });
    await view.closeDetail();
    await expect(view.list).toContainText("已读");
    await view.nextPage();
    await expect(view.list).toContainText("后续公告");
    expect(pages).toEqual(["1", "2"]);
    expect(retired).toEqual([]);
    await view.closeList();
    await new LoginPage(page).switchLanguage("English");
    await view.open("Announcements");
    await expect(view.list).toContainText("Read");
    await expect(view.list).not.toContainText("announcements.");
    await page.screenshot({ path: testInfo.outputPath("100002-announcement-english.png"), animations: "disabled" });
  });

  test("TC-9b: 公告读取失败可重试，已读保存失败不隐藏正文和误标状态", async ({ page }, testInfo) => {
    let fail = true;
    await page.route("**/x/content-notice/api/v1/notice/announcements?*", r => fail ? r.fulfill({ status: 503, json: { code: 503 } }) : r.fulfill({ json: { code: 0, data: { list: [{ id: 321, title: "维护公告", read: false, createdAt: 1792640000000 }], total: 1 } } }));
    await page.route("**/x/content-notice/api/v1/notice/announcements/321", r => r.fulfill({ json: { code: 0, data: { id: 321, title: "维护公告", read: false, createdAt: 1792640000000, content: "<p>公告正文</p>" } } }));
    await page.route("**/x/content-notice/api/v1/notice/announcements/321/read", r => r.fulfill({ status: 503, json: { code: 503 } }));
    await login(page);
    const view = new CustomerAnnouncementsPage(page);
    await view.open();
    await expect(view.list).toContainText("公告加载失败，请重试。");
    fail = false;
    await view.retry();
    await view.select("维护公告");
    await expect(view.detail).toContainText("公告正文");
    await expect(page.getByText("已读状态保存失败，请关闭后重试。", { exact: true })).toBeVisible();
    await page.screenshot({ path: testInfo.outputPath("100003-announcement-read-failed.png"), animations: "disabled" });
    await view.closeDetail();
    await expect(view.list).toContainText("未读");
  });
});

import { test, expect } from "../support/fixtures";

test.describe("TC-12 官网公告同源展示", () => {
  test("TC-12a: 公告读取统一数据，分页及详情正文图片正确且不执行HTML脚本", async ({ site, page }) => {
    const pages: string[] = [];
    let retired = 0;
    page.on("request", r => { if (/content\/items\?.*type=announcement/.test(r.url())) retired++; });
    await page.route("**/x/content-notice/api/v1/public/announcements?*", r => {
      const current = new URL(r.request().url()).searchParams.get("pageNum")!;
      pages.push(current);
      return r.fulfill({ json: { code: 0, data: { list: [{ id: current === "1" ? 321 : 322, title: current === "1" ? "维护公告" : "后续公告", createdAt: 1792640000000 }], total: 13 } } });
    });
    await page.route("**/x/content-notice/api/v1/public/announcements/321", r => r.fulfill({ json: { code: 0, data: { id: 321, title: "维护公告", createdAt: 1792640000000, content: '<p>今晚进行服务维护。</p><img alt="公告配图" src="https://example.test/announcement.png" onerror="window.announcementInjected=true"><script>window.announcementInjected=true</script>' } } }));
    await page.route("https://example.test/announcement.png", r => r.fulfill({ contentType: "image/svg+xml", body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="120"><rect width="400" height="120" fill="#edf4ff"/><text x="24" y="68" font-size="24" fill="#1677ff">Service maintenance</text></svg>' }));
    await site.open("/announcements.html");
    await expect(page.locator("#content-items")).toContainText("维护公告");
    await expect(page.locator("#content-search")).toBeHidden();
    await site.moreContent();
    await expect(page.locator("#content-items")).toContainText("后续公告");
    expect(pages).toEqual(["1", "2"]);
    await site.screenshot("announcements-list");
    await site.openArticle("维护公告");
    await expect(page.locator("#article-title")).toHaveText("维护公告");
    await expect(page.locator("#article-back")).toContainText("返回列表");
    await expect(page.locator("#article-date")).toContainText("2026-10-22");
    await expect(page.locator("#article-body")).toContainText("今晚进行服务维护。");
    await expect(page.getByRole("img", { name: "公告配图" })).toBeVisible();
    expect(await page.evaluate(() => (window as any).announcementInjected)).toBeUndefined();
    expect(retired).toBe(0);
    await site.screenshot("announcement-detail");
  });
  test("TC-12b: 下线公告不再展示旧标题与正文", async ({ site, page }) => {
    await page.route("**/x/content-notice/api/v1/public/announcements/321", r => r.fulfill({ json: { code: 65, data: null, errorCode: "CONTENT_NOTICE_NOT_FOUND", message: "Announcement not found" } }));
    await site.open("/article.html?kind=announcements&id=321");
    await expect(page.locator("#article-status")).toContainText("内容不存在或已下架");
    await expect(page.locator("#article")).toBeHidden();
    await site.screenshot("announcement-unavailable");
  });
});

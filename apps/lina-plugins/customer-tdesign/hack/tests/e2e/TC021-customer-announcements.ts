import { test, expect, now } from "../support/fixture";
import { CustomerPage } from "../pages/CustomerPage";

test.describe("TC-21 客户公告", () => {
  test("TC-21a 顶部公告直接打开分页列表，详情显示正文图片并持久已读", async ({
    consolePage: page,
  }) => {
    let read = false,
      reads = 0;
    const requestedPages: string[] = [];
    const retiredRequests: string[] = [];
    page.on("request", (request) => {
      if (/account\/notifications|user\/message/.test(request.url()))
        retiredRequests.push(request.url());
    });
    await page.route(
      "**/x/content-notice/api/v1/notice/announcements?*",
      (route) => {
        const current = new URL(route.request().url()).searchParams.get(
          "pageNum",
        )!;
        requestedPages.push(current);
        return route.fulfill({
          json: {
            code: 0,
            data: {
              list: [
                {
                  id: current === "1" ? 321 : 322,
                  title: current === "1" ? "维护公告" : "后续公告",
                  read: current === "1" && read,
                  createdAt: now,
                },
              ],
              total: 11,
            },
          },
        });
      },
    );
    await page.route("https://example.test/announcement.png", (r) =>
      r.fulfill({
        contentType: "image/svg+xml",
        body: '<svg xmlns="http://www.w3.org/2000/svg" width="400" height="120"><rect width="400" height="120" fill="#edf4ff"/><text x="24" y="68" font-size="24" fill="#1677ff">Service maintenance</text></svg>',
      }),
    );
    await page.route(
      "**/x/content-notice/api/v1/notice/announcements/321",
      (route) =>
        route.fulfill({
          json: {
            code: 0,
            data: {
              id: 321,
              title: "维护公告",
              read,
              createdAt: now,
              content:
                '<p>服务将于今晚维护。</p><img alt="公告配图" src="https://example.test/announcement.png" onerror="window.announcementInjected=true"><script>window.announcementInjected=true</script>',
            },
          },
        }),
    );
    await page.route(
      "**/x/content-notice/api/v1/notice/announcements/321/read",
      (route) => {
        reads++;
        read = true;
        expect(route.request().method()).toBe("PUT");
        return route.fulfill({ json: { code: 0, data: {} } });
      },
    );
    const view = new CustomerPage(page);
    await view.open();
    await expect(view.button("公告")).toBeVisible();
    expect(requestedPages).toHaveLength(0);
    await view.openAnnouncements();
    await expect(view.announcements).toContainText("维护公告");
    await expect(view.announcements).toContainText("未读");
    await expect(page).toHaveURL(/\/account$/);
    await view.screenshot("announcements-list");
    await view.openAnnouncement("维护公告");
    await expect(view.announcementDetail).toContainText("服务将于今晚维护。");
    await expect(view.announcementDetail).toContainText("2026/10/22");
    await expect(
      view.announcementDetail.getByRole("img", { name: "公告配图" }),
    ).toBeVisible();
    await expect.poll(() => reads).toBe(1);
    expect(
      await page.evaluate(() => (window as any).announcementInjected),
    ).toBeUndefined();
    await view.screenshot("announcement-detail-image");
    await view.closeAnnouncement();
    await expect(view.announcements).toContainText("已读");
    await view.nextAnnouncementsPage();
    await expect(view.announcements).toContainText("后续公告");
    expect(requestedPages).toEqual(["1", "2"]);
    expect(retiredRequests).toEqual([]);
    await view.screenshot("announcements-page-two");
    await view.open("/account/notifications");
    await expect(page.getByText("通知设置", { exact: true })).toHaveCount(0);
    await expect(page.getByText("通知发送记录", { exact: true })).toHaveCount(
      0,
    );
  });

  test("TC-21b 列表失败可重试，详情失败不标已读，已读写入失败保留未读", async ({
    consolePage: page,
  }) => {
    let failList = true,
      failDetail = true,
      reads = 0;
    await page.route(
      "**/x/content-notice/api/v1/notice/announcements?*",
      (route) =>
        failList
          ? route.fulfill({
              status: 503,
              json: { code: 503, message: "公告暂时不可用" },
            })
          : route.fulfill({
              json: {
                code: 0,
                data: {
                  list: [
                    { id: 321, title: "维护公告", read: false, createdAt: now },
                  ],
                  total: 1,
                },
              },
            }),
    );
    await page.route(
      "**/x/content-notice/api/v1/notice/announcements/321",
      (route) =>
        failDetail
          ? route.fulfill({
              status: 404,
              json: { code: 404, message: "公告不存在" },
            })
          : route.fulfill({
              json: {
                code: 0,
                data: {
                  id: 321,
                  title: "维护公告",
                  read: false,
                  createdAt: now,
                  content: "<p>仍可阅读的公告正文</p>",
                },
              },
            }),
    );
    await page.route(
      "**/x/content-notice/api/v1/notice/announcements/321/read",
      (route) => {
        reads++;
        return route.fulfill({
          status: 503,
          json: { code: 503, message: "稍后重试" },
        });
      },
    );
    const view = new CustomerPage(page);
    await view.open();
    await view.openAnnouncements();
    await expect(
      page.getByText("公告暂时不可用", { exact: true }),
    ).toBeVisible();
    await view.screenshot("announcements-list-error");
    failList = false;
    await view.button("重新加载").click();
    await view.openAnnouncement("维护公告");
    await expect(page.getByText("公告不存在", { exact: true })).toBeVisible();
    expect(reads).toBe(0);
    await page
      .getByRole("dialog")
      .last()
      .getByRole("button", { name: "关闭弹窗" })
      .click();
    failDetail = false;
    await view.openAnnouncement("维护公告");
    await expect(view.announcementDetail).toContainText("仍可阅读的公告正文");
    await expect(
      page.getByText("已读状态保存失败，请关闭后重试。", { exact: true }),
    ).toBeVisible();
    await view.screenshot("announcement-read-error");
    await view.closeAnnouncement();
    await expect(view.announcements).toContainText("未读");
    expect(reads).toBe(1);
  });
  test("TC-21c 已读保存等待期间关闭详情，迟到成功仍更新列表状态", async ({
    consolePage: page,
  }) => {
    let read = false;
    let reads = 0;
    let finishRead: (() => Promise<void>) | undefined;
    await page.route(
      "**/x/content-notice/api/v1/notice/announcements?*",
      (route) =>
        route.fulfill({
          json: {
            code: 0,
            data: {
              list: [{ id: 321, title: "维护公告", read, createdAt: now }],
              total: 1,
            },
          },
        }),
    );
    await page.route(
      "**/x/content-notice/api/v1/notice/announcements/321",
      (route) =>
        route.fulfill({
          json: {
            code: 0,
            data: {
              id: 321,
              title: "维护公告",
              content: "<p>延迟保存阅读状态</p>",
              read,
              createdAt: now,
            },
          },
        }),
    );
    await page.route(
      "**/x/content-notice/api/v1/notice/announcements/321/read",
      (route) => {
        reads++;
        finishRead = async () => {
          read = true;
          await route.fulfill({ json: { code: 0, data: {} } });
        };
      },
    );
    const view = new CustomerPage(page);
    await view.open();
    await view.openAnnouncements();
    await view.openAnnouncement("维护公告");
    await expect(view.announcementDetail).toContainText("延迟保存阅读状态");
    await expect.poll(() => !!finishRead).toBe(true);
    await view.closeAnnouncement();
    await expect(view.announcementDetail).toHaveCount(0);
    await expect(view.announcements).toContainText("未读");
    await finishRead!();
    await expect(view.announcements).toContainText("已读");
    await expect(view.announcements).not.toContainText("未读");
    expect(reads).toBe(1);
    await view.screenshot("announcements-late-read-after-close");
  });
});

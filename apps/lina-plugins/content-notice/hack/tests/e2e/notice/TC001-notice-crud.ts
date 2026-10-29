import { test, expect } from "@host-tests/fixtures/auth";
import { NoticePage } from "../../pages/NoticePage";

test.describe("TC001 通知公告 CRUD", () => {

  test('TC-1a: 独立完成公告新增修改删除', async ({ adminPage }) => {
    const noticePage = new NoticePage(adminPage);
    const title = `公告测试_${Date.now()}`;
    const updated = `${title}_修改`;
    await noticePage.goto();
    try {
      await noticePage.createNotice(title, '草稿', '测试正文');
      expect(await noticePage.hasNotice(title)).toBe(true);
      await noticePage.editNotice(title, updated);
      expect(await noticePage.hasNotice(updated)).toBe(true);
      await noticePage.deleteNotice(updated);
      expect(await noticePage.hasNotice(updated)).toBe(false);
    } finally {
      await noticePage.deleteNoticeIfExists(title);
      await noticePage.deleteNoticeIfExists(updated);
    }
  });

  test("TC001e: 内置通知公告首屏不再等待动态插件沙箱", async ({
    adminPage,
  }, testInfo) => {
    const requestStartedAt = new Map<object, number>();
    const noticeApiDurations: number[] = [];
    adminPage.on("request", (request) => {
      const path = new URL(request.url()).pathname;
      if (path.includes("/content-notice/") && path.endsWith("/notice")) {
        requestStartedAt.set(request, Date.now());
      }
    });
    adminPage.on("response", (response) => {
      const request = response.request();
      const startedAt = requestStartedAt.get(request);
      if (startedAt !== undefined) {
        noticeApiDurations.push(Date.now() - startedAt);
      }
    });

    const routeStartedAt = Date.now();
    await adminPage.goto("/system/notice", { waitUntil: "domcontentloaded" });
    await adminPage
      .getByTestId("notice-management-page")
      .waitFor({ state: "visible", timeout: 8_000 });
    await adminPage
      .locator(".vxe-body--row:visible")
      .first()
      .or(adminPage.locator(".vxe-table--empty-placeholder:visible").first())
      .waitFor({ state: "visible", timeout: 8_000 });
    const firstTableStateVisibleMs = Date.now() - routeStartedAt;

    const sourceResourceDurations = await adminPage.evaluate(() =>
      performance
        .getEntriesByType("resource")
        .filter((entry) => entry.name.includes("content-notice"))
        .map((entry) => ({ duration: entry.duration, name: entry.name })),
    );
    await testInfo.attach("content-notice-first-screen-timing.json", {
      body: Buffer.from(
        JSON.stringify(
          {
            firstTableStateVisibleMs,
            noticeApiDurations,
            sourceResourceDurations,
          },
          null,
          2,
        ),
      ),
      contentType: "application/json",
    });

    expect(noticeApiDurations.length).toBeGreaterThan(0);
    expect(Math.max(...noticeApiDurations)).toBeLessThan(3_000);
    expect(firstTableStateVisibleMs).toBeLessThan(8_000);
    await expect(
      adminPage.locator('[data-testid="plugin-sandbox-frame"]'),
    ).toHaveCount(0);
  });
});

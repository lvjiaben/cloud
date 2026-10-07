import { test, expect } from "@host-tests/fixtures/auth";
import { prepareSourcePluginsBaseline } from "@host-tests/fixtures/plugin";
import { NoticePage } from "../../pages/NoticePage";

test.describe("TC001 通知公告 CRUD", () => {
  test.beforeAll(async () => {
    await prepareSourcePluginsBaseline(["content-notice"]);
  });

  const testTitle = `测试通知_${Date.now()}`;
  const testTitleRenamed = `${testTitle}_修改`;

  test("TC001a: 创建新通知公告", async ({ adminPage }) => {
    const noticePage = new NoticePage(adminPage);
    await noticePage.goto();
    await noticePage.createNotice(testTitle, "通知", "草稿", "这是测试内容");

    await expect(adminPage.getByText(/新增成功|创建成功|success/i)).toBeVisible(
      { timeout: 5000 },
    );
  });

  test("TC001b: 通知公告列表中可见新创建的记录", async ({ adminPage }) => {
    const noticePage = new NoticePage(adminPage);
    await noticePage.goto();

    const hasNotice = await noticePage.hasNotice(testTitle);
    expect(hasNotice).toBeTruthy();
  });

  test("TC001c: 编辑通知公告", async ({ adminPage }) => {
    const noticePage = new NoticePage(adminPage);
    await noticePage.goto();
    await noticePage.editNotice(testTitle, testTitleRenamed);

    await expect(adminPage.getByText(/更新成功|success/i)).toBeVisible({
      timeout: 5000,
    });
  });

  test("TC001d: 删除通知公告", async ({ adminPage }) => {
    const noticePage = new NoticePage(adminPage);
    await noticePage.goto();
    await noticePage.deleteNotice(testTitleRenamed);

    await expect(adminPage.getByText(/删除成功|success/i)).toBeVisible({
      timeout: 5000,
    });
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

import { readFile } from "node:fs/promises";
import { test, expect, article } from "../support/fixtures";

async function downloadArticle(page: import("@playwright/test").Page) {
  await page.route("**/content/items/DL_REVIEW", (route) =>
    route.fulfill({
      json: {
        code: 0,
        data: {
          item: {
            ...article,
            type: "download",
            contentNo: "DL_REVIEW",
            currentFileNo: "FILE_REVIEW",
          },
        },
      },
    }),
  );
}

test.describe("TC-3 下载传输失败与安全边界", () => {
  test("TC-3a: 文件错误和断网留在详情，重试保存正确文件", async ({
    page,
    site,
  }) => {
    await downloadArticle(page);
    await page.route("**/content/items/DL_REVIEW/download-tickets", (route) =>
      route.fulfill({ json: { code: 0, data: { token: "review-token" } } }),
    );
    let mode = "unavailable";
    await page.route("**/content/downloads/review-token", (route) => {
      if (mode === "offline") return route.abort("connectionfailed");
      if (mode === "unavailable")
        return route.fulfill({ status: 503, json: { code: 503 } });
      if (mode === "json")
        return route.fulfill({ json: { code: 403, message: "unavailable" } });
      return route.fulfill({
        body: "verified file bytes",
        contentType: "application/octet-stream",
        headers: {
          "content-disposition":
            "attachment; filename=fallback.txt; filename*=UTF-8''%E5%B9%B3%E5%8F%B0%E6%89%8B%E5%86%8C.txt",
        },
      });
    });
    let downloads = 0;
    page.on("download", () => downloads++);
    await site.open("/article.html?kind=downloads&id=DL_REVIEW");
    for (mode of ["unavailable", "json", "offline"]) {
      await site.download();
      await expect(page.locator("#article-status")).toContainText(
        "文件暂时无法下载",
      );
      await expect(page.locator("#download")).toBeEnabled();
      await expect(page).toHaveURL(
        /article\.html\?kind=downloads&id=DL_REVIEW/u,
      );
      expect(downloads).toBe(0);
    }
    await site.screenshot("download-network-retry");
    mode = "success";
    const pending = page.waitForEvent("download");
    await site.retry("article-status");
    const file = await pending;
    expect(file.suggestedFilename()).toBe("平台手册.txt");
    expect(await readFile((await file.path())!, "utf8")).toBe(
      "verified file bytes",
    );
    await expect(page.locator("#download")).toBeEnabled();
  });

  test("TC-3b: 不可信票据不能拼接路径或发起文件请求", async ({
    page,
    site,
  }) => {
    await downloadArticle(page);
    let token: unknown = {};
    let requests = 0;
    await page.route("**/content/items/DL_REVIEW/download-tickets", (route) =>
      route.fulfill({ json: { code: 0, data: { token } } }),
    );
    page.on("request", (request) => {
      if (request.url().includes("/content/downloads/")) requests++;
    });
    await site.open("/article.html?kind=downloads&id=DL_REVIEW");
    for (token of [
      {},
      "../items/private",
      "https://example.test/file",
      "",
      "a".repeat(129),
    ]) {
      await site.download();
      await expect(page.locator("#article-status")).toContainText(
        "文件暂时无法下载",
      );
      await expect(page.locator("#download")).toBeEnabled();
    }
    expect(requests).toBe(0);
    await expect(page).toHaveURL(/article\.html/u);
  });

  test("TC-3c: 文件请求不携带浏览器会话且下载期间不能重复提交", async ({
    page,
    site,
    context,
    baseURL,
  }) => {
    await downloadArticle(page);
    await context.addCookies([
      { name: "session", value: "test-only", url: baseURL! },
    ]);
    await page.route("**/content/items/DL_REVIEW/download-tickets", (route) =>
      route.fulfill({ json: { code: 0, data: { token: "review-token" } } }),
    );
    let release!: () => void;
    const gate = new Promise<void>((done) => {
      release = done;
    });
    let headers: Record<string, string> | undefined;
    await page.route("**/content/downloads/review-token", async (route) => {
      headers = await route.request().allHeaders();
      await gate;
      await route.fulfill({
        body: "anonymous",
        headers: { "content-disposition": "attachment; filename=guide.txt" },
      });
    });
    await site.open("/article.html?kind=downloads&id=DL_REVIEW");
    await site.download();
    await expect.poll(() => headers).toBeDefined();
    expect(headers!.cookie).toBeUndefined();
    expect(headers!.authorization).toBeUndefined();
    await expect(page.locator("#download")).toBeDisabled();
    await expect(page.locator("#article-status")).toHaveText(
      "正在准备下载，请稍候…",
    );
    const pending = page.waitForEvent("download");
    release();
    await pending;
    await expect(page.locator("#download")).toBeEnabled();
  });
});

import { test, expect } from "../support/fixtures";
test.describe("TC013 内容富文本展示", () => {
  test("TC013a: 新闻正文保留富文本且过滤脚本", async ({ site, page }) => {
    await page.route(
      "**/x/idc-core/api/v1/public/content/items/CNT_RICH",
      (route) =>
        route.fulfill({
          json: {
            code: 0,
            data: {
              item: {
                contentNo: "CNT_RICH",
                type: "news",
                title: "富文本新闻",
                summary: "新闻描述",
                body: "<p>正文<strong>重点</strong></p><ul><li>要点一</li></ul><script>window.contentInjected=true</script>",
                publishedAt: 1792640000000,
              },
            },
          },
        }),
    );
    await site.open("/article.html?kind=news&id=CNT_RICH");
    await expect(page.locator("#article-title")).toHaveText("富文本新闻");
    await expect(page.locator("#article-body strong")).toHaveText("重点");
    await expect(page.locator("#article-body li")).toHaveText("要点一");
    expect(
      await page.evaluate(() => (window as any).contentInjected),
    ).toBeUndefined();
    await site.screenshot("news-rich-content");
  });
});

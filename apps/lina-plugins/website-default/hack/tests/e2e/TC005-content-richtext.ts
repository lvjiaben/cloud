import { test, expect } from "@host-tests/fixtures/auth";
import { createServer } from "node:http";
import { spawn, type ChildProcess } from "node:child_process";
import { once } from "node:events";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
const templateID = process.env.E2E_WEBSITE_TEMPLATE || "website-default";
const pluginRoot = resolve(
  fileURLToPath(new URL("../../../../", import.meta.url)),
  templateID,
);
let origin = "",
  renderer: ChildProcess;
const api = createServer((req, res) => {
  const path = new URL(req.url!, "http://local").pathname;
  let data: unknown;
  if (path.endsWith("/public/brand"))
    data = {
      brand: {
        siteName: { zhCN: "云服务平台", enUS: "Cloud services" },
        footer: { zhCN: "服务支持", enUS: "Support" },
        navigation: [],
        footerNavigation: [],
      },
    };
  else if (path.includes("/content/items/"))
    data = {
      item: {
        contentNo: "CNT_RICH",
        type: "news",
        title: "富文本新闻",
        summary: "新闻描述",
        status: "published",
        visibility: "public",
        body: path.endsWith("CNT_PLAIN")
          ? "第一行\n第二行"
          : '<p>正文<strong>重点</strong></p><ul><li>要点一</li></ul><a href="javascript:window.contentInjected=true">链接</a><script>window.contentInjected=true</script>',
        publishedAt: 1792640000000,
        updatedAt: 1792640000000,
      },
    };
  else if (path.endsWith("/campaigns")) data = { campaigns: [] };
  else data = { items: [], nextCursor: "" };
  res.setHeader("content-type", "application/json");
  res.end(JSON.stringify({ code: 0, data }));
});
test.describe("TC005 内容富文本展示", () => {
  test.beforeAll(async () => {
    api.listen(0, "127.0.0.1");
    await once(api, "listening");
    const probe = createServer();
    probe.listen(0, "127.0.0.1");
    await once(probe, "listening");
    const port = (probe.address() as any).port;
    await new Promise<void>((done) => probe.close(() => done()));
    origin = `http://127.0.0.1:${port}`;
    renderer = spawn(
      process.execPath,
      [resolve(pluginRoot, "frontend/site/.output/server/index.mjs")],
      {
        env: {
          ...process.env,
          PORT: String(port),
          HOST: "127.0.0.1",
          NUXT_API_ORIGIN: `http://127.0.0.1:${(api.address() as any).port}`,
        },
        stdio: "pipe",
      },
    );
    renderer.stderr?.on("data", (data) => process.stderr.write(data));
    await expect
      .poll(
        async () => {
          try {
            return (await fetch(origin + "/__site/health")).status;
          } catch {
            return 0;
          }
        },
        { timeout: 20000 },
      )
      .toBe(200);
  });
  test.afterAll(async () => {
    renderer?.kill();
    await new Promise<void>((done) => api.close(() => done()));
  });
  test("TC005a: 新闻保留段落强调和列表，过滤脚本并兼容纯文本换行", async ({
    page,
  }, testInfo) => {
    const html = await (await fetch(origin + "/pages/news/CNT_RICH")).text();
    expect(html).toContain("<strong>重点</strong>");
    expect(html).not.toContain('<a href="javascript:window.contentInjected');
    await page.goto(origin + "/pages/news/CNT_RICH");
    const detail = page.getByTestId("public-content-detail");
    await expect(detail.locator("strong")).toHaveText("重点");
    await expect(detail.locator("li")).toHaveText("要点一");
    await expect(detail.locator('a[href^="javascript:"]')).toHaveCount(0);
    expect(
      await page.evaluate(() => (window as any).contentInjected),
    ).toBeUndefined();
    await page.screenshot({
      path: testInfo.outputPath(`${Date.now()}-${templateID}-rich-content.png`),
      animations: "disabled",
    });
    await page.goto(origin + "/pages/news/CNT_PLAIN");
    await expect(detail.locator(".content-body")).toHaveText("第一行\n第二行");
    await expect(detail.locator(".content-body")).toHaveCSS(
      "white-space",
      "pre-wrap",
    );
  });
});

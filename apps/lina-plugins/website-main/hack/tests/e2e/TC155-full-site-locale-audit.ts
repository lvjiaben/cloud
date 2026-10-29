import { mkdirSync, readdirSync, readFileSync } from "node:fs";
import { resolve, relative } from "node:path";
import { test, expect } from "../support/source-site";
import { FullSiteLocalePage } from "../pages/FullSiteLocalePage";
const publicRoot = resolve(import.meta.dirname, "../../../frontend/public");
const exactEnglishCopy: Record<string, string> = {
  公司名称: "Company name",
  版权信息版权信息版权信息版权信息版权信息版权信息版权信息:
    "Copyright information",
  地址地址地址: "Address",
  电信增值业务许可证编号电信增值业务许可证编号:
    "Value-added telecommunications business license number",
  公安备案编号公安备案编号公安备案编号公安备案编号:
    "Public security registration number",
  ICP备案编号ICP备案编号ICP备案编号: "ICP registration number",
  新闻标题: "News title",
  知识库标题: "Knowledge base title",
  知识库摘要知识库摘要知识库摘要知识库摘要知识库摘要: "Knowledge base summary",
  公告标题: "Announcement title",
  新闻摘要新闻摘要新闻摘要新闻摘要新闻摘要新闻摘要: "News summary",
  公告摘要公告摘要公告摘要公告摘要: "Announcement summary",
};
const architectureVariants: Record<
  string,
  { original: string; english: string; copy: string[] }
> = {
  "/plan/electricity.html": {
    original: "/lina/electricity-assets/architecture.svg",
    english: "/lina/electricity-assets/architecture.en.svg",
    copy: ["Smart Retail", "Customer-facing", "Core Operations", "Store Management"],
  },
  "/plan/financial.html": {
    original: "/lina/financial-assets/remoteL17139330959664.png",
    english: "/lina/financial-assets/remoteL17139330959664.en.svg",
    copy: ["Financial Services System", "Elastic Load Balancing"],
  },
  "/plan/game.html": {
    original: "/lina/game-assets/remoteL17139291334123.png",
    english: "/lina/game-assets/remoteL17139291334123.en.svg",
    copy: ["Region A Users", "Cloud Gaming Services", "Shared Game Services", "Mobile Game", "Virtual Machines", "ARM", "X86", "Game CP", "Game Publishing"],
  },
  "/plan/Goingtosea.html": {
    original: "/lina/goingsea-assets/remoteL17139341909320.png",
    english: "/lina/goingsea-assets/remoteL17139341909320.en.svg",
    copy: ["User", "Load Balancer", "Availability Zone 1"],
  },
  "/plan/government.html": {
    original: "/lina/government-assets/remoteL17139346523543.png",
    english: "/lina/government-assets/remoteL17139346523543.en.svg",
    copy: ["Security Framework", "Application Platform", "Operations Framework", "Resource Pool", "Compute", "Network", "Storage", "Security"],
  },
  "/plan/safety.html": {
    original: "/lina/safety-assets/remoteL17139281467723.png",
    english: "/lina/safety-assets/remoteL17139281467723.en.svg",
    copy: ["Perimeter Defense", "Protected Core Zone", "DDoS Protection", "Full-Stack Web Protection", "Next-Generation Cloud Firewall", "Website Security Guard", "Log Auditing", "Enhanced Vulnerability", "Penetration Testing Services"],
  },
};
function screenshotPath(testId: string, state: string) {
  const now = new Date(),
    pad = (value: number) => String(value).padStart(2, "0");
  const date = `${now.getFullYear()}${pad(now.getMonth() + 1)}${pad(now.getDate())}`;
  const time = `${pad(now.getHours())}${pad(now.getMinutes())}${pad(now.getSeconds())}-${String(now.getMilliseconds()).padStart(3, "0")}`;
  const directory = resolve(
    import.meta.dirname,
    "../../../../../../temp",
    date,
  );
  mkdirSync(directory, { recursive: true });
  return resolve(
    directory,
    `${time}-tc155-${testId.replace(/[^a-zA-Z0-9_-]/g, "-")}-${state}.png`,
  );
}
function discover(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = resolve(directory, entry.name);
    return entry.isDirectory()
      ? discover(path)
      : entry.name.endsWith(".html") &&
          /<html[\s>]/i.test(readFileSync(path, "utf8"))
        ? ["/" + relative(publicRoot, path).replaceAll("\\", "/")]
        : [];
  });
}
const discovered = discover(publicRoot);
const routes = [
  "/",
  ...discovered,
  "/products.html?product=ecs-zhenjiang-balanced-draft-20261020",
  "/products.html?product=ecs-zhenjiang-configurable-simulation&action=configureproduct",
  "/configure.html?product=ecs-zhenjiang-configurable-simulation",
  "/content.html?id=cnt_26e5270566ed24746761c2632bfe76b9",
  "/content.html?id=missing-locale-audit",
];
test.use({
  baseURL: "http://localhost:6680",
  websiteGateway: true,
  headless: true,
  reducedMotion: "reduce",
});
test.describe("TC-155 实际官网全部页面语言和布局审计", () => {
  for (const [device, viewport] of [
    ["desktop", { width: 1440, height: 1000 }],
    ["mobile", { width: 390, height: 844 }],
  ] as const) {
    for (const [index, path] of routes.entries())
      test(`TC-155${device === "desktop" ? "a" : "b"}${index + 1}: ${device} ${path}`, async ({
        page,
      }, info) => {
        test.setTimeout(60000);
        await page.setViewportSize(viewport);
        const site = new FullSiteLocalePage(page);
        const status = await site.open(path);
        await site.waitForBrand();
        await site.waitForPageData(path);
        expect(status).toBe(path === "/404.html" ? 404 : 200);
        await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
        const chinese = await site.snapshot();
        const captureId = `${device}-${index + 1}-${path.split("?")[0].replaceAll("/", "-").replace(".html", "") || "home"}`;
        await site.capture(screenshotPath(captureId, "chinese"));
        await site.language("en-US");
        await expect(page.locator("html")).toHaveAttribute("lang", "en-US");
        const english = await site.snapshot();
        if (chinese.footer)
          for (const phrase of [
            "Around-the-clock service",
            "Prompt responses",
            "Customer value first",
            "Comprehensive security",
            "Service guide",
            "Identity verification",
            "Partner platform",
            "Referral program",
            "Referral details",
            "Company news",
            "Certifications",
          ])
            expect(english.footer).toContain(phrase);
        for (const key of ["company", "copyright", "address"] as const)
          expect(english[key]).toBe(
            exactEnglishCopy[chinese[key] || ""] || chinese[key],
          );
        expect(english.businessQq).toBe(chinese.businessQq);
        expect(
          english.filings.map(({ kind, text, href, icon }) => ({
            kind,
            text,
            href,
            icon,
          })),
        ).toEqual(
          chinese.filings.map(({ kind, text, href, icon }) => ({
            kind,
            text: exactEnglishCopy[text || ""] || text,
            href,
            icon,
          })),
        );
        expect(english.news).toEqual(
          chinese.news.map((item) => ({
            title: exactEnglishCopy[item.title] || item.title,
            summary: exactEnglishCopy[item.summary] || item.summary,
          })),
        );
        if (path.startsWith("/plan/")) {
          expect(english.partnerTitle).toBe("Partners");
          expect(english.partnerSubtitle).toBe("They use our products and services");
          expect(english.partnerLogos).toEqual(chinese.partnerLogos);
        }
        if (path === "/cooperation/partner.html") {
          expect(english.partnerPageTitle).toBe("Partners");
          expect(english.partnerPageSubtitle).toBe(
            "Create new possibilities with our partners. Our comprehensive cloud services support business growth and innovation.",
          );
          expect(english.partnerPageTitle).not.toBe(chinese.partnerPageTitle);
          expect(english.partnerPageSubtitle).not.toBe(chinese.partnerPageSubtitle);
        }
        const route = new URL(path, "http://localhost").pathname,
          diagram = architectureVariants[route];
        if (diagram) {
          expect(chinese.architectureImage).toBe(diagram.original);
          expect(english.architectureImage).toBe(diagram.english);
          const svg = await page.evaluate(async (url) => {
            const response = await fetch(url);
            return { status: response.status, text: await response.text() };
          }, diagram.english);
          expect(svg.status).toBe(200);
          expect(svg.text).not.toMatch(/[\u3400-\u9fff]/);
          for (const phrase of diagram.copy) expect(svg.text).toContain(phrase);
        }
        if (path === "/plan/web.html") {
          expect(english.architectureImage).toBe(chinese.architectureImage);
          const svg = await page.evaluate(async (url) => {
            const response = await fetch(url!);
            return { status: response.status, text: await response.text() };
          }, english.architectureImage);
          expect(svg.status).toBe(200);
          expect(svg.text).not.toMatch(/[\u3400-\u9fff]/);
        }
        expect
          .soft(english.scrollWidth, "English horizontal overflow")
          .toBeLessThanOrEqual(viewport.width + 1);
        if (device === "mobile" && chinese.footer)
          expect
            .soft(
              await site.mobileFooterFits(),
              "English footer columns remain readable on mobile",
            )
            .toBe(true);
        if (device === "mobile")
          expect
            .soft(
              await site.mobileTranslatedControlsFit(),
              "English hero buttons, consultation links and partner headings fit their containers",
            )
            .toBe(true);
        if (device === "desktop" && path !== "/404.html")
          expect
            .soft(
              await site.desktopMenuFits(),
              "English top-level menu is fully readable",
            )
            .toBe(true);
        if (path === "/" || path === "/index.html")
          expect
            .soft(
              await site.homeSolutionTabsFit(),
              "English solution names do not wrap into narrow vertical columns",
            )
            .toBe(true);
        if (device === "mobile" && (path === "/" || path === "/index.html")) {
          const layout = await site.mobileHomeFits();
          expect(
            layout.title,
            "Mobile home hero title is present and visible",
          ).not.toBeNull();
          expect(
            layout.description,
            "Mobile home hero description is present and visible",
          ).not.toBeNull();
          expect(
            layout.tabs,
            "Mobile product tabs are available",
          ).not.toBeNull();
          expect
            .soft(
              layout.tabs!.bottom,
              "Product tabs do not cover the hero title",
            )
            .toBeLessThanOrEqual(layout.title!.top);
          expect
            .soft(
              layout.arrows,
              "Mobile carousel arrows cannot obscure product copy",
            )
            .toBe(0);
          for (const item of [layout.title!, layout.description!]) {
            expect
              .soft(item.left, `${path} content starts in viewport`)
              .toBeGreaterThanOrEqual(0);
            expect
              .soft(item.right, `${path} content ends in viewport`)
              .toBeLessThanOrEqual(viewport.width);
          }
          expect(
            layout.footer,
            "Mobile footer commitments remain visible",
          ).toHaveLength(4);
          for (const item of layout.footer) {
            expect
              .soft(item.width, "Mobile footer commitment has readable width")
              .toBeGreaterThan(44);
            expect
              .soft(item.height, `${item.text} must not wrap vertically`)
              .toBeLessThanOrEqual(item.lineHeight * 2 + 2);
          }
        }
        expect
          .soft(await site.untranslated(), "Untranslated static copy")
          .toEqual([]);
        if (path === "/content.html?id=cnt_26e5270566ed24746761c2632bfe76b9")
          await expect(page.locator("[data-content-body]")).not.toBeEmpty();
        if (
          path === "/content.html?id=missing-locale-audit" ||
          path === "/content.html"
        )
          await expect(page.locator("[data-status]")).toHaveText(
            "Content is temporarily unavailable. Try again.",
          );
        await site.capture(screenshotPath(captureId, "english"));
        await site.reload();
        await expect(page.locator("html")).toHaveAttribute("lang", "en-US");
        await site.waitForBrand();
        expect
          .soft(await site.untranslated(), "English after reload")
          .toEqual([]);
        await site.language("zh-CN");
        await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
        const restored = await site.snapshot();
        expect(restored.footer).toBe(chinese.footer);
        expect(restored.title).toBe(chinese.title);
        expect(restored.company).toBe(chinese.company);
        expect(restored.copyright).toBe(chinese.copyright);
        expect(restored.address).toBe(chinese.address);
        expect(restored.filings).toEqual(chinese.filings);
        expect(restored.partnerTitle).toBe(chinese.partnerTitle);
        expect(restored.partnerSubtitle).toBe(chinese.partnerSubtitle);
        expect(restored.partnerPageTitle).toBe(chinese.partnerPageTitle);
        expect(restored.partnerPageSubtitle).toBe(chinese.partnerPageSubtitle);
        if (diagram) expect(restored.architectureImage).toBe(diagram.original);
        expect
          .soft(restored.scrollWidth, "Chinese horizontal overflow")
          .toBeLessThanOrEqual(viewport.width + 1);
        expect(site.errors, "Uncaught browser errors").toEqual([]);
        expect(site.failedResources, "Missing local assets").toEqual([]);
        expect(site.writes, "Unexpected business writes").toEqual([]);
        await info.attach("locale-audit", {
          body: JSON.stringify(
            {
              path,
              device,
              chinese,
              english,
              businessCopy: [...site.businessCopy],
            },
            null,
            2,
          ),
          contentType: "application/json",
        });
      });
  }
  for(const [kind,id,zh,en] of [
    ['announcement','cnt_4500bccbf995008440481ae4aee3e534','公告正文公告正文公告正文公告正文公告正文公告正文公告正文','Announcement content'],
    ['knowledge','cnt_b7732284f030927fb1904c5399d64cb9','知识库正文知识库正文知识库正文知识库正文知识库正文知识库正文知识库正文','Knowledge base content'],
  ])test(`TC-155d-${kind}: article placeholder switches and restores on desktop and mobile`,async({page})=>{
    const site=new FullSiteLocalePage(page);
    for(const width of [1440,390]){
      await page.setViewportSize({width,height:900});
      await site.open('/content.html?id='+id);
      await site.language('zh-CN');
      await expect(page.locator('[data-content-body]')).toHaveText(zh);
      await site.language('en-US');
      await expect(page.locator('[data-content-body]')).toHaveText(en);
      await page.reload();
      await expect(page.locator('[data-content-body]')).toHaveText(en);
      await site.language('zh-CN');
      await expect(page.locator('[data-content-body]')).toHaveText(zh);
    }
    expect(site.writes).toEqual([]);expect(site.errors).toEqual([]);
  });
  test("TC-155c: 全部独立页面已发现，英文跨页和跨标签同步，中文可恢复", async ({
    page,
  }, info) => {
    expect(discovered).toHaveLength(26);
    const site = new FullSiteLocalePage(page);
    await site.open("/");
    await site.waitForBrand();
    await site.language("en-US");
    await site.open("/downloads.html");
    await site.waitForBrand();
    await expect(page.locator("html")).toHaveAttribute("lang", "en-US");
    const sibling = await site.sibling("/terms.html");
    try {
      await expect(sibling.page.locator("html")).toHaveAttribute(
        "lang",
        "en-US",
      );
      await sibling.language("zh-CN");
      await expect(page.locator("html")).toHaveAttribute("lang", "zh-CN");
      await expect(page.locator(".navbar-Login")).toHaveText("免费注册");
      await sibling.capture(screenshotPath(info.testId, "cross-tab-chinese"));
      expect(site.writes).toEqual([]);
      expect(sibling.writes).toEqual([]);
    } finally {
      await sibling.page.close();
    }
  });
});

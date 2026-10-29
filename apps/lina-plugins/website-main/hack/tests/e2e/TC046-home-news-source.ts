import {test,expect} from "../support/source-site";
import {readFileSync} from "node:fs";
import {HomeDetailPage} from "../pages/HomeDetailPage";
import {previewResponse} from "../support/home-preview-data.mjs";
const expected=JSON.parse(readFileSync(new URL("../support/static-news-fixture.json",import.meta.url),"utf8"));
test.use({baseURL:"http://localhost:6680",viewport:{width:1440,height:900},reducedMotion:"reduce"});
test.describe("TC-46 原新闻区布局与本方新闻",()=>{
 test("TC-46a: 静态条款保留，后台更新、悬停和空态不污染左卡",async({page},info)=>{
  let mode="normal";
  await page.route("**/x/**",async route=>{const req=route.request(),url=new URL(req.url()),r=previewResponse(url,req.method());if(url.pathname.includes("/content/items")){if(mode==="failed")return route.fulfill({status:503,json:{code:503}});r.data={items:mode==="empty"?[]:[{contentNo:"own-"+url.searchParams.get("type"),title:(mode==="updated"?"后台已更新：":"本方新闻：")+"长标题用于验证日期不会被挤出".repeat(10),category:{nameZhCN:"本方技术分类"},summary:"本方后台摘要",publishedAt:"2026-10-20T00:00:00Z"}]};}await route.fulfill({status:r.status,json:{code:r.status===200?0:r.status,data:r.data}});});
  const site=new HomeDetailPage(page);
  for(mode of ["normal","updated","empty","failed"]){
   await site.open("/index.html");const area=page.locator(".container_bg6"),left=area.locator(".comprehensive-news-left");
   await expect(area.locator(".sk-title")).toHaveText(expected.heading);await expect(area.locator(".sk-desc")).toHaveText(expected.description);await expect(left.locator(".comprehensive-news-left-inner-title")).toHaveText(expected.title);await expect(left.locator(".comprehensive-news-left-inner-describe")).toHaveText(expected.body);await expect(left).toBeVisible();await expect(left.locator("a")).toHaveAttribute("aria-disabled","true");await expect(left.locator("a[href]")).toHaveCount(0);
   const rows=area.locator("[data-home-news] > a");
   if(["normal","updated"].includes(mode)){await expect(rows).toHaveCount(2);await expect(rows.first()).toContainText(mode==="updated"?"后台已更新：":"本方新闻：");await expect(rows.first()).toHaveAttribute("href",/content[.]html[?]id=own-/);await site.hoverNews(1);await expect(rows.nth(1).locator(".news-article-item-title span")).toHaveText("本方技术分类");await expect(rows.nth(1)).toHaveClass(/active/);await expect(rows.first()).not.toHaveClass(/active/);await expect(rows.nth(1)).toHaveCSS("background-color","rgb(255, 255, 255)");await expect(rows.nth(1)).toHaveCSS("box-shadow",/rgba/);expect(await rows.nth(1).locator(".news-article-item-desc").evaluate(n=>n.getBoundingClientRect().height)).toBeGreaterThan(0);expect(await rows.nth(1).locator(".news-article-item-date").evaluate(n=>n.getBoundingClientRect().right<=innerWidth)).toBe(true);await expect(rows.nth(1).locator(".news-article-item-desc")).toContainText("本方后台摘要");await expect(rows.nth(1).locator(".news-article-item-date")).toHaveText("2026/10/20");}
   else {await expect(rows).toHaveCount(0);await expect(area.locator("[data-home-news-status]")).toHaveText(mode==="empty"?"暂无已发布的新闻与公告。":"新闻与公告加载失败，请重试。");}
   expect(await left.evaluate(n=>n.querySelector(".comprehensive-news-left-inner-describe").getBoundingClientRect().bottom<n.querySelector(".comprehensive-news-left-inner-button").getBoundingClientRect().top)).toBe(true);await expect(left.locator(".comprehensive-news-left-inner")).toHaveCSS("background-image",/remoteL17075739033142/);await site.captureSection(".container_bg6",info.outputPath(mode+"-news.png"));expect(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth)).toBe(true);
  }
 });
});

